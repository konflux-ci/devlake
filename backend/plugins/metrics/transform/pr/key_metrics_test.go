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
	"strings"
	"testing"

	"github.com/apache/incubator-devlake/plugins/metrics/model"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildKeyMetrics_NoCompare(t *testing.T) {
	stats := qpr.KeyMetricsStatsRow{
		TotalClosedPRs:  42,
		MedianCycleTime: 8.5,
		OpenedPRs:       30,
		OutlierPRs:      3,
	}
	outliers := []qpr.OutlierPRRow{
		{PRNumber: "repo#10", PRURL: "https://example.com/10", CycleTimeHours: 72.5},
	}

	resp := BuildKeyMetrics(stats, nil, outliers)
	sd, ok := resp.Data.(model.StatsData)
	require.True(t, ok, "expected StatsData, got %T", resp.Data)
	assert.Equal(t, 4, len(sd.Metrics))
	// No comparison → explain strings should be empty
	for _, m := range sd.Metrics {
		assert.Empty(t, m.Explain)
	}
	// Outlier drill-down
	assert.NotNil(t, sd.Extra)
	entries, ok := sd.Extra.Data["outlier"].([]OutlierEntry)
	require.True(t, ok, "expected []OutlierEntry, got %T", sd.Extra.Data["outlier"])
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "repo#10", entries[0].ID)
}

func TestBuildKeyMetrics_WithCompare(t *testing.T) {
	stats := qpr.KeyMetricsStatsRow{TotalClosedPRs: 80, MedianCycleTime: 5, OpenedPRs: 60, OutlierPRs: 2}
	prev := qpr.KeyMetricsStatsRow{TotalClosedPRs: 100, MedianCycleTime: 10, OpenedPRs: 50, OutlierPRs: 4}

	resp := BuildKeyMetrics(stats, &prev, nil)
	sd, ok := resp.Data.(model.StatsData)
	require.True(t, ok, "expected StatsData, got %T", resp.Data)

	// Throughput went down (bad for throughput → comparedown)
	assert.Contains(t, sd.Metrics[0].Explain, "comparedown")

	// Cycle time went down (good → compareup)
	assert.Contains(t, sd.Metrics[1].Explain, "compareup")
}

func TestCompareHTML(t *testing.T) {
	tests := []struct {
		name          string
		current, prev float64
		lowerIsBetter bool
		wantClass     string
	}{
		{"throughput up", 120, 100, false, "compareup"},
		{"throughput down", 80, 100, false, "comparedown"},
		{"cycle time down", 5, 10, true, "compareup"},
		{"cycle time up", 15, 10, true, "comparedown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareHTML(tt.current, tt.prev, tt.lowerIsBetter, "prev")
			if !strings.Contains(got, tt.wantClass) {
				assert.Contains(t, got, tt.wantClass)
			}
		})
	}
}
