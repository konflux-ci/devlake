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

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFlow_MergedPR(t *testing.T) {
	merged := testsupport.ParseTime("2024-01-05T12:00:00Z")
	prs := []qpr.PRRow{
		{
			ID: "pr1", Key: "repo#1", URL: "https://x.com/1",
			Status:      "MERGED",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  merged,
			AuthorName:  "alice",
		},
	}
	reviews := []qpr.ReviewRow{
		{PullRequestID: "pr1", CreatedDate: *testsupport.ParseTime("2024-01-02T00:00:00Z"), Body: "", Status: "", AuthorName: "bob"},
		{PullRequestID: "pr1", CreatedDate: *testsupport.ParseTime("2024-01-03T00:00:00Z"), Body: "/lgtm", Status: "APPROVED", AuthorName: "carol"},
	}

	resp := BuildFlow(prs, reviews)
	sd, ok := resp.Data.(model.SankeyData)
	require.True(t, ok, "expected SankeyData, got %T", resp.Data)

	// Should have OPENED->REVIEWED, REVIEWED->APPROVED, APPROVED->MERGED transitions
	flowByKey := map[string]model.SankeyFlow{}
	for _, f := range sd.Datasets[0].Data {
		flowByKey[f.From+"->"+f.To] = f
	}
	assert.Contains(t, flowByKey, "opened->reviewed")
	assert.Contains(t, flowByKey, "reviewed->approved")
	assert.Contains(t, flowByKey, "approved->merged")
}

func TestBuildFlow_BotReviewsExcluded(t *testing.T) {
	merged := testsupport.ParseTime("2024-01-05T00:00:00Z")
	prs := []qpr.PRRow{
		{
			ID: "pr2", Key: "repo#2", URL: "https://x.com/2",
			Status:      "MERGED",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  merged,
			AuthorName:  "alice",
		},
	}
	// Only bot review — should be excluded → no REVIEWED state
	reviews := []qpr.ReviewRow{
		{PullRequestID: "pr2", CreatedDate: *testsupport.ParseTime("2024-01-02T00:00:00Z"), Body: "", Status: "APPROVED", AuthorName: "dependabot[bot]"},
	}

	resp := BuildFlow(prs, reviews)
	sd, ok := resp.Data.(model.SankeyData)
	require.True(t, ok, "expected SankeyData, got %T", resp.Data)

	flowByKey := map[string]bool{}
	for _, f := range sd.Datasets[0].Data {
		flowByKey[f.From+"->"+f.To] = true
	}

	assert.NotContains(t, flowByKey, "opened->reviewed")
	assert.Contains(t, flowByKey, "opened->merged")
}
