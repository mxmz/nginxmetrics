package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricConfig struct {
	Type        string            `json:"type,omitempty"`
	ValueSource string            `json:"value_source,omitempty"`
	LabelMap    map[string]string `json:"label_map,omitempty"`
	IfMatch     map[string]string `json:"if_match,omitempty"`
}

type Config struct {
	Metrics map[string]*MetricConfig `json:"metrics,omitempty"`
}

type injectLineFunc func(line map[string]string)

type Metrics struct {
	r       *prometheus.Registry
	metrics map[string]injectLineFunc
}

func keys(m map[string]string) []string {
	l := []string{}
	for k := range m {
		l = append(l, k)
	}
	return l
}

func makeIfMatchMap(m map[string]string) map[string]*regexp.Regexp {
	if m == nil {
		return nil
	}
	var rv = map[string]*regexp.Regexp{}
	for k, v := range m {
		rv[k] = regexp.MustCompile(v)
	}
	return rv
}

func NewMetrics(c *Config) *Metrics {
	var metrics = map[string]injectLineFunc{}
	var r = prometheus.NewRegistry()

	for k, v := range c.Metrics {
		var name = k
		var labelMap = v.LabelMap
		var valueSource = v.ValueSource
		var ifMatch = makeIfMatchMap(v.IfMatch)
		switch v.Type {
		case "counter":
			{
				counter := promauto.With(r).NewCounterVec(prometheus.CounterOpts{
					Name: name,
					Help: name,
				}, keys(v.LabelMap))
				metrics[k] = func(l map[string]string) {
					for k, v := range ifMatch {
						if !v.MatchString(l[k]) {
							return
						}
					}
					c, err := strconv.ParseFloat(l[valueSource], 64)
					if err == nil {
						var labelValues = map[string]string{}
						for k, v := range labelMap {
							labelValues[k] = l[v]
						}
						counter.With(labelValues).Add(float64(c))
					}
				}
			}
			break
		case "summary":
			{
				counter := promauto.With(r).NewSummaryVec(prometheus.SummaryOpts{
					Name:       name,
					Help:       name,
					MaxAge:     10 * time.Minute,
					Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
				}, keys(v.LabelMap))
				metrics[k] = func(l map[string]string) {
					for k, v := range ifMatch {
						if !v.MatchString(l[k]) {
							return
						}
					}
					c, err := strconv.ParseFloat(l[valueSource], 64)
					if err == nil {
						var labelValues = map[string]string{}
						for k, v := range labelMap {
							labelValues[k] = l[v]
						}
						counter.With(labelValues).Observe(float64(c))
					}
				}
			}
			break
		default:
			panic("Unsupporter metric type")
		}
	}

	return &Metrics{r, metrics}
}

func (m *Metrics) HandleLogLine(line map[string]string) {

	for _, v := range m.metrics {
		v(line)
	}

	// 	bytes_sent, err := strconv.Atoi(line["body_bytes_sent"])
	// 	if err == nil {
	// 		m.bytes_sent.With(map[string]string{
	// 			"vhost":  line["vhost"],
	// 			"method": line["method"],
	// 		}).Add(float64(bytes_sent))

	// 	}

	// 	request_time, err := strconv.ParseFloat(line["request_time"], 64)
	// 	if err == nil {
	// 		m.request_time.With(map[string]string{
	// 			"vhost":  line["vhost"],
	// 			"status": line["status"],
	// 		}).Observe(float64(request_time))
	// 	}

	// 	backend_response_time, err := strconv.ParseFloat(line["backend_response_time"], 64)
	// 	if err == nil {
	// 		m.backend_response_time.With(map[string]string{
	// 			"vhost":          line["vhost"],
	// 			"backend_status": line["backend_status"],
	// 		}).Observe(float64(backend_response_time))
	// 	}
}

func (m *Metrics) HttpHandler() http.Handler {
	return promhttp.HandlerFor(m.r, promhttp.HandlerOpts{})
}
