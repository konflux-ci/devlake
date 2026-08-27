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

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
)

const keyMetricsLabel = `Key Metrics <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.uq7gf0mhsbga" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const keyMetricsDesc = "The key metrics offer a single-figure snapshot of PR delivery for the team. " +
	"'Throughput' counts PRs merged or closed in the time frame. " +
	"'Median Cycle Time' is the median time from PR creation to close/merge. " +
	"'Opened PRs' counts PRs created during the time frame. " +
	"'Outlier PRs' counts PRs with cycle time exceeding 3x the median, indicating distribution skew. " +
	"When the time range is 30 days or less, each metric shows a comparison against the previous period."

// OutlierEntry is the drill-down row for the outlier PR list.
type OutlierEntry struct {
	ID        string  `json:"id"`
	URL       string  `json:"url"`
	CycleTime float64 `json:"cycle_time"`
}

// BuildKeyMetrics transforms pre-aggregated SQL stats into a "stats" MetricResponse.
// compare is optional (nil = no comparison shown).
// outliers is the list of PRs whose cycle time exceeds 3× median.
func BuildKeyMetrics(
	stats qpr.KeyMetricsStatsRow,
	compare *qpr.KeyMetricsStatsRow,
	outliers []qpr.OutlierPRRow,
) model.MetricResponse {
	// build outlier drill-down entries
	outlierEntries := make([]OutlierEntry, 0, len(outliers))
	for _, o := range outliers {
		outlierEntries = append(outlierEntries, OutlierEntry{
			ID:        o.PRNumber,
			URL:       o.PRURL,
			CycleTime: round1(o.CycleTimeHours),
		})
	}

	// comparison text
	var (
		closedExplain  string
		cycleExplain   string
		openedExplain  string
		outlierExplain string
	)
	if compare != nil {
		closedExplain = compareHTML(stats.TotalClosedPRs, compare.TotalClosedPRs, false,
			fmt.Sprintf("was %.1f", compare.TotalClosedPRs))
		cycleExplain = compareHTML(stats.MedianCycleTime, compare.MedianCycleTime, true,
			fmt.Sprintf("was %.1f", compare.MedianCycleTime))
		openedExplain = compareHTML(stats.OpenedPRs, compare.OpenedPRs, false,
			fmt.Sprintf("was %.1f", compare.OpenedPRs))
		if compare.OutlierPRs > 0 {
			outlierExplain = compareHTML(stats.OutlierPRs, compare.OutlierPRs, true,
				fmt.Sprintf("was %.0f", compare.OutlierPRs))
		}
	}

	return model.MetricResponse{
		Type:        "stats",
		Label:       keyMetricsLabel,
		Description: keyMetricsDesc,
		LastUpdate:  nowMillis(),
		Data: model.StatsData{
			Metrics: []model.StatMetric{
				{Label: "Throughput (total closed PRs)", Explain: closedExplain, Value: math.Round(stats.TotalClosedPRs)},
				{Label: "Median Cycle Time (hours)", Explain: cycleExplain, Value: round1(stats.MedianCycleTime)},
				{Label: "Opened PRs", Explain: openedExplain, Value: math.Round(stats.OpenedPRs)},
				{Label: "Outlier PRs (>3x median)", Explain: outlierExplain, Value: math.Round(stats.OutlierPRs)},
			},
			Extra: &model.ExtraData{
				Config: []model.ExtraConfig{
					{
						Location: "appendToAnalytics",
						Type:     "openList",
						Field:    "outlier",
						Template: "Show Outlier PRs",
						Labels: []model.ExtraLabel{
							{Label: "PR ID", Field: "id"},
							{Label: "URL", Field: "url"},
							{Label: "Cycle Time (hours)", Field: "cycle_time"},
						},
					},
				},
				Data: map[string]interface{}{"outlier": outlierEntries},
			},
		},
	}
}

// compareHTML builds the HTML comparison string shown below each metric card.
// lowerIsBetter flips the colour direction (e.g. cycle time: lower = green).
func compareHTML(current, prev float64, lowerIsBetter bool, prevStr string) string {
	if prev == 0 {
		return ""
	}
	changePct := ((current - prev) / prev) * 100
	absChg := math.Abs(math.Round(changePct*10) / 10)

	var direction, cssClass string
	if changePct < 0 {
		direction = "Down by"
		if lowerIsBetter {
			cssClass = "compareup"
		} else {
			cssClass = "comparedown"
		}
	} else {
		direction = "Up by"
		if lowerIsBetter {
			cssClass = "comparedown"
		} else {
			cssClass = "compareup"
		}
	}
	return fmt.Sprintf("<span class='%s'>%s %.1f%% from previous cycle (%s).</span>",
		cssClass, direction, absChg, prevStr)
}
