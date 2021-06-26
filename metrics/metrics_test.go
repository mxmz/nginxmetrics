package metrics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
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
			"time_window": 3600,
			"id_source": "remote_addr",
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
}

func TestUniqueValueMetrics_HandleLogLine(t *testing.T) {

	var file, _ = ioutil.ReadFile("./sample.json.log")
	var lines = strings.Split(string(file), "\n")

	var config Config
	json.Unmarshal([]byte(config1), &config)
	var m = NewUniqueValueMetrics(config.Unique)

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
}
