// Copyright 2022 Mohammad Hadi Hosseinpour
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

var (
	showHelp          bool
	dialTimeout       int
	keepAliveInterval int
	configFile        string

	rules  = make([]*ProxyRule, 0, 10)
	handle = make([]ProxyHandle, 0, 10)
)

// ProxyRule 代理规则
type ProxyRule struct {
	Listen      string `remark:"listening address (<host>:<port>)"`
	Target      string `remark:"remote target (<host>:<port>)"`
	Proxy       string `remark:"proxy address (<proto>://[user[:password]@]<host>:<port>/)"`
	DialTimeout int
	Keepalive   int
}

// ProxyHandle 代理处理器
type ProxyHandle interface {
	Run() error
}

func init() {
	flag.BoolVar(&showHelp, "help", false, "show usage")
	flag.IntVar(&dialTimeout, "timeout", 10, "dial timeout")
	flag.IntVar(&keepAliveInterval, "keepalive", 30, "keep-alive interval")
	flag.StringVar(&configFile, "config", "config.json", "config file path")
}

func main() {
	var pair []string
	var ok, hasError bool
	var key, port, scheme, listen, target, proxy string
	var wg = new(sync.WaitGroup)
	var opt = make(map[string]map[string]*url.URL)
	var signals = make(chan os.Signal, 1)

	flag.Parse()

	if showHelp || "" == configFile {
		flag.Usage()
		return
	}

	if data, err := os.ReadFile(configFile); nil == err {
		if err = json.Unmarshal(data, &rules); err != nil {
			log.Println("read config file error:", err)
			return
		}
	} else {
		log.Println("read config file error:", err)
		return
	}

	signal.Notify(signals, os.Interrupt, os.Kill)

	for _, rule := range rules {
		if u, err := url.Parse(rule.Listen); nil == err {
			scheme = strings.ToLower(u.Scheme)
			port = u.Port()

			if "http" == scheme {
				if "" == port {
					port = "80"
				} else if "80" == port {
					rule.Listen = strings.ReplaceAll(rule.Listen, ":80", "")
				}
			}
			if "https" == scheme {
				if "" == port {
					port = "443"
				} else if "443" == port {
					rule.Listen = strings.ReplaceAll(rule.Listen, ":443", "")
				}
			}

			key = scheme + ":" + port
			if _, ok = opt[key]; !ok {
				opt[key] = make(map[string]*url.URL)
			}

			rule.Listen = strings.ToLower(rule.Listen)
			if opt[key][rule.Listen], err = url.Parse(rule.Target); nil != err {
				hasError = true
				log.Println("parse rule error:", rule.Target, err)

				continue
			}
			if ("tcp" == scheme && "" == port) || ("tcp" == opt[key][rule.Listen].Scheme && "" == opt[key][rule.Listen].Port()) {
				hasError = true
				log.Println("tcp miss port:", rule)

				continue
			}
		} else {
			hasError = true
			log.Println("parse rule error:", rule.Target, err)
		}
	}

	if !hasError {
		for k, v := range opt {
			pair = strings.Split(k, ":")
			listen = ":" + pair[1]

			if "https" == pair[0] || "tcp" == pair[0] {
				if 0 == len(v) || len(v) > 1 {
					hasError = true
					log.Println("listen error:", k, " is not valid")
				} else {
					for _, v1 := range v {
						target = v1.Host
					}

					handle = append(handle, NewTCPProxy(listen, target, proxy, time.Duration(dialTimeout), time.Duration(keepAliveInterval), signals))
				}
			} else {
				handle = append(handle, NewHTTPProxy(listen, v))
			}
		}

		if (!hasError) && (len(handle) > 0) {
			wg.Add(len(handle))
			for _, h := range handle {
				go h.Run()
			}
		}

		wg.Wait()
	}
}
