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
)

func TestComputeBaseline_Empty(t *testing.T) {
	mu, sigma := computeBaseline(nil)
	assert.Equal(t, 0.0, mu)
	assert.Greater(t, sigma, 1e-11)
}

func TestComputeBaseline_Consistent(t *testing.T) {
	// All PRs have the same cycle time → sigma should be near zero (clamped to 1e-10)
	d24 := testsupport.ParseTime("2024-01-02T00:00:00Z")
	prs := []qpr.PRRow{
		{ID: "1", Status: "MERGED", AuthorName: "alice", CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"), MergedDate: d24},
		{ID: "2", Status: "MERGED", AuthorName: "bob", CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"), MergedDate: d24},
	}
	mu, sigma := computeBaseline(prs)
	assert.Greater(t, mu, 0.0)
	assert.Greater(t, sigma, 1e-11)
}

func TestBuildZScore_Categories(t *testing.T) {
	// Three PRs: 1h (fast), 24h (average baseline), 240h (slow)
	lookback := []qpr.PRRow{
		{ID: "l1", Status: "MERGED", AuthorName: "alice",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-02T00:00:00Z")}, // 24h
	}
	current := []qpr.PRRow{
		{ID: "c1", Key: "r#1", Status: "MERGED", AuthorName: "alice",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-01T01:00:00Z")}, // 1h — fast
		{ID: "c2", Key: "r#2", Status: "MERGED", AuthorName: "bob",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-11T00:00:00Z")}, // 240h — slow
	}

	resp := BuildZScore(lookback, current)

	assert.Equal(t, "bar", resp.Type)
	bd := resp.Data.(model.BarData)
	assert.Equal(t, 3, len(bd.Labels))
	counts := bd.Datasets[0].Data.([]int)
	// fast=1, average=0, slow=1 (with sigma=1e-10 the baseline is narrow)
	total := counts[0] + counts[1] + counts[2]
	assert.Equal(t, 2, total)
}
