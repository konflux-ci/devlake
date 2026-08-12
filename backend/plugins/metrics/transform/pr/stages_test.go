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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildStages_empty(t *testing.T) {
	resp := BuildStages(nil)
	assert.Equal(t, "doughnut", resp.Type)
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "expected map[string]interface{}, got %T", resp.Data)
	datasets, ok := data["datasets"].([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} for datasets")
	assert.Equal(t, []float64{0, 0, 0}, datasets[0]["data"])
}

func TestBuildStages_averages(t *testing.T) {
	prs := []qpr.StagePRRow{
		{Key: "repo#1", URL: "https://github.com/org/repo/pull/1", PickupHours: 2.0, ReviewHours: 4.0, IntegrationHours: 1.0},
		{Key: "repo#2", URL: "https://github.com/org/repo/pull/2", PickupHours: 4.0, ReviewHours: 8.0, IntegrationHours: 3.0},
	}

	resp := BuildStages(prs)
	assert.Equal(t, "doughnut", resp.Type)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "expected map[string]interface{}, got %T", resp.Data)

	datasets, ok := data["datasets"].([]map[string]interface{})
	require.True(t, ok, "expected []map[string]interface{} for datasets")

	vals, ok := datasets[0]["data"].([]float64)
	require.True(t, ok, "expected []float64 for dataset data")
	assert.Equal(t, []float64{3.0, 6.0, 2.0}, vals)

	// Check drill-down lists exist under extra.data
	extra, ok := data["extra"].(map[string]interface{})
	require.True(t, ok, "expected extra map. got %T", data["extra"])
	drilldown, ok := extra["data"].(map[string]interface{})
	require.True(t, ok, "expected extra.data map. got %T", extra["data"])
	assert.Contains(t, drilldown, "pickup_prs")
	assert.Contains(t, drilldown, "review_prs")
	assert.Contains(t, drilldown, "integration_prs")
}
