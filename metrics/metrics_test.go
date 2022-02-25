package metrics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"log"
)

var config1 = `
{
	"metrics": {
		"bytes_sent": {
			"value_source": "body_bytes_sent",
			"type":        "counter",
			"label_map": {
				"vhost":  "vhost",
				"method": "method"
			}
		},
		"request_time": {
			"value_source": "request_time",
			"type":        "summary",
			"label_map": {
				"vhost":  "vhost",
				"status": "status"
			}
		},
		"backend_response_time": {
			"value_source": "backend_response_time",
			"type":        "summary",
			"label_map": {
				"vhost":          "vhost",
				"backend_status": "backend_status"
			}
		},
		"js_backend_response_time": {
			"value_source": "backend_response_time",
			"type":        "summary",
			"label_map": {
				"vhost":          "vhost",
				"backend_status": "backend_status"
			},
			"if_match": {
				"uri": "^/js/"
			}
		},
		"lib_backend_response_time": {
			"value_source": "backend_response_time",
			"type":        "summary",
			"label_map": {
				"vhost":          "vhost",
				"backend_status": "backend_status"
			},
			"if_match": {
				"uri": "^/lib/"
			}
		}
	},
	"unique": {
		"users": {
			"time_window": 60,
			"value_source": "remote_addr,user_agent",
			"label_map": {
				"vhost":          "vhost"
			},
			"notify_rate_threshold": 0
	  	},
		"users2": {
			"time_window": 60,
			"value_source": "remote_addr,user_agent",
			"label_map": {
				"vhost":          "vhost"
			}
	  	}

	}
}`

type Config struct {
	Metrics map[string]*MetricConfig          `json:"metrics,omitempty"`
	Unique  map[string]*DistinctCounterConfig `json:"unique,omitempty"`
}

func TestMetrics_HandleLogLine(t *testing.T) {

	var file, _ = ioutil.ReadFile("./sample.json.log")
	var lines = strings.Split(string(file), "\n")

	var config Config
	json.Unmarshal([]byte(config1), &config)
	var m = NewMetrics(config.Metrics)

	for _, line := range lines {
		var lineMap map[string]string
		json.Unmarshal([]byte(line), &lineMap)
		m.HandleLogLine(lineMap)

	}

	var r = m.r

	fmt.Printf("r: %v\n", r)

	var c, _ = r.Gather()
	for _, v := range c {

		fmt.Printf("v: %v\n", v)
	}
	var _ = r
	t.Log("finished")
}
func TestMetrics_HandleLogLine1(t *testing.T) {

	var file = sample
	var lines = strings.Split(string(file), "\n")

	var config Config
	json.Unmarshal([]byte(config1), &config)
	var m = NewMetrics(config.Metrics)

	for _, line := range lines {
		var lineMap map[string]interface{}
		json.Unmarshal([]byte(line), &lineMap)
		m.HandleLogLine(StringizeMap(lineMap))

	}

	var r = m.r

	t.Logf("r: %v\n", r)

	var c, _ = r.Gather()
	for _, v := range c {

		t.Logf("v: %v\n", v)
	}
	var _ = r

	t.Log("finished")
}

func TestUniqueValueMetrics_HandleLogLine(t *testing.T) {

	var file, _ = ioutil.ReadFile("./sample.json.log")
	var lines = strings.Split(string(file), "\n")

	var config Config
	json.Unmarshal([]byte(config1), &config)
	var m = NewUniqueValueMetrics(config.Unique, func(k string, rate float64, labels map[string]string) {
		fmt.Printf("%s %v %v\n", k, rate, labels)
	})

	for _, line := range lines {
		var lineMap map[string]string
		json.Unmarshal([]byte(line), &lineMap)
		m.HandleLogLine(lineMap)

	}

	var r = m.r

	log.Printf("r = %v\n", r)

	var c, _ = r.Gather()
	for _, v := range c {

		log.Printf("v: %v\n", v)
		if *v.Name == "users" && *v.GetMetric()[0].Label[0].Value == "https://troubleticket-pistacchio-areaclienti.irideos.it" {
			t.Logf("value: %f", *v.GetMetric()[0].Gauge.Value)
		}
		if *v.Name == "users" && *v.GetMetric()[0].Label[0].Value == "https://troubleticket-areaclienti-areaclienti.irideos.it" {
			t.Logf("value: %f", *v.GetMetric()[0].Gauge.Value)
		}
	}
	m.Purge(time.Now())
	var _ = r
	t.Log("finished")
}

const sample = `
{"@timestamp":"2021-06-07T07:01:00+02:00","remote_addr":"79.53.93.15 ","remote_user":"","auth_times":"0.000 0.004 0.004","auth_addr":"172.27.193.20:9470","method":"GET","uri":"/Issues/Tickets/Create?return_to=/redirect/areaclienti/TechnicalPanel/ConnectivityView.aspx&TICKET_TYPE=TICKET_TYPE_EXTERNAL&PROBLEM_TYPE=INCIDENT&SCOPE=SCOPE_ASSURANCE&SERVIZIO=XDSL&REMEDY_SERVICE=CI-571532-887392&ORARI_DIPONIB=09:00%2013:00%20-%2014:00%2018:00&REFERENTE_TECNICO=&LINE_FTTH=&EMAIL_REF_TEC=&TEL_REF_TEC=&OPENER_NAME=FUSI&OPENER_SURNAME=PAOLO","status": "200","body_bytes_sent":"170","request_time":0.002,"http_referrer":"","backend_addr":"","backend_status":"","backend_response_time":"","vhost":"https://troubleticket-reseller-areaclienti.irideos.it","jwt_exp":"","user_agent":"Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.77 Safari/537.36","request_length":1554}`
