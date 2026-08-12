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

	"github.com/stretchr/testify/assert"

	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/testsupport"
)

func TestValidPR(t *testing.T) {
	merged := testsupport.ParseTime("2024-01-02T00:00:00Z")
	created := testsupport.ParseTime("2024-01-01T00:00:00Z")

	tests := []struct {
		name string
		row  qpr.PRRow
		want bool
	}{
		{
			"open PR excluded",
			qpr.PRRow{Status: "OPEN", CreatedDate: created, MergedDate: merged},
			false,
		},
		{
			"draft excluded",
			qpr.PRRow{Status: "MERGED", IsDraft: true, CreatedDate: created, MergedDate: merged},
			false,
		},
		{
			"nil created_date excluded",
			qpr.PRRow{Status: "MERGED", CreatedDate: nil, MergedDate: merged},
			false,
		},
		{
			"no end date excluded",
			qpr.PRRow{Status: "MERGED", CreatedDate: created, MergedDate: nil, ClosedDate: nil},
			false,
		},
		{
			"bot author excluded",
			qpr.PRRow{Status: "MERGED", CreatedDate: created, MergedDate: merged, AuthorName: "dependabot[bot]"},
			false,
		},
		{
			"valid merged PR",
			qpr.PRRow{Status: "MERGED", CreatedDate: created, MergedDate: merged, AuthorName: "alice"},
			true,
		},
		{
			"valid closed (not merged) PR",
			qpr.PRRow{Status: "CLOSED", CreatedDate: created, ClosedDate: merged, AuthorName: "alice"},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validPR(tt.row))
		})
	}
}

func TestCycleTimeHours(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	plus2h := base.Add(2 * time.Hour)
	minus1h := base.Add(-1 * time.Hour)

	tests := []struct {
		name string
		row  qpr.PRRow
		want float64
	}{
		{
			"nil created returns -1",
			qpr.PRRow{MergedDate: &plus2h},
			-1,
		},
		{
			"nil end date returns -1",
			qpr.PRRow{CreatedDate: &base},
			-1,
		},
		{
			"negative duration returns -1",
			qpr.PRRow{CreatedDate: &plus2h, MergedDate: &minus1h},
			-1,
		},
		{
			"merged_date used when set",
			qpr.PRRow{CreatedDate: &base, MergedDate: &plus2h},
			2,
		},
		{
			"closed_date used as fallback",
			qpr.PRRow{CreatedDate: &base, ClosedDate: &plus2h},
			2,
		},
		{
			"merged_date preferred over closed_date",
			qpr.PRRow{CreatedDate: &base, MergedDate: &plus2h, ClosedDate: &minus1h},
			2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cycleTimeHours(tt.row))
		})
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		name string
		in   []float64
		want float64
	}{
		{"nil", nil, 0},
		{"empty", []float64{}, 0},
		{"single", []float64{5}, 5},
		{"even two", []float64{1, 3}, 2},
		{"odd unsorted", []float64{3, 1, 2}, 2},
		{"even four", []float64{1, 2, 3, 4}, 2.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := median(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}
