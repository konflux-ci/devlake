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

// Package pr contains pure transform functions that convert raw SQL rows into
// MetricResponse JSON payloads consumed by the dashboard.
// All functions are deterministic and free of I/O — testable without a DB.
package pr

import (
	"math"
	"sort"
	"time"

	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/transform/botfilter"
)

// validPR returns true when the PR row should be included in transform output:
// - not OPEN, not draft
// - has created_date
// - has merged_date or closed_date
// - author is not a bot
func validPR(r qpr.PRRow) bool {
	if r.Status == "OPEN" {
		return false
	}
	if r.IsDraft {
		return false
	}
	if r.CreatedDate == nil {
		return false
	}
	if r.MergedDate == nil && r.ClosedDate == nil {
		return false
	}
	return !botfilter.IsBot(r.AuthorName, r.Title, r.Description)
}

// endDate returns merged_date if set, otherwise closed_date.
func endDate(r qpr.PRRow) *time.Time {
	if r.MergedDate != nil {
		return r.MergedDate
	}
	return r.ClosedDate
}

// cycleTimeHours returns (endDate - createdDate) in hours, or -1 if invalid.
func cycleTimeHours(r qpr.PRRow) float64 {
	ed := endDate(r)
	if ed == nil || r.CreatedDate == nil {
		return -1
	}
	h := ed.Sub(*r.CreatedDate).Hours()
	if h < 0 {
		return -1
	}
	return h
}

// median returns the median of a sorted (or unsorted) float64 slice.
// Returns 0 for empty slices.
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 0 {
		return (cp[n/2-1] + cp[n/2]) / 2
	}
	return cp[n/2]
}

// round1 rounds to 1 decimal place.
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// round2 rounds to 2 decimal places.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// nowMillis returns the current UTC time in milliseconds.
func nowMillis() int64 {
	return time.Now().UTC().UnixMilli()
}
