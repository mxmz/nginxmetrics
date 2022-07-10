package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync/atomic"

	"github.com/hpcloud/tail"

	"encoding/json"
	"net/http"
	"path/filepath"
	"time"

	jsoniter "github.com/json-iterator/go"
	"mxmz.it/nginxmetrics/metrics"
)

type NELConfig struct {
	NELReportLog string `json:"nel_report_log,omitempty"`
	CSPReportLog string `json:"csp_report_log,omitempty"`
	Uuid         string `json:"uuid,omitempty"`
}

type config struct {
	Metrics map[string]*metrics.MetricConfig          `json:"metrics,omitempty"`
	Unique  map[string]*metrics.DistinctCounterConfig `json:"unique,omitempty"`
	NEL     NELConfig                                 `json:"nel,omitempty"`
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
	case "nel":
		{
			doNELReport(&config)
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
	http.Handle("/config", returnAsJson((config.Metrics)))
	http.Handle("/", m.HttpHandler())
	http.ListenAndServe(":9802", nil)
}
func doUniqueMetrics(config *config, files []string) {
	var warnings int64 = 0
	var warnCh = make(chan int64)

	var m = metrics.NewUniqueValueMetrics(config.Unique, func(name string, k string, labels map[string]string, rate float64) {
		v := atomic.AddInt64(&warnings, 1)
		select {
		case warnCh <- v:
		default:
			log.Printf("WARN: %s: id = %s labels = [%v] rate = %v", name, k, labels, rate)
		}

	})

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
			time.Sleep(60 * time.Second)
			m.Purge(time.Now())
		}

	}()
	http.Handle("/metrics", m.HttpHandler())
	http.Handle("/", m.HttpHandler())

	var inspect = m.InspectHttpHandler()
	http.Handle("/inspect", inspect)
	http.Handle("/config", returnAsJson((config.Unique)))
	http.HandleFunc("/inspect/wait", func(rsp http.ResponseWriter, r *http.Request) {

		select {
		case <-warnCh:
		case <-r.Context().Done():
			{
				log.Println("abort /inspect/wait")
			}
		case <-time.After(30 * time.Second):
		}
		rsp.Header().Add("X-Warnings", fmt.Sprintf("%d", warnings))
		inspect.ServeHTTP(rsp, r)

	})

	http.ListenAndServe(":9803", nil)
}

func returnAsJson(rv interface{}) http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, _ *http.Request) {
		var json, _ = json.Marshal(rv)
		rsp.Header().Add("Content-Type", "application/json")
		rsp.WriteHeader(200)
		rsp.Write(json)
	})
}

func followLog(m logHandler, path string) {

	t, err := tail.TailFile(path, tail.Config{Follow: true, ReOpen: true, Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}, Poll: true})
	if err != nil {
		panic(err)
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	var lines = t.Lines
	var count = 0
	for {
		select {
		case line := <-lines:
			{
				var err error
				var lineMap map[string]interface{}
				if strings.HasPrefix(line.Text, "{") {
					err = json.Unmarshal([]byte(line.Text), &lineMap)
				} else {
					if strings.Contains(line.Text, "[error]") {
						lineMap = map[string]interface{}{
							"error": "1",
						}
					} else if strings.Contains(line.Text, "[crit]") {
						lineMap = map[string]interface{}{
							"crit": "1",
						}
					} else {
						err = errors.New("SKIPPING LINE")
					}
				}

				if err == nil {
					m.HandleLogLine(metrics.StringizeMap(lineMap))
					count++
					//println(count, line.Text)
				}

			}
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const max_nel_report_length = 100000

func sendReportToChan(typ string, ch chan<- interface{}) http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, r *http.Request) {
		if r.ContentLength > max_nel_report_length || r.ContentLength < 0 {
			r.Body.Close()
			panic("Invalid content length")
		}
		var body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			r.Body.Close()
			panic(err)
		}
		var v interface{}
		err = json.Unmarshal(body, &v)
		if err != nil {
			r.Body.Close()
			panic(err)
		}
		var data = map[string]interface{}{}
		data["type"] = typ
		data["@timestamp"] = time.Now().Format(time.RFC3339)
		data["report"] = &v
		var ctx = r.URL.Query().Get("context")
		if len(ctx) > 64 {
			panic("Bad ctx")
		}
		var x_forwarded_for = r.Header.Get("X-Forwarded-For")
		data["x_forwarded_for"] = x_forwarded_for

		data["context"] = ctx

		ch <- data
		rsp.Header().Add("Content-Type", "application/json")
		rsp.WriteHeader(200)
		rsp.Write([]byte("ok\n"))
	})
}
func doNELReport(config *config) {
	var nelLog = config.NEL.NELReportLog
	var cspLog = config.NEL.CSPReportLog
	var nelLogCh = make(chan interface{})
	var cspLogCh = make(chan interface{})

	var logger = func(ch <-chan interface{}, path string) {
		var outlog *os.File
		for {
			select {
			case line := <-ch:
				{
					if outlog == nil || !fileExists(path) {
						if outlog != nil {
							outlog.Close()
						}

						outlog, _ = os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0770)
					}
					if outlog != nil {
						var json, _ = json.Marshal(line)
						json = append(json, '\n')
						outlog.Write(json)
					}
				}
			}
		}

	}
	go logger(nelLogCh, nelLog)
	go logger(cspLogCh, cspLog)

	http.Handle("/nel/"+config.NEL.Uuid, sendReportToChan("nel", nelLogCh))
	http.Handle("/csp/"+config.NEL.Uuid, sendReportToChan("csp", cspLogCh))

	var nop = func(rsp http.ResponseWriter, _ *http.Request) {
		rsp.Header().Add("Content-Type", "text/plain")
		rsp.WriteHeader(200)
		rsp.Write([]byte("nop\n"))
	}
	http.HandleFunc("/nop", nop)
	http.Handle("/config", returnAsJson(config.NEL))
	http.ListenAndServe(":10666", nil)
}
