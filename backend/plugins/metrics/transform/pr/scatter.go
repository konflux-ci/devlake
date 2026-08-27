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
	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
)

// bucket defines a cycle-time range and its display label.
type bucket struct {
	maxHours float64
	label    string
}

var cycleTimeBuckets = []bucket{
	{1, "< 1h"},
	{4, "1–4h"},
	{8, "4–8h"},
	{24, "8–24h"},
	{48, "1–2d"},
	{72, "2–3d"},
	{168, "3–7d"},
	{336, "1–2w"},
	{672, "2–4w"},
	{1<<62 - 1, "> 4w"},
}

// BuildScatter builds a "line/cycleTimeDistribution" MetricResponse from raw
// PR rows.  Each closed non-bot non-draft PR is bucketed by its cycle time
// into one of ten ranges, with separate counts for merged and abandoned.
func BuildScatter(prs []qpr.PRRow) model.MetricResponse {
	merged := make([]int, len(cycleTimeBuckets))
	abandoned := make([]int, len(cycleTimeBuckets))

	for _, pr := range prs {
		if !validPR(pr) {
			continue
		}
		ct := cycleTimeHours(pr)
		if ct < 0 {
			continue
		}

		idx := len(cycleTimeBuckets) - 1
		for j, b := range cycleTimeBuckets {
			if ct < b.maxHours {
				idx = j
				break
			}
		}

		if pr.Status == "MERGED" {
			merged[idx]++
		} else {
			abandoned[idx]++
		}
	}

	labels := make([]string, len(cycleTimeBuckets))
	for i, b := range cycleTimeBuckets {
		labels[i] = b.label
	}

	return model.MetricResponse{
		Type:        "line",
		Subtype:     "cycleTimeDistribution",
		Label:       "PR Cycle Time Distribution",
		Description: "Distribution of PR cycle times across time buckets. Shows how many merged and abandoned PRs fall into each cycle time range.",
		LastUpdate:  nowMillis(),
		Data: map[string]interface{}{
			"labels": labels,
			"datasets": []map[string]interface{}{
				{
					"label":            "Merged",
					"data":             merged,
					"borderColor":      "rgba(75, 192, 192, 1)",
					"backgroundColor":  "rgba(75, 192, 192, 0.2)",
					"fill":             true,
					"tension":          0.3,
					"pointRadius":      4,
					"pointHoverRadius": 6,
				},
				{
					"label":            "Abandoned",
					"data":             abandoned,
					"borderColor":      "rgba(255, 99, 132, 1)",
					"backgroundColor":  "rgba(255, 99, 132, 0.2)",
					"fill":             true,
					"tension":          0.3,
					"pointRadius":      4,
					"pointHoverRadius": 6,
				},
			},
		},
		Options: map[string]interface{}{
			"scales": map[string]interface{}{
				"x": map[string]interface{}{
					"title": map[string]interface{}{"display": true, "text": "Cycle Time"},
				},
				"y": map[string]interface{}{
					"title":       map[string]interface{}{"display": true, "text": "Number of closed PRs"},
					"beginAtZero": true,
					"ticks":       map[string]interface{}{"stepSize": 1, "precision": 0},
				},
			},
			"plugins": map[string]interface{}{
				"legend": map[string]interface{}{
					"display":  true,
					"position": "top",
					"labels": map[string]interface{}{
						"usePointStyle": true,
						"pointStyle":    "circle",
						"padding":       15,
					},
				},
			},
		},
	}
}
