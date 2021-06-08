package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Config struct {
}

type Metrics struct {
	r                     *prometheus.Registry
	bytes_sent            *prometheus.CounterVec
	request_time          *prometheus.SummaryVec
	backend_response_time *prometheus.SummaryVec
}

func NewMetrics(c *Config) *Metrics {

	var r = prometheus.NewRegistry()

	bytes_sent := promauto.With(r).NewCounterVec(prometheus.CounterOpts{
		Name: "body_bytes_sent",
		Help: "The total number of bytes sent",
	}, []string{"vhost", "method"})

	request_time := promauto.With(r).NewSummaryVec(prometheus.SummaryOpts{
		Name:       "request_time",
		Help:       "Request time",
		MaxAge:     10 * time.Minute,
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"vhost", "status"})
	backend_response_time := promauto.With(r).NewSummaryVec(prometheus.SummaryOpts{
		Name:       "backend_response_time",
		Help:       "Backend request time",
		MaxAge:     30 * time.Second,
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"vhost", "backend_status"})

	return &Metrics{r, bytes_sent, request_time, backend_response_time}
}

func (m *Metrics) HandleLogLine(line map[string]string) {

	bytes_sent, err := strconv.Atoi(line["body_bytes_sent"])
	if err == nil {
		m.bytes_sent.With(map[string]string{
			"vhost":  line["vhost"],
			"method": line["method"],
		}).Add(float64(bytes_sent))

	}

	request_time, err := strconv.ParseFloat(line["request_time"], 64)
	if err == nil {
		m.request_time.With(map[string]string{
			"vhost":  line["vhost"],
			"status": line["status"],
		}).Observe(float64(request_time))
	}

	backend_response_time, err := strconv.ParseFloat(line["backend_response_time"], 64)
	if err == nil {
		m.backend_response_time.With(map[string]string{
			"vhost":          line["vhost"],
			"backend_status": line["backend_status"],
		}).Observe(float64(backend_response_time))
	}
}

func (m *Metrics) HttpHandler() http.Handler {
	return promhttp.HandlerFor(m.r, promhttp.HandlerOpts{})
}
