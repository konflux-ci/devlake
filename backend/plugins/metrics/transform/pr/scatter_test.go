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

	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildScatter_empty(t *testing.T) {
	resp := BuildScatter(nil)
	assert.Equal(t, "line", resp.Type)
	assert.Equal(t, "cycleTimeDistribution", resp.Subtype)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "expected map[string]interface{}, got %T", resp.Data)

	labels := data["labels"].([]string)
	assert.Equal(t, len(cycleTimeBuckets), len(labels))

	datasets := data["datasets"].([]map[string]interface{})
	require.Equal(t, 2, len(datasets))

	merged := datasets[0]["data"].([]int)
	abandoned := datasets[1]["data"].([]int)
	assert.Equal(t, make([]int, len(cycleTimeBuckets)), merged)
	assert.Equal(t, make([]int, len(cycleTimeBuckets)), abandoned)
}

func TestBuildScatter_bucketing(t *testing.T) {
	prs := []qpr.PRRow{
		// 30 min → bucket "< 1h", merged
		{ID: "1", Status: "MERGED", AuthorName: "alice",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-01T00:30:00Z")},
		// 2h → bucket "1–4h", merged
		{ID: "2", Status: "MERGED", AuthorName: "alice",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-01T02:00:00Z")},
		// 25h → bucket "1–2d", abandoned (CLOSED, not merged)
		{ID: "3", Status: "CLOSED", AuthorName: "bob",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			ClosedDate:  testsupport.ParseTime("2024-01-02T01:00:00Z")},
		// bot — excluded
		{ID: "4", Status: "MERGED", AuthorName: "dependabot[bot]",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-01T01:00:00Z")},
		// draft — excluded
		{ID: "5", Status: "MERGED", AuthorName: "carol", IsDraft: true,
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-01T01:00:00Z")},
	}

	resp := BuildScatter(prs)
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)

	datasets := data["datasets"].([]map[string]interface{})
	merged := datasets[0]["data"].([]int)
	abandoned := datasets[1]["data"].([]int)

	// bucket 0 = "< 1h": 1 merged
	assert.Equal(t, 1, merged[0], "< 1h merged")
	assert.Equal(t, 0, abandoned[0], "< 1h abandoned")

	// bucket 1 = "1–4h": 1 merged
	assert.Equal(t, 1, merged[1], "1-4h merged")
	assert.Equal(t, 0, abandoned[1], "1-4h abandoned")

	// bucket 4 = "1–2d" (25h): 1 abandoned
	assert.Equal(t, 0, merged[4], "1-2d merged")
	assert.Equal(t, 1, abandoned[4], "1-2d abandoned")

	// all other buckets zero
	total := 0
	for _, v := range merged {
		total += v
	}
	for _, v := range abandoned {
		total += v
	}
	assert.Equal(t, 3, total, "only 3 valid PRs counted")
}

func TestBuildScatter_labels(t *testing.T) {
	resp := BuildScatter(nil)
	data := resp.Data.(map[string]interface{})
	labels := data["labels"].([]string)
	assert.Equal(t, "< 1h", labels[0])
	assert.Equal(t, "> 4w", labels[len(labels)-1])
}
