package metrics

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type DistinctCounterConfig struct {
	ValueSource string            `json:"value_source,omitempty"`
	TimeWindow  int               `json:"time_window,omitempty"`
	LabelMap    map[string]string `json:"label_map,omitempty"`
	IfMatch     map[string]string `json:"if_match,omitempty"`
}

type UniqueCounter struct {
	cache  *lru.Cache
	maxAge time.Duration
	gauge  prometheus.Gauge
}

type cacheEntry struct {
	count int
	first time.Time
	last  time.Time
}

func NewUniqueCounter(size int, maxAge time.Duration, gauge prometheus.Gauge) *UniqueCounter {
	var c, _ = lru.New(size)
	return &UniqueCounter{c, maxAge, gauge}
}

func (uc *UniqueCounter) purge(reftime time.Time) {
	var oldestBound = reftime.Add(-uc.maxAge)

	for {
		var k, v, ok = uc.cache.GetOldest()
		if !ok {
			break
		}
		e := v.(*cacheEntry)
		vt := e.last
		if vt.Before(oldestBound) {
			uc.cache.Remove(k)
			uc.gauge.Set(float64(uc.cache.Len()))
			log.Println(k)
		} else {
			break
		}
	}
}

func (uc *UniqueCounter) Add(id string, reftime time.Time) {
	var e, ok = uc.cache.Get(id)
	var updated *cacheEntry
	if !ok {
		updated = &cacheEntry{1, reftime, reftime}
	} else {
		updated = e.(*cacheEntry)
		updated.count++
	}
	uc.cache.Add(id, updated)
	uc.gauge.Set(float64(uc.cache.Len()))
}

func (uc *UniqueCounter) Count() int {
	return uc.cache.Len()
}

type UniqueCounterMap struct {
	counters map[string]*UniqueCounter
	lock     sync.RWMutex
}

func (cm *UniqueCounterMap) purge(reftime time.Time) {
	cm.lock.Lock()
	var l = make([]*UniqueCounter, 0, len(cm.counters))
	for k, v := range cm.counters {
		l = append(l, v)
		log.Printf("purging %s[%d]...\n", k, v.Count())
	}
	cm.lock.Unlock()
	for _, v := range l {
		v.purge(reftime)

	}
}

func (cm *UniqueCounterMap) get(name string) *UniqueCounter {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	var rv, ok = cm.counters[name]
	if ok {
		return rv
	}
	return nil
}
func (cm *UniqueCounterMap) keys() []string {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	var rv = make([]string, 0, len(cm.counters))
	for k, _ := range cm.counters {
		rv = append(rv, k)
	}
	return rv
}
func (cm *UniqueCounterMap) create(name string, timeWindow time.Duration, gauge prometheus.Gauge) *UniqueCounter {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	var rv, ok = cm.counters[name]
	if ok {
		return rv
	}
	rv = NewUniqueCounter(1024, timeWindow, gauge)
	cm.counters[name] = rv
	return rv
}

type UniqueValueMetrics struct {
	r       *prometheus.Registry
	metrics []injectLineFunc
	ucm     *UniqueCounterMap
}

func NewUniqueValueMetrics(config map[string]*DistinctCounterConfig) *UniqueValueMetrics {
	var ucm = &UniqueCounterMap{}
	ucm.counters = map[string]*UniqueCounter{}
	var r = prometheus.NewRegistry()
	r.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	var metrics = []injectLineFunc{}
	for k, v := range config {
		var name = k
		var labelMap = v.LabelMap
		var idSource = strings.Split(v.ValueSource, ",")
		var ifMatch = makeIfMatchMap(v.IfMatch)
		gaugevec := promauto.With(r).NewGaugeVec(prometheus.GaugeOpts{
			Name: name,
			Help: name,
		}, keys(v.LabelMap))
		metrics = append(metrics, func(l map[string]string) {
			for k, v := range ifMatch {
				if !v.MatchString(l[k]) {
					return
				}
			}
			id := ""
			for _, v := range idSource {
				id += "#" + l[v]
			}
			//id, ok := l[idSource]
			if len(id) > len(idSource) {
				//	fmt.Printf("id = %v\n", id)

				var labelValues = map[string]string{}
				var labelKey = ""
				for k, v := range labelMap {
					labelValues[k] = l[v]
					labelKey += "#" + k + "#" + l[v]
				}
				uc := ucm.get(labelKey)
				if uc == nil {
					gauge := gaugevec.With(labelValues)
					uc = ucm.create(labelKey, time.Duration(v.TimeWindow)*time.Second, gauge)
				}
				uc.Add(id, time.Now())
			}
		})

	}
	return &UniqueValueMetrics{r, metrics, ucm}
}

func (m *UniqueValueMetrics) HttpHandler() http.Handler {
	return promhttp.HandlerFor(m.r, promhttp.HandlerOpts{})
}

func (m *UniqueValueMetrics) HandleLogLine(line map[string]string) {

	for _, v := range m.metrics {
		v(line)
	}
}

func (m *UniqueValueMetrics) Purge(timeref time.Time) {
	m.ucm.purge(timeref)
}

func (m *UniqueValueMetrics) InspectHttpHandler() http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		var ks = m.ucm.keys()
		var rv = map[string]interface{}{}
		for _, v := range ks {
			rv[v] = &struct{}{}
		}
		var json, _ = json.Marshal(rv)
		rsp.WriteHeader(200)
		rsp.Header().Add("Content-Type", "application/json")
		rsp.Write(json)
	})
}
