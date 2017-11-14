// Copyright 2016 Ajit Yagaty
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

package client

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"fmt"
	"github.com/ArthurHlt/go-kairosdb/builder"
	"github.com/ArthurHlt/go-kairosdb/response"
	"regexp"
	"time"
)

var (
	api_version                  = "/api/v1"
	datapoints_ep                = api_version + "/datapoints"
	query_ep                     = api_version + "/datapoints/query"
	version_ep                   = api_version + "/version"
	health_ep                    = api_version + "/health/check"
	metricnames_ep               = api_version + "/metricnames"
	tagnames_ep                  = api_version + "/tagnames"
	tagvalues_ep                 = api_version + "/tagvalues"
	delmetric_ep                 = api_version + "/metric/"
	cacheTTL       time.Duration = time.Second * 30
)

type CacheElem struct {
	resp         *response.GetResponse
	lastRetrieve time.Time
}

type optSetter func(c *httpClient)

func NetHttpClient(cli *http.Client) optSetter {
	return func(c *httpClient) {
		c.cli = cli
	}
}

// This is the type that implements the Client interface.
type httpClient struct {
	serverAddress string
	cli           *http.Client
	cacheStore    map[string]CacheElem
}

func NewHttpClient(serverAddress string, setters ...optSetter) Client {
	c := &httpClient{
		serverAddress: serverAddress,
		cli:           &http.Client{},
		cacheStore:    make(map[string]CacheElem),
	}
	for _, s := range setters {
		s(c)
	}
	return c
}

func (hc *httpClient) cache(key string, fallback func() (*response.GetResponse, error)) (*response.GetResponse, error) {
	if elem, ok := hc.cacheStore[key]; ok {
		whenExpire := elem.lastRetrieve.Add(cacheTTL)
		if whenExpire.After(time.Now()) {
			return elem.resp, nil
		}
	}
	resp, err := fallback()
	if err != nil {
		return nil, err
	}
	hc.cacheStore[key] = CacheElem{resp, time.Now()}
	return resp, nil
}

// Returns a list of all metrics names.
func (hc *httpClient) GetMetricNames() (*response.GetResponse, error) {
	return hc.cache("metric_names", func() (*response.GetResponse, error) {
		return hc.get(hc.serverAddress + metricnames_ep)
	})
}

func (hc *httpClient) GetMetricNamesNeq(metricName string) ([]string, error) {
	metricNames, err := hc.GetMetricNames()
	if err != nil {
		return []string{}, err
	}
	finalNames := make([]string, 0)
	for _, m := range metricNames.Results {
		if m == metricName {
			continue
		}
		finalNames = append(finalNames, m)
	}
	return finalNames, nil
}

func (hc *httpClient) GetMetricNamesReg(metricNameReg string, neq bool) ([]string, error) {
	metricNames, err := hc.GetMetricNames()
	if err != nil {
		return []string{}, err
	}
	reg := regexp.MustCompile(fmt.Sprintf("/^%s/", metricNameReg))
	finalNames := make([]string, 0)
	for _, m := range metricNames.Results {
		if reg.MatchString(m) == neq {
			continue
		}
		finalNames = append(finalNames, m)
	}
	return finalNames, nil
}

func (hc *httpClient) GetTagNamesReg(tagNameReg string, neq bool) ([]string, error) {
	tagNames, err := hc.GetTagNames()
	if err != nil {
		return []string{}, err
	}
	reg := regexp.MustCompile(fmt.Sprintf("/^%s/", tagNameReg))
	finalNames := make([]string, 0)
	for _, t := range tagNames.Results {
		if reg.MatchString(t) == neq {
			continue
		}
		finalNames = append(finalNames, t)
	}
	return finalNames, nil
}

// Returns a list of all tag names.
func (hc *httpClient) GetTagNames() (*response.GetResponse, error) {
	return hc.cache("tag_names", func() (*response.GetResponse, error) {
		return hc.get(hc.serverAddress + tagnames_ep)
	})
}

// Returns a list of all tag values.
func (hc *httpClient) GetTagValues() (*response.GetResponse, error) {
	return hc.cache("tag_values", func() (*response.GetResponse, error) {
		return hc.get(hc.serverAddress + tagvalues_ep)
	})
}

func (hc *httpClient) GetTagValuesNeq(tagValue string) ([]string, error) {
	tagValNames, err := hc.GetTagValues()
	if err != nil {
		return []string{}, err
	}
	finalNames := make([]string, 0)
	for _, t := range tagValNames.Results {
		if t == tagValue {
			continue
		}
		finalNames = append(finalNames, t)
	}
	return finalNames, nil
}

func (hc *httpClient) GetTagValuesReg(tagValueReg string, neq bool) ([]string, error) {
	tagValNames, err := hc.GetTagValues()
	if err != nil {
		return []string{}, err
	}
	reg := regexp.MustCompile(fmt.Sprintf("/^%s/", tagValueReg))
	finalNames := make([]string, 0)
	for _, v := range tagValNames.Results {
		if reg.MatchString(v) == neq {
			continue
		}
		finalNames = append(finalNames, v)
	}
	return finalNames, nil
}

// Queries KairosDB using the query built using builder.
func (hc *httpClient) Query(qb builder.QueryBuilder) (*response.QueryResponse, error) {
	// Get the JSON representation of the query.
	data, err := qb.Build()
	if err != nil {
		return nil, err
	}

	return hc.postQuery(hc.serverAddress+query_ep, data)
}

// Sends metrics from the builder to the KairosDB server.
func (hc *httpClient) PushMetrics(mb builder.MetricBuilder) (*response.Response, error) {
	data, err := mb.Build()
	if err != nil {
		return nil, err
	}

	return hc.postData(hc.serverAddress+datapoints_ep, data)
}

// Deletes a metric. This is the metric and all its datapoints.
func (hc *httpClient) DeleteMetric(name string) (*response.Response, error) {
	return hc.deleteQuery(hc.serverAddress + delmetric_ep + name)
}

// TODO: Deletes data in KairosDB using the query built by the builder.
func (hc *httpClient) Delete(builder builder.QueryBuilder) (*response.Response, error) {
	return nil, nil
}

// Checks the health of the KairosDB Server.
func (hc *httpClient) HealthCheck() (*response.Response, error) {
	resp, err := hc.sendRequest(hc.serverAddress+health_ep, "GET")
	if err != nil {
		return nil, err
	}

	r := &response.Response{}
	r.SetStatusCode(resp.StatusCode)
	return r, nil
}

func (hc *httpClient) sendRequest(url, method string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("accept", "application/json")

	return hc.cli.Do(req)
}

func (hc *httpClient) httpRespToResponse(httpResp *http.Response) (*response.Response, error) {
	resp := &response.Response{}
	resp.SetStatusCode(httpResp.StatusCode)

	if httpResp.StatusCode != http.StatusNoContent {
		// If the request has failed, then read the response body.
		defer httpResp.Body.Close()
		contents, err := ioutil.ReadAll(httpResp.Body)
		if err != nil {
			return nil, err
		} else {
			// Unmarshal the contents into Response object.
			err = json.Unmarshal(contents, resp)
			if err != nil {
				return nil, err
			}
		}
	}

	return resp, nil
}

func (hc *httpClient) httpRespToQueryResponse(httpResp *http.Response) (*response.QueryResponse, error) {
	// Read the HTTP response body.
	defer httpResp.Body.Close()
	contents, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	qr := response.NewQueryResponse(httpResp.StatusCode)

	// Unmarshal the contents into QueryResponse object.
	err = json.Unmarshal(contents, qr)
	if err != nil {
		return nil, err
	}

	return qr, nil
}

func (hc *httpClient) get(url string) (*response.GetResponse, error) {
	resp, err := hc.sendRequest(url, "GET")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	} else {
		gr := response.NewGetResponse(resp.StatusCode)

		err = json.Unmarshal(contents, gr)
		if err != nil {
			return nil, err
		}

		return gr, nil
	}
}

func (hc *httpClient) postData(url string, data []byte) (*response.Response, error) {
	resp, err := hc.cli.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return hc.httpRespToResponse(resp)
}

func (hc *httpClient) postQuery(url string, data []byte) (*response.QueryResponse, error) {
	resp, err := hc.cli.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	return hc.httpRespToQueryResponse(resp)
}

func (hc *httpClient) deleteQuery(url string) (*response.Response, error) {
	resp, err := hc.sendRequest(url, "DELETE")
	if err != nil {
		return nil, err
	}

	return hc.httpRespToResponse(resp)
}
