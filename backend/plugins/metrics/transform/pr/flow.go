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
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/transform/botfilter"
)

const flowLabel = `PR Stage Flow <a href="https://docs.google.com/document/d/1TbY719t1tl3A_7glZFY4SyShMGHyH-Qxb_jBAf0eh80/edit?tab=t.0#heading=h.n56txnenwb3r" target="_blank" style="text-decoration: none; margin-left: 4px;"><img src="help-icon.svg" alt="Help" style="vertical-align: middle; width: 16px; height: 16px;" /></a>`

const flowDesc = "This visualization shows how PRs move through their lifecycle from creation to final state " +
	"(merged or closed without merge). Only PRs that reached a final state during the selected time period are included. " +
	"It provides a quick, visual overview of where, and with what frequency, state transfers are occurring."

var (
	reApprove = regexp.MustCompile(`/approve([^a-zA-Z0-9_]|$)`)
	reLGTM    = regexp.MustCompile(`/lgtm([^a-zA-Z0-9_]|$)`)
)

// prFlowEntry is the drill-down data stored in extra per Sankey edge.
type prFlowEntry struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// BuildFlow builds a "sankey" MetricResponse from raw PR and review rows.
func BuildFlow(prs []qpr.PRRow, reviews []qpr.ReviewRow) model.MetricResponse {
	// Index reviews by pull_request_id; filter bots
	prReviews := map[string][]qpr.ReviewRow{}
	for _, rev := range reviews {
		if botfilter.IsBot(rev.AuthorName, "", rev.Body) {
			continue
		}
		prReviews[rev.PullRequestID] = append(prReviews[rev.PullRequestID], rev)
	}

	// Filter valid PRs
	validPRs := make([]qpr.PRRow, 0, len(prs))
	for _, pr := range prs {
		if !validPR(pr) {
			continue
		}
		isMerged := pr.Status == "MERGED" && pr.MergedDate != nil
		isAbandoned := !isMerged && (pr.Status == "CLOSED" || pr.ClosedDate != nil)
		if !isMerged && !isAbandoned {
			continue
		}
		validPRs = append(validPRs, pr)
	}

	allTransfers := []string{
		"OPENED->REVIEWED", "OPENED->APPROVED", "OPENED->MERGED", "OPENED->ABANDONED",
		"REVIEWED->APPROVED", "REVIEWED->MERGED", "REVIEWED->ABANDONED",
		"APPROVED->MERGED", "APPROVED->ABANDONED",
	}
	counts := make(map[string]int, len(allTransfers))
	prsByTransfer := make(map[string][]prFlowEntry, len(allTransfers))
	for _, k := range allTransfers {
		counts[k] = 0
		prsByTransfer[k] = nil
	}

	var reviewIntervals []float64

	for _, pr := range validPRs {
		revList := prReviews[pr.ID]

		// Compute review intervals for median
		for i := 1; i < len(revList); i++ {
			d := revList[i].CreatedDate.Sub(revList[i-1].CreatedDate).Hours()
			if d >= 0 {
				reviewIntervals = append(reviewIntervals, d)
			}
		}

		firstApproval := findApproval(pr, revList)

		isMerged := pr.Status == "MERGED" && pr.MergedDate != nil

		// Build timeline
		type event struct {
			state string
			ts    time.Time
		}
		events := []event{{"OPENED", *pr.CreatedDate}}
		if len(revList) > 0 {
			events = append(events, event{"REVIEWED", revList[0].CreatedDate})
		}
		if firstApproval != nil {
			events = append(events, event{"APPROVED", *firstApproval})
		}
		if isMerged {
			events = append(events, event{"MERGED", *pr.MergedDate})
		} else {
			closed := pr.ClosedDate
			if closed == nil {
				closed = pr.CreatedDate
			}
			events = append(events, event{"ABANDONED", *closed})
		}

		// Sort by timestamp
		for i := 1; i < len(events); i++ {
			for j := i; j > 0 && events[j].ts.Before(events[j-1].ts); j-- {
				events[j], events[j-1] = events[j-1], events[j]
			}
		}

		entry := prFlowEntry{ID: pr.Key, URL: pr.URL}
		for i := 0; i < len(events)-1; i++ {
			key := events[i].state + "->" + events[i+1].state
			if _, ok := counts[key]; ok {
				counts[key]++
				prsByTransfer[key] = append(prsByTransfer[key], entry)
			}
		}
	}

	total := len(validPRs)

	// Build flow data
	stateLabels := map[string]string{
		"OPENED": "Opened", "REVIEWED": "Reviewed", "APPROVED": "Approved",
		"MERGED": "Merged", "ABANDONED": "Abandoned",
	}
	flowData := make([]model.SankeyFlow, 0)
	extraElements := map[string]model.ExtraElement{}
	extraData := map[string]interface{}{}

	for _, transfer := range allTransfers {
		cnt := counts[transfer]
		if cnt == 0 {
			continue
		}
		parts := strings.SplitN(transfer, "->", 2)
		from, to := strings.ToLower(parts[0]), strings.ToLower(parts[1])
		pct := 0.0
		if total > 0 {
			pct = math.Round((float64(cnt)/float64(total))*1000) / 10
		}
		flowData = append(flowData, model.SankeyFlow{From: from, To: to, Flow: pct, Count: cnt})

		dataField := from + "_to_" + to
		extraElements[from+"->"+to] = model.ExtraElement{
			Field:    dataField,
			Template: stateLabels[parts[0]] + " → " + stateLabels[parts[1]] + " PRs",
		}
		extraData[dataField] = prsByTransfer[transfer]
	}

	return model.MetricResponse{
		Type:        "sankey",
		Label:       flowLabel,
		Description: flowDesc,
		LastUpdate:  nowMillis(),
		Data: model.SankeyData{
			MedianReviewInterval: round1(median(reviewIntervals)),
			Extra: &model.ExtraData{
				Config: []model.ExtraConfig{
					{
						Location: "inDiagram",
						Type:     "openList",
						Elements: extraElements,
					},
				},
				Data: extraData,
			},
			Datasets: []model.SankeyDataset{
				{
					Label: "PR Lifecycle Flow",
					Data:  flowData,
					Color: map[string]string{
						"opened":    "#0066CC",
						"reviewed":  "#6753AC",
						"approved":  "#4AB740",
						"merged":    "#3E8635",
						"abandoned": "#A30000",
					},
					ColorMode: "gradient",
					Labels:    map[string]string{"opened": "Opened", "reviewed": "Reviewed", "approved": "Approved", "merged": "Merged", "abandoned": "Abandoned"},
					Priority:  map[string]int{"opened": 0, "reviewed": 1, "approved": 2, "merged": 3, "abandoned": 4},
					Column:    map[string]int{"opened": 0, "reviewed": 1, "approved": 2, "merged": 3, "abandoned": 3},
					Size:      "max",
				},
			},
		},
	}
}

// findApproval returns the time of the "first approval" event for a PR,
// applying Prow-aware detection (last /approve + /lgtm if both labels present;
// falling back to native APPROVED review).
func findApproval(pr qpr.PRRow, revList []qpr.ReviewRow) *time.Time {
	prowLabels := parseProwLabels(pr.ProwLabels)
	hasApproved := prowLabels["approved"]
	hasLGTM := prowLabels["lgtm"]

	if hasApproved || hasLGTM {
		var lastApprove, lastLGTM *time.Time
		for i := len(revList) - 1; i >= 0; i-- {
			rev := revList[i]
			if strings.Contains(rev.Body, "[APPROVALNOTIFIER]") {
				continue
			}
			if hasApproved && lastApprove == nil && reApprove.MatchString(rev.Body) {
				t := rev.CreatedDate
				lastApprove = &t
			}
			if hasLGTM && lastLGTM == nil && reLGTM.MatchString(rev.Body) {
				t := rev.CreatedDate
				lastLGTM = &t
			}
			if (!hasApproved || lastApprove != nil) && (!hasLGTM || lastLGTM != nil) {
				break
			}
		}
		if hasApproved && hasLGTM && lastApprove != nil && lastLGTM != nil {
			if lastApprove.After(*lastLGTM) {
				return lastApprove
			}
			return lastLGTM
		}
		if t := coalesceTime(lastApprove, lastLGTM); t != nil {
			return t
		}
		return nativeApproval(revList)
	}
	return nativeApproval(revList)
}

func parseProwLabels(labels string) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(labels, ",") {
		l = strings.TrimSpace(strings.ToLower(l))
		if l != "" {
			m[l] = true
		}
	}
	return m
}

func nativeApproval(revList []qpr.ReviewRow) *time.Time {
	for _, rev := range revList {
		if rev.Status == "APPROVED" {
			t := rev.CreatedDate
			return &t
		}
	}
	return nil
}

func coalesceTime(a, b *time.Time) *time.Time {
	if a != nil {
		return a
	}
	return b
}
