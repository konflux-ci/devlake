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
	"testing"
	"time"

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prPtr(t time.Time) *time.Time { return &t }

func makePR(key, status string, merged bool, draft bool, author string) qpr.PRRow {
	now := time.Now()
	r := qpr.PRRow{
		Key:         key,
		URL:         "https://github.com/org/repo/pull/" + key,
		Status:      status,
		AuthorName:  author,
		CreatedDate: prPtr(now.Add(-48 * time.Hour)),
		ClosedDate:  prPtr(now),
		IsDraft:     draft,
	}
	if merged {
		r.MergedDate = prPtr(now)
		r.Status = "MERGED"
	}
	return r
}

func TestBuildProductivity(t *testing.T) {
	prs := []qpr.PRRow{
		makePR("1", "MERGED", true, false, "alice"),
		makePR("2", "MERGED", true, false, "bob"),
		makePR("3", "CLOSED", false, false, "charlie"),
		makePR("4", "OPEN", false, false, "dave"),        // excluded: open
		makePR("5", "MERGED", true, true, "eve"),         // excluded: draft
		makePR("6", "MERGED", true, false, "dependabot"), // excluded: bot
	}

	resp := BuildProductivity(prs)

	assert.Equal(t, "doughnut", resp.Type)
	assert.IsType(t, ProductivitySummary{}, resp.Summary)
	summary := resp.Summary.(ProductivitySummary)
	assert.Equal(t, 2, summary.Merged)
	assert.Equal(t, 1, summary.ClosedWithoutMerge)
	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 66.7, summary.MergedPercentage, 0.1)

	data := resp.Data.(model.DoughnutData)
	counts := data.Datasets[0].Data.([]int)
	assert.Equal(t, []int{2, 1}, counts)

	// closed-without-merge drill-down should have 1 entry
	cwm := data.Extra.Data["closed_without_merge"].([]ProductivityEntry)
	assert.IsType(t, []ProductivityEntry{}, cwm)
	assert.Equal(t, 1, len(cwm))
	assert.Equal(t, "3", cwm[0].ID)
}

func TestBuildProductivity_Empty(t *testing.T) {
	resp := BuildProductivity(nil)
	summary := resp.Summary.(ProductivitySummary)
	assert.Equal(t, 0, summary.Total)

	data, ok := resp.Data.(model.DoughnutData)
	require.True(t, ok, "expected DoughnutData, got %T", resp.Data)
	assert.Empty(t, data.Extra.Data["closed_without_merge"])
}
