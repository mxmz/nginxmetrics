package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hpcloud/tail"

	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"mxmz.it/nginxmetrics/metrics"
)

type config struct {
	Metrics map[string]*metrics.MetricConfig `json:"metrics,omitempty"`
}

func main() {
	var config config
	var configContent, err = ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	json.Unmarshal(configContent, &config)

	var m = metrics.NewMetrics(&metrics.Config{Metrics: config.Metrics})

	go func() {
		var found = map[string]struct{}{}
		for {
			fmt.Println(found)
			for _, v := range os.Args[2:] {

				var files, _ = filepath.Glob(v)
				for _, v := range files {
					if _, ok := found[v]; !ok {
						go followLog(m, v)
						found[v] = struct{}{}
					}
				}
			}
			time.Sleep(10 * time.Second)
		}

	}()
	http.Handle("/metrics", m.HttpHandler())
	http.ListenAndServe(":9802", nil)
}

// func readLog(m *metrics.Metrics) {

// 	for {
// 		var file, _ = ioutil.ReadFile("../../metrics/sample.json.log")

// 		var lines = strings.Split(string(file), "\n")

// 		for _, line := range lines {
// 			var lineMap map[string]string
// 			json.Unmarshal([]byte(line), &lineMap)
// 			m.HandleLogLine(lineMap)
// 			if lineMap["vhost"] == "" {
// 				panic("")
// 			}
// 			time.Sleep(10 * time.Millisecond)
// 		}
// 	}
// }

func followLog(m *metrics.Metrics, path string) {

	t, err := tail.TailFile(path, tail.Config{Follow: true, ReOpen: true, Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}, Poll: true})
	if err != nil {
		panic(err)
	}
	var lines = t.Lines
	var count = 0
	for {
		select {
		case line := <-lines:
			{
				var lineMap map[string]string
				err := json.Unmarshal([]byte(line.Text), &lineMap)
				if err == nil {
					m.HandleLogLine(lineMap)
					if lineMap["vhost"] == "" {
						panic("")
					}
					count++
					//println(count, line.Text)
				}

			}
		}
	}
}
