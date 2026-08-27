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

func TestBuildCycleTime(t *testing.T) {
	prs := []qpr.PRRow{
		// 24h cycle, closed 2024-01-02
		{ID: "1", Status: "MERGED", AuthorName: "alice",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-02T00:00:00Z")},
		// bot — should be excluded
		{ID: "2", Status: "MERGED", AuthorName: "dependabot[bot]",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-02T00:00:00Z")},
		// draft — excluded
		{ID: "3", Status: "MERGED", AuthorName: "bob", IsDraft: true,
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			MergedDate:  testsupport.ParseTime("2024-01-02T00:00:00Z")},
		// 48h cycle, closed 2024-01-03
		{ID: "4", Status: "CLOSED", AuthorName: "carol",
			CreatedDate: testsupport.ParseTime("2024-01-01T00:00:00Z"),
			ClosedDate:  testsupport.ParseTime("2024-01-03T00:00:00Z")},
	}

	resp := BuildCycleTime(prs)
	assert.IsType(t, model.LineData{}, resp.Data)
	ld := resp.Data.(model.LineData)

	// Two distinct close days
	assert.Equal(t, 2, len(ld.Labels))
	assert.Equal(t, "2024-01-02", ld.Labels[0])
	assert.Equal(t, "2024-01-03", ld.Labels[1])
	// Daily median for day 0 is 24h
	daily := ld.Datasets[0].Data.([]float64)
	assert.Equal(t, 24.0, daily[0])
	// Daily median for day 1 is 48h
	assert.Equal(t, 48.0, daily[1])
}
