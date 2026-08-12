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

const productivityLabel = `PR Productivity <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.k72pk27jaev" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const productivityDesc = "This metric assesses the efficiency of work by calculating the percentage of closed PRs that were ultimately merged versus those closed without merging. Only PRs that reached a final state (merged or closed) within the selected time period are included."

// ProductivityEntry is the drill-down row for closed-without-merge PRs.
type ProductivityEntry struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// ProductivitySummary holds aggregate counts shown alongside the doughnut.
type ProductivitySummary struct {
	Merged                       int     `json:"merged"`
	ClosedWithoutMerge           int     `json:"closedWithoutMerge"`
	Total                        int     `json:"total"`
	MergedPercentage             float64 `json:"mergedPercentage"`
	ClosedWithoutMergePercentage float64 `json:"closedWithoutMergePercentage"`
}

// BuildProductivity builds a "doughnut" MetricResponse from raw PR rows.
// It classifies each valid closed PR as merged or closed-without-merge and
// returns percentages for the two segments.
func BuildProductivity(prs []qpr.PRRow) model.MetricResponse {
	var merged, closedNoMerge int
	var closedNoMergePRs []ProductivityEntry

	for _, r := range prs {
		if !validPR(r) {
			continue
		}
		isMerged := r.MergedDate != nil || r.Status == "MERGED"
		isClosed := r.Status == "CLOSED" && r.MergedDate == nil

		if isMerged {
			merged++
		} else if isClosed {
			closedNoMerge++
			closedNoMergePRs = append(closedNoMergePRs, ProductivityEntry{
				ID:  r.Key,
				URL: r.URL,
			})
		}
	}

	total := merged + closedNoMerge

	var mergedPct, closedPct float64
	if total > 0 {
		mergedPct = round1(float64(merged) / float64(total) * 100)
		closedPct = round1(float64(closedNoMerge) / float64(total) * 100)
	}

	if closedNoMergePRs == nil {
		closedNoMergePRs = []ProductivityEntry{}
	}

	return model.MetricResponse{
		Type:        "doughnut",
		Label:       productivityLabel,
		Description: productivityDesc,
		LastUpdate:  nowMillis(),
		Data: model.DoughnutData{
			Labels: []string{"Merged", "Closed w/o Merge"},
			Datasets: []model.ChartDataset{
				{
					Data:            []int{merged, closedNoMerge},
					BackgroundColor: []string{"#3E8635", "#A30000"},
					BorderColor:     []string{"#1E4F18", "#7D1007"},
					BorderWidth:     2,
				},
			},
			Extra: &model.ExtraData{
				Config: []model.ExtraConfig{
					{
						Location: "appendToAnalytics",
						Type:     "openList",
						Field:    "closed_without_merge",
						Template: "Show Closed w/o Merge PRs",
						Labels: []model.ExtraLabel{
							{Label: "PR ID", Field: "id"},
							{Label: "URL", Field: "url"},
						},
					},
				},
				Data: map[string]interface{}{"closed_without_merge": closedNoMergePRs},
			},
		},
		Options: map[string]interface{}{
			"responsive":          true,
			"maintainAspectRatio": false,
			"cutout":              "60%",
			"plugins": map[string]interface{}{
				"legend": map[string]interface{}{"position": "bottom"},
			},
		},
		Summary: ProductivitySummary{
			Merged:                       merged,
			ClosedWithoutMerge:           closedNoMerge,
			Total:                        total,
			MergedPercentage:             mergedPct,
			ClosedWithoutMergePercentage: closedPct,
		},
	}
}
