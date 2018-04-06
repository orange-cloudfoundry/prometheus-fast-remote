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
	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
	"encoding/json"
)

type adapterHandler struct {
	adapter Adapter
	workers int
}

type HealthResponse struct {
	Adapter string             `json:"adapter"`
	Tsdb    TsdbHealthResponse `json:"tsdb"`
}

type TsdbHealthResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func NewAdapterHandler(adapter Adapter, workers int) http.Handler {
	adaptHandler := &adapterHandler{adapter, workers}
	r := mux.NewRouter()
	r.HandleFunc("/write", adaptHandler.write)
	r.HandleFunc("/read", adaptHandler.read)
	r.HandleFunc("/health", adaptHandler.health)
	return r
}

func (h adapterHandler) health(w http.ResponseWriter, r *http.Request) {
	healthy := h.adapter.Healthy()
	statusCode := http.StatusOK
	status := "ok"
	if !healthy {
		status = "ko"
		statusCode = http.StatusInternalServerError
	}

	w.WriteHeader(statusCode)
	w.Header().Add("Content-Type", "application/json")

	b, _ := json.MarshalIndent(HealthResponse{
		Adapter: "ok",
		Tsdb: TsdbHealthResponse{
			Name:   h.adapter.Name(),
			Status: status,
		},
	}, "", "\t")
	w.Write(b)
}

func (h adapterHandler) read(w http.ResponseWriter, r *http.Request) {
	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error("Read error: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		log.Error("Decode error: " + err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req prompb.ReadRequest
	err = proto.Unmarshal(reqBuf, &req)
	if err != nil {
		log.Error("Unmarshal error: " + err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := h.adapter.Read(&req)
	if err != nil {
		entry := log.WithField("query", req)
		entry.Warn("Error executing query: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Content-Encoding", "snappy")

	compressed = snappy.Encode(nil, data)
	if _, err := w.Write(compressed); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h adapterHandler) write(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	start := time.Now()
	rmtIp := remoteIp(r)
	entry := log.WithField("content_length", r.ContentLength).
		WithField("ip", rmtIp)
	entry.Debug("Sending data to kairos ...")

	compressed, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Error("Error when getting data from response:" + err.Error())
		return
	}

	reqBuf, err := snappy.Decode(nil, compressed)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error("Error when decoding data:" + err.Error())
		return
	}

	var req prompb.WriteRequest
	err = proto.Unmarshal(reqBuf, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Error("Error when unmarshalling decoded data:" + err.Error())
		return
	}
	samples := protoToSamples(&req)

	// Get faster as possible by creating worker pool to send data through adapter
	jobsSample := make(chan *model.Sample, 100)
	var wg sync.WaitGroup
	wg.Add(h.workers)
	for w := 1; w <= h.workers; w++ {
		go func() {
			defer wg.Done()
			h.writeWorker(w, jobsSample)
		}()
	}

	for _, s := range samples {
		entry.WithField("sample_ts", s.Timestamp).WithField("value", s.Value).
			Debugf("Sending sample with labels: %s", s.Metric.String())
		jobsSample <- s
	}
	close(jobsSample)
	wg.Wait()

	entry.Debugf(
		"Finished sending data to kairos in %s .",
		time.Since(start).String(),
	)
}
func (h adapterHandler) writeWorker(id int, jobsSample <-chan *model.Sample) {
	entry := log.WithField("id", id)
	entry.Debug("Starting write worker...")
	for sample := range jobsSample {
		err := h.adapter.Write(sample)
		if err != nil {
			log.Error("Error when sending one sample to kairos, skipping the sample:" + err.Error())
		}
	}
	entry.Debug("Finished write worker.")
}

func remoteIp(r *http.Request) string {
	// getting ip from header fed by reverse proxy if set
	if r.Header.Get("X-Forwarded-For") != "" {
		return strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]
	}
	return r.RemoteAddr
}
