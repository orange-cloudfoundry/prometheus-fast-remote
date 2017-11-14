// Copyright 2017 Orange
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"flag"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"time"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yml", "Set config file path.")
	flag.Parse()

	config, err := LoadFile(configPath)
	if err != nil {
		log.Panic(err)
	}
	adapter := NewKairosAdapter(config.KairosUrl, createClient(config.SkipInsecure))
	log.Infof("Server is started and listen at %s\n", config.ListenAddr)
	http.ListenAndServe(config.ListenAddr, NewAdapterHandler(adapter, config.Workers))
}
func createClient(skipInsecure bool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipInsecure,
			},
		},
	}
}
