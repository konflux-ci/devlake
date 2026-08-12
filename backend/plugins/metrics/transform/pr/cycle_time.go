/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pr

import (
	"sort"

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
)

const cycleTimeLabel = `Median Cycle Time (hours) <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.d6lkr552tyub" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const cycleTimeDesc = "Each dot shows the median cycle time for PRs closed on that day. " +
	"The trend line is a 7-day rolling average, smoothing out daily spikes to show whether cycle time is trending up or down. " +
	"Log scale compresses outliers so daily patterns are easier to read."

// BuildCycleTime aggregates per-day median cycle times from raw PR rows and
// produces a "line" MetricResponse with two datasets: daily median and 7-day rolling average.
func BuildCycleTime(prs []qpr.PRRow) model.MetricResponse {
	// group cycle times by close date
	byDay := map[string][]float64{}
	for _, pr := range prs {
		if !validPR(pr) {
			continue
		}
		ct := cycleTimeHours(pr)
		if ct < 0 {
			continue
		}
		ed := endDate(pr)
		day := ed.UTC().Format("2006-01-02")
		byDay[day] = append(byDay[day], ct)
	}

	// stable sort by day
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)

	labels := make([]string, 0, len(days))
	daily := make([]float64, 0, len(days))
	for _, d := range days {
		med := median(byDay[d])
		labels = append(labels, d)
		daily = append(daily, round1(med))
	}

	// 7-day rolling average
	windowSize := 7
	rolling := make([]float64, len(daily))
	for i := range daily {
		start := i - windowSize + 1
		if start < 0 {
			start = 0
		}
		sum := 0.0
		for _, v := range daily[start : i+1] {
			sum += v
		}
		rolling[i] = round1(sum / float64(i-start+1))
	}

	return model.MetricResponse{
		Type:        "line",
		Label:       cycleTimeLabel,
		Description: cycleTimeDesc,
		LastUpdate:  nowMillis(),
		Data: model.LineData{
			Labels: labels,
			Datasets: []model.ChartDataset{
				{
					Label:           "Daily Median (hours)",
					Data:            daily,
					BackgroundColor: "rgba(75, 192, 192, 0.7)",
					BorderColor:     "rgba(75, 192, 192, 1)",
				},
				{
					Label:       "7-day Rolling Avg (hours)",
					Data:        rolling,
					BorderColor: "rgb(255, 99, 132)",
					BorderWidth: 2,
				},
			},
		},
		Options: map[string]interface{}{
			"scales": map[string]interface{}{
				"y": map[string]interface{}{
					"type":  "logarithmic",
					"title": map[string]interface{}{"display": true, "text": "Hours (log scale)"},
				},
			},
		},
	}
}
