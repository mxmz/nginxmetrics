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
		}
	}
}`

func TestMetrics_HandleLogLine(t *testing.T) {

	var file, _ = ioutil.ReadFile("./sample.json.log")
	var lines = strings.Split(string(file), "\n")

	var config Config
	json.Unmarshal([]byte(config1), &config)
	var m = NewMetrics(&config)

	for _, line := range lines {
		var lineMap map[string]string
		json.Unmarshal([]byte(line), &lineMap)
		m.HandleLogLine(lineMap)

	}

	var r = m.r

	fmt.Printf("r: %v\n", r)

	//var c = m.bytes_sent
	var _ = r
}
