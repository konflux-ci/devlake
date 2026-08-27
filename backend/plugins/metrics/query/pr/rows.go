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

// Package pr provides SQL query functions and row types for pull-request metrics.
package pr

import "time"

// Params holds the dashboard request parameters used to scope all PR queries.
type Params struct {
	// BlueprintID is passed as a comma-separated string, e.g. "1,3,7".
	// FIND_IN_SET(bp.id, BlueprintID) is used in SQL.
	BlueprintID string
	// Repos is the list of repository short-names to filter on (r.name IN ...).
	Repos []string
	// From / To are Unix timestamps (seconds).  Both must be non-zero.
	From, To int64
	// Whitelist optionally restricts results to specific author names.
	Whitelist []string
}

// PRRow is a single pull-request record returned by BasePRs and ZScorePRs.
// Fields map to columns in the DevLake pull_requests domain table.
type PRRow struct {
	ID          string
	Key         string // pull_request_key (e.g. "repo#42")
	URL         string
	Status      string // "OPEN", "MERGED", "CLOSED"
	CreatedDate *time.Time
	MergedDate  *time.Time
	ClosedDate  *time.Time
	AuthorName  string
	Title       string
	Description string
	IsDraft     bool
	Additions   int
	Deletions   int
	ProwLabels  string // comma-separated label_names (only 'approved','lgtm')
}

// ReviewRow is a single pull-request comment / review record.
type ReviewRow struct {
	PullRequestID string
	CreatedDate   time.Time
	Body          string
	Status        string // "APPROVED" for native GitHub reviews
	AuthorName    string // accounts.user_name
}

// KeyMetricsStatsRow is the single aggregate row returned by KeyMetricsStats.
type KeyMetricsStatsRow struct {
	TotalClosedPRs        float64
	MedianCycleTime       float64
	OpenedPRs             float64
	OutlierPRs            float64
	MedianInteractionTime float64
}

// OutlierPRRow is one entry from the outlier PRs list query.
type OutlierPRRow struct {
	PRNumber       string
	PRURL          string
	CycleTimeHours float64
}

// StagePRRow holds per-PR cycle-time stage hours returned by StagesPRs.
// Only MERGED PRs with a complete review→approval chain are included.
type StagePRRow struct {
	Key              string // pull_request_key
	URL              string
	PickupHours      float64 // created → first review
	ReviewHours      float64 // first review → last approval
	IntegrationHours float64 // last approval → merge
}
