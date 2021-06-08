package main

import (
	"fmt"
	"os"

	"github.com/hpcloud/tail"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"mxmz.it/nginxmetrics/metrics"
)

func main() {
	var m = metrics.NewMetrics(nil)

	go func() {
		var found = map[string]struct{}{}
		for {
			fmt.Println(found)
			for _, v := range os.Args[1:] {

				var files, _ = filepath.Glob(v)
				if files != nil {
					for _, v := range files {
						if _, ok := found[v]; !ok {
							go followLog(m, v)
							found[v] = struct{}{}
						}
					}

				}
			}
			time.Sleep(10 * time.Second)
		}

	}()
	http.Handle("/metrics", m.HttpHandler())
	http.ListenAndServe(":2112", nil)
}

func readLog(m *metrics.Metrics) {

	for {
		var file, _ = ioutil.ReadFile("../../metrics/sample.json.log")

		var lines = strings.Split(string(file), "\n")

		for _, line := range lines {
			var lineMap map[string]string
			json.Unmarshal([]byte(line), &lineMap)
			m.HandleLogLine(lineMap)
			if lineMap["vhost"] == "" {
				panic("")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

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
