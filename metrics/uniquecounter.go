package metrics

import (
	"log"
	"net/http"
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
		vt := v.(time.Time)
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
	uc.cache.Add(id, reftime)
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
		var idSource = v.ValueSource
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
			id, ok := l[idSource]
			if ok && len(id) > 0 {

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
