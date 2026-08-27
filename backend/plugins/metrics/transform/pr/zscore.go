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
	"fmt"
	"math"
	"sort"

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
)

const zScoreLabel = `PR Cycle Time Z-Score <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.hq24rkkp1hu6" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const zScoreDesc = "Z-score of PR cycle time (created to close/merge) over a baseline. " +
	"Green = Fast (z < -1), Gray = Average (-1 ≤ z ≤ 1), Red = Slow (z > 1). " +
	"Baseline is using a 90 days lookback from the end of the selected time range. " +
	"Click on bar to show PR details."

type zscorePREntry struct {
	ID     string  `json:"id"`
	URL    string  `json:"url"`
	ZScore float64 `json:"z_score"`
	Days   float64 `json:"days"`
}

// BuildZScore computes log-normal z-scores for each PR in the current period,
// using lookbackPRs to establish the baseline (mu_log, sigma_log).
// lookbackPRs typically covers 90 days; currentPRs is the selected period.
func BuildZScore(lookbackPRs, currentPRs []qpr.PRRow) model.MetricResponse {
	muLog, sigmaLog := computeBaseline(lookbackPRs)

	type result struct {
		pr       qpr.PRRow
		z        float64
		minutes  float64
		category string
	}

	results := make([]result, 0, len(currentPRs))
	for _, pr := range currentPRs {
		if !validPR(pr) {
			continue
		}
		ct := cycleTimeHours(pr)
		if ct < 0 {
			continue
		}
		minutes := ct * 60
		logT := math.Log(minutes + 1)
		z := (logT - muLog) / sigmaLog

		cat := "Average"
		if z < -1.0 {
			cat = "Fast"
		} else if z > 1.0 {
			cat = "Slow"
		}
		results = append(results, result{pr: pr, z: z, minutes: minutes, category: cat})
	}

	// Aggregate by category
	cats := []string{"Fast", "Average", "Slow"}
	counts := map[string]int{"Fast": 0, "Average": 0, "Slow": 0}
	byCategory := map[string][]result{}
	for _, r := range results {
		counts[r.category]++
		byCategory[r.category] = append(byCategory[r.category], r)
	}

	// Median cycle time per category (in days)
	catMedianDays := map[string]float64{}
	for _, cat := range cats {
		mins := make([]float64, len(byCategory[cat]))
		for i, r := range byCategory[cat] {
			mins[i] = r.minutes
		}
		sort.Float64s(mins)
		m := median(mins)
		catMedianDays[cat] = round1(m / (60 * 24))
	}

	// Build bar labels with median annotation
	barLabels := make([]string, len(cats))
	for i, cat := range cats {
		if counts[cat] > 0 {
			barLabels[i] = cat + " (median: " + ftoa(catMedianDays[cat]) + "d)"
		} else {
			barLabels[i] = cat
		}
	}

	// Build extra drill-down lists
	toList := func(cat string, reverse bool) []zscorePREntry {
		list := byCategory[cat]
		if reverse {
			sort.Slice(list, func(i, j int) bool { return list[i].z < list[j].z })
		} else {
			sort.Slice(list, func(i, j int) bool { return list[i].z > list[j].z })
		}
		out := make([]zscorePREntry, len(list))
		for i, r := range list {
			out[i] = zscorePREntry{
				ID:     r.pr.Key,
				URL:    r.pr.URL,
				ZScore: round2(r.z),
				Days:   round2(r.minutes / (60 * 24)),
			}
		}
		return out
	}

	barData := []int{counts["Fast"], counts["Average"], counts["Slow"]}
	barColors := []string{
		"rgba(34, 197, 94, 0.8)",
		"rgba(107, 114, 128, 0.8)",
		"rgba(239, 68, 68, 0.8)",
	}
	borderColors := []string{
		"rgba(34, 197, 94, 1)",
		"rgba(107, 114, 128, 1)",
		"rgba(239, 68, 68, 1)",
	}

	extraCfg := []model.ExtraConfig{
		{Location: "appendToAnalytics", Type: "openList", Field: "fast_prs", Template: "Show Fast PRs",
			Labels: []model.ExtraLabel{{Label: "PR ID", Field: "id"}, {Label: "URL", Field: "url"}, {Label: "Z-Score", Field: "z_score"}, {Label: "Cycle Time (days)", Field: "days"}}},
		{Location: "appendToAnalytics", Type: "openList", Field: "average_prs", Template: "Show Average PRs",
			Labels: []model.ExtraLabel{{Label: "PR ID", Field: "id"}, {Label: "URL", Field: "url"}, {Label: "Z-Score", Field: "z_score"}, {Label: "Cycle Time (days)", Field: "days"}}},
		{Location: "appendToAnalytics", Type: "openList", Field: "slow_prs", Template: "Show Slow PRs",
			Labels: []model.ExtraLabel{{Label: "PR ID", Field: "id"}, {Label: "URL", Field: "url"}, {Label: "Z-Score", Field: "z_score"}, {Label: "Cycle Time (days)", Field: "days"}}},
	}

	return model.MetricResponse{
		Type:        "bar",
		Label:       zScoreLabel,
		Description: zScoreDesc,
		LastUpdate:  nowMillis(),
		Data: model.BarData{
			Labels: barLabels,
			Datasets: []model.ChartDataset{
				{
					Label:           "PR count",
					Data:            barData,
					BackgroundColor: barColors,
					BorderColor:     borderColors,
					BorderWidth:     1,
				},
			},
			Extra: &model.ExtraData{
				Config: extraCfg,
				Data: map[string]interface{}{
					"fast_prs":    toList("Fast", true),
					"average_prs": toList("Average", false),
					"slow_prs":    toList("Slow", false),
				},
			},
		},
		Options: map[string]interface{}{
			"indexAxis": "y",
			"scales": map[string]interface{}{
				"x": map[string]interface{}{"title": map[string]interface{}{"display": true, "text": "PR count"}, "beginAtZero": true},
				"y": map[string]interface{}{"title": map[string]interface{}{"display": true, "text": "Category"}},
			},
		},
	}
}

// computeBaseline calculates mu_log and sigma_log from the lookback PRs.
// Returns (mu=0, sigma=1e-10) when no valid PRs are found.
func computeBaseline(prs []qpr.PRRow) (muLog, sigmaLog float64) {
	logTimes := make([]float64, 0, len(prs))
	for _, pr := range prs {
		if !validPR(pr) {
			continue
		}
		ct := cycleTimeHours(pr)
		if ct < 0 {
			continue
		}
		minutes := ct * 60
		logTimes = append(logTimes, math.Log(minutes+1))
	}
	if len(logTimes) == 0 {
		return 0, 1e-10
	}
	sum := 0.0
	for _, v := range logTimes {
		sum += v
	}
	mu := sum / float64(len(logTimes))
	variance := 0.0
	for _, v := range logTimes {
		d := v - mu
		variance += d * d
	}
	variance /= float64(len(logTimes))
	sigma := math.Sqrt(variance)
	if sigma < 1e-10 {
		sigma = 1e-10
	}
	return mu, sigma
}

// ftoa converts a float to its decimal string (e.g. 2.5 → "2.5", 3.0 → "3").
func ftoa(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}
