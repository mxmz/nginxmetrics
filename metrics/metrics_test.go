package metrics

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"
)

func TestMetrics_HandleLogLine(t *testing.T) {

	var file, _ = ioutil.ReadFile("./sample.json.log")
	var lines = strings.Split(string(file), "\n")

	var m = NewMetrics(nil)

	for _, line := range lines {
		var lineMap map[string]string
		json.Unmarshal([]byte(line), &lineMap)
		m.HandleLogLine(lineMap)

	}

	var c = m.bytes_sent
	var _ = c
}
