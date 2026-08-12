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

const stagesLabel = `Where PRs Spend Their Time (averages)  <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.ycp7wxl34qwz" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const stagesDesc = "Percentage of average cycle time spent in each stage. " +
	"Pickup: PR creation to first review. " +
	"Review: first review to last approval. " +
	"Integration: last approval to merge. " +
	"Only merged PRs included."

// stagePREntry is a drill-down row for a single PR in one stage.
type stagePREntry struct {
	ID    string  `json:"id"`
	URL   string  `json:"url"`
	Hours float64 `json:"hours"`
}

// BuildStages builds a "doughnut" MetricResponse from per-PR stage rows.
// Averages are computed in Go; per-PR lists are included for drill-down.
func BuildStages(prs []qpr.StagePRRow) model.MetricResponse {
	if len(prs) == 0 {
		return model.MetricResponse{
			Type:        "doughnut",
			Label:       stagesLabel,
			Description: stagesDesc,
			LastUpdate:  nowMillis(),
			Data: map[string]interface{}{
				"labels":   []string{"Pickup (Created ➜ First Review)", "Review (First Review ➜ Last Approval)", "Integration (Last Approval ➜ Merged)"},
				"datasets": []map[string]interface{}{{"data": []float64{0, 0, 0}, "backgroundColor": []string{"#81c784", "#64b5f6", "#ffb74d"}}},
			},
		}
	}

	var sumPickup, sumReview, sumIntegration float64
	pickupPRs := make([]stagePREntry, 0, len(prs))
	reviewPRs := make([]stagePREntry, 0, len(prs))
	integrationPRs := make([]stagePREntry, 0, len(prs))

	for _, pr := range prs {
		sumPickup += pr.PickupHours
		sumReview += pr.ReviewHours
		sumIntegration += pr.IntegrationHours

		pickupPRs = append(pickupPRs, stagePREntry{ID: pr.Key, URL: pr.URL, Hours: round2(pr.PickupHours)})
		reviewPRs = append(reviewPRs, stagePREntry{ID: pr.Key, URL: pr.URL, Hours: round2(pr.ReviewHours)})
		integrationPRs = append(integrationPRs, stagePREntry{ID: pr.Key, URL: pr.URL, Hours: round2(pr.IntegrationHours)})
	}

	n := float64(len(prs))
	avgPickup := round2(sumPickup / n)
	avgReview := round2(sumReview / n)
	avgIntegration := round2(sumIntegration / n)

	return model.MetricResponse{
		Type:        "doughnut",
		Label:       stagesLabel,
		Description: stagesDesc,
		LastUpdate:  nowMillis(),
		Data: map[string]interface{}{
			"extra": map[string]interface{}{
				"config": []map[string]interface{}{
					{
						"location": "appendToAnalytics",
						"type":     "openList",
						"field":    "pickup_prs",
						"template": "Show Pickup PRs",
						"labels": []map[string]string{
							{"label": "PR ID", "field": "id"},
							{"label": "URL", "field": "url"},
							{"label": "Pickup (hours)", "field": "hours"},
						},
					},
					{
						"location": "appendToAnalytics",
						"type":     "openList",
						"field":    "review_prs",
						"template": "Show Review PRs",
						"labels": []map[string]string{
							{"label": "PR ID", "field": "id"},
							{"label": "URL", "field": "url"},
							{"label": "Review (hours)", "field": "hours"},
						},
					},
					{
						"location": "appendToAnalytics",
						"type":     "openList",
						"field":    "integration_prs",
						"template": "Show Integration PRs",
						"labels": []map[string]string{
							{"label": "PR ID", "field": "id"},
							{"label": "URL", "field": "url"},
							{"label": "Integration (hours)", "field": "hours"},
						},
					},
				},
				"data": map[string]interface{}{
					"pickup_prs":      pickupPRs,
					"review_prs":      reviewPRs,
					"integration_prs": integrationPRs,
				},
			},
			"labels": []string{
				"Pickup (Created ➜ First Review)",
				"Review (First Review ➜ Last Approval)",
				"Integration (Last Approval ➜ Merged)",
			},
			"datasets": []map[string]interface{}{
				{
					"data":            []float64{avgPickup, avgReview, avgIntegration},
					"backgroundColor": []string{"#81c784", "#64b5f6", "#ffb74d"},
				},
			},
			"note": "Only merged PRs are included. PRs merged without review or approval are treated as approved at merge time.",
		},
	}
}
