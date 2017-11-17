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
	"fmt"
	"github.com/ArthurHlt/go-kairosdb/builder"
	kclient "github.com/ArthurHlt/go-kairosdb/client"
	"github.com/ArthurHlt/go-kairosdb/response"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	log "github.com/sirupsen/logrus"
	"math"
	"net/http"
	"strings"
	"time"
)

type Adapter interface {
	Write(s *model.Sample) error
	Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error)
}

type KairosAdapter struct {
	client kclient.Client
}

func NewKairosAdapter(kairosUrl string, client *http.Client) *KairosAdapter {
	return &KairosAdapter{kclient.NewHttpClient(kairosUrl, kclient.NetHttpClient(client))}
}
func (a KairosAdapter) mergeResult(labelsToSeries map[string]*prompb.TimeSeries, results []response.Queries) error {
	for _, r := range results {
		for _, s := range r.ResultsArr {
			k := a.concatLabels(s.Tags)
			ts, ok := labelsToSeries[k]
			if !ok {
				ts = &prompb.TimeSeries{
					Labels: a.tagsToLabelPairs(s.Name, s.Tags),
				}
				labelsToSeries[k] = ts
			}

			samples, err := a.valuesToSamples(s.DataPoints)
			if err != nil {
				return err
			}

			ts.Samples = a.mergeSamples(ts.Samples, samples)
		}
	}
	return nil
}

func (KairosAdapter) mergeSamples(a, b []*prompb.Sample) []*prompb.Sample {
	result := make([]*prompb.Sample, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].Timestamp < b[j].Timestamp {
			result = append(result, a[i])
			i++
		} else if a[i].Timestamp > b[j].Timestamp {
			result = append(result, b[j])
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)
	return result
}

func (KairosAdapter) valuesToSamples(datapoints []builder.DataPoint) ([]*prompb.Sample, error) {
	samples := make([]*prompb.Sample, 0, len(datapoints))
	for _, datapoint := range datapoints {
		v, err := datapoint.Float64Value()
		if err != nil {
			return nil, err
		}
		samples = append(samples, &prompb.Sample{
			Timestamp: datapoint.Timestamp(),
			Value:     v,
		})
	}
	return samples, nil
}
func (KairosAdapter) tagsToLabelPairs(name string, tags map[string][]string) []*prompb.Label {
	pairs := make([]*prompb.Label, 0, len(tags))
	for k, values := range tags {
		if len(values) == 0 {
			// I don't know what i'm doing here
			// If we select metrics with different sets of labels names,
			// InfluxDB returns *all* possible tag names on all returned
			// series, with empty tag values on series where they don't
			// apply. In Prometheus, an empty label value is equivalent
			// to a non-existent label, so we just skip empty ones here
			// to make the result correct.
			continue
		}
		for _, v := range values {
			pairs = append(pairs, &prompb.Label{
				Name:  k,
				Value: v,
			})
		}
	}
	pairs = append(pairs, &prompb.Label{
		Name:  model.MetricNameLabel,
		Value: name,
	})
	return pairs
}

func (KairosAdapter) concatLabels(labels map[string][]string) string {
	// 0xff cannot cannot occur in valid UTF-8 sequences, so use it
	// as a separator here.
	separator := "\xff"
	pairs := make([]string, 0, len(labels))
	for k, v := range labels {
		pairs = append(pairs, k+separator+strings.Join(v, separator))
	}
	return strings.Join(pairs, separator)
}
func (a KairosAdapter) Read(req *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	labelsToSeries := map[string]*prompb.TimeSeries{}
	for _, q := range req.Queries {
		qbuilder, err := a.buildQuery(q)
		if err != nil {
			return nil, err
		}

		resp, err := a.client.Query(qbuilder)
		if err != nil {
			return nil, err
		}
		if resp.Errors != nil && len(resp.Errors) > 0 {
			return nil, fmt.Errorf(strings.Join(resp.Errors, "\n"))
		}

		if err = a.mergeResult(labelsToSeries, resp.QueriesArr); err != nil {
			return nil, err
		}
	}

	resp := prompb.ReadResponse{
		Results: []*prompb.QueryResult{
			{Timeseries: make([]*prompb.TimeSeries, 0, len(labelsToSeries))},
		},
	}
	for _, ts := range labelsToSeries {
		resp.Results[0].Timeseries = append(resp.Results[0].Timeseries, ts)
	}
	return &resp, nil
}
func (a KairosAdapter) Write(s *model.Sample) error {
	v := float64(s.Value)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		log.Debug("Skiping sample, kairosdb doesn't support NaN or infinite value.")
		return nil
	}
	mb := builder.NewMetricBuilder()
	metricName := "none"
	tags := make(map[string]string)
	for name, value := range s.Metric {
		sVal := string(value)
		sName := string(name)
		if sName == model.MetricNameLabel {
			metricName = sVal
		} else {
			tags[sName] = sVal
		}
	}
	metric := mb.AddMetric(metricName).AddTags(tags)
	metric.AddType("double")
	metric.AddDataPoint(makeTimestamp(s.Timestamp), v)

	_, err := a.client.PushMetrics(mb)
	return err
}
func (a KairosAdapter) buildQuery(q *prompb.Query) (builder.QueryBuilder, error) {
	// Note: GetMetricNamesNeq, GetMetricNamesReg, GetTagValuesNeq, GetTagValuesReg are cached for 30 seconds by clients to make things faster

	a.client.GetMetricNames()
	qBuilder := builder.NewQueryBuilder()
	qBuilder.SetAbsoluteStart(msToTime(q.StartTimestampMs))
	qBuilder.SetAbsoluteEnd(msToTime(q.EndTimestampMs))

	metricNames := make([]string, 0)
	tags := make(map[string][]string)
	for _, m := range q.Matchers {
		if m.Name == model.MetricNameLabel {
			switch m.Type {
			case prompb.LabelMatcher_EQ:
				metricNames = append(metricNames, m.Value)
			case prompb.LabelMatcher_NEQ:
				reNames, err := a.client.GetMetricNamesNeq(m.Value)
				if err != nil {
					return nil, err
				}
				metricNames = append(metricNames, reNames...)
			case prompb.LabelMatcher_RE:
				reNames, err := a.client.GetMetricNamesReg(m.Value, false)
				if err != nil {
					return nil, err
				}
				metricNames = append(metricNames, reNames...)
			case prompb.LabelMatcher_NRE:
				reNames, err := a.client.GetMetricNamesReg(m.Value, true)
				if err != nil {
					return nil, err
				}
				metricNames = append(metricNames, reNames...)
			default:
				return nil, fmt.Errorf("unknown match type %v", m.Type)
			}
			continue
		}
		if _, ok := tags[m.Name]; !ok {
			tags[m.Name] = make([]string, 0)
		}
		switch m.Type {
		case prompb.LabelMatcher_EQ:
			tags[m.Name] = append(tags[m.Name], m.Value)
		case prompb.LabelMatcher_NEQ:
			neqValues, err := a.client.GetTagValuesNeq(m.Value)
			if err != nil {
				return nil, err
			}
			tags[m.Name] = append(tags[m.Name], neqValues...)
		case prompb.LabelMatcher_RE:
			reValues, err := a.client.GetTagValuesReg(m.Value, false)
			if err != nil {
				return nil, err
			}
			tags[m.Name] = append(tags[m.Name], reValues...)
		case prompb.LabelMatcher_NRE:
			reValues, err := a.client.GetTagValuesReg(m.Value, true)
			if err != nil {
				return nil, err
			}
			tags[m.Name] = append(tags[m.Name], reValues...)
		default:
			return nil, fmt.Errorf("unknown match type %v", m.Type)
		}
	}
	for _, name := range metricNames {
		qBuilder.AddMetric(name).AddTags(tags)
	}
	return qBuilder, nil
}
func makeTimestamp(timestamp model.Time) int64 {
	return timestamp.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}
