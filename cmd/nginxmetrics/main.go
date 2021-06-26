package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hpcloud/tail"

	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	"mxmz.it/nginxmetrics/metrics"
)

type config struct {
	Metrics map[string]*metrics.MetricConfig          `json:"metrics,omitempty"`
	Unique  map[string]*metrics.DistinctCounterConfig `json:"unique,omitempty"`
}

type logHandler interface {
	HandleLogLine(line map[string]string)
}

func main() {
	var config config
	var configContent, err = ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		panic(err)
	}

	var mode = os.Args[2]
	switch mode {
	case "standard":
		{
			doStandardMetrics(&config, os.Args[3:])
		}

	case "unique":
		{
			doUniqueMetrics(&config, os.Args[3:])
		}
	}

}

func doStandardMetrics(config *config, files []string) {
	var m = metrics.NewMetrics(config.Metrics)

	go func() {
		var found = map[string]struct{}{}
		for {
			fmt.Println(len(found))
			for _, v := range files {

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
	http.Handle("/", m.HttpHandler())
	http.ListenAndServe(":9802", nil)
}
func doUniqueMetrics(config *config, files []string) {
	var m = metrics.NewUniqueValueMetrics(config.Unique)

	go func() {
		var found = map[string]struct{}{}
		for {
			fmt.Println(len(found))
			for _, v := range files {

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
	http.Handle("/", m.HttpHandler())
	http.ListenAndServe(":9803", nil)
}

func followLog(m logHandler, path string) {

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
				var err error
				var lineMap map[string]string
				if strings.HasPrefix(line.Text, "{") {
					err = json.Unmarshal([]byte(line.Text), &lineMap)
				} else {
					if strings.Contains(line.Text, "[error]") {
						lineMap = map[string]string{
							"error": "1",
						}
					} else if strings.Contains(line.Text, "[crit]") {
						lineMap = map[string]string{
							"crit": "1",
						}
					} else {
						err = errors.New("SKIPPING LINE")
					}
				}

				if err == nil {
					m.HandleLogLine(lineMap)
					count++
					//println(count, line.Text)
				}

			}
		}
	}
}
