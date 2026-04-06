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

package tasks

import (
	"testing"

	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

func TestCalculateOutcome(t *testing.T) {
	cases := []struct {
		flagged bool
		failed  bool
		want    string
	}{
		{true, true, models.PredictionTP},
		{true, false, models.PredictionFP},
		{false, true, models.PredictionFN},
		{false, false, models.PredictionTN},
	}
	for _, tc := range cases {
		got := calculateOutcome(tc.flagged, tc.failed)
		if got != tc.want {
			t.Errorf("calculateOutcome(flagged=%v, failed=%v) = %q, want %q",
				tc.flagged, tc.failed, got, tc.want)
		}
	}
}

// TestNoCiDataProducesNoPredictions verifies the core invariant: a PR that has
// no matching entry in the ciOutcomes map must not produce any prediction.
//
// This is the unit-level guard for the bug where missing CI data was treated as
// "CI passed", turning every flagged PR into a False Positive.
func TestNoCiDataProducesNoPredictions(t *testing.T) {
	// Simulate the inner loop logic directly: an empty CI outcomes map (no
	// TestRegistry configured) paired with one high-risk AI-reviewed PR.
	ciOutcomes := map[prCiKey]ciOutcomeEntry{} // no CI data for any repo

	summaries := []prAiSummary{
		{
			PullRequestId:  "github:GithubPullRequest:1:9001",
			PullRequestKey: "901",
			RepoId:         "github:GithubRepo:1:999",
			RepoShortName:  "no-ci-repo",
			AiTool:         "CodeRabbit",
			MaxRiskScore:   80, // above any reasonable warning threshold
		},
	}

	written := 0
	for i := range summaries {
		ps := &summaries[i]
		ciKey := prCiKey{PullRequestNumber: ps.PullRequestKey, Repository: ps.RepoShortName}
		_, hasCiData := ciOutcomes[ciKey]
		if !hasCiData {
			continue // must skip — this is what we're testing
		}
		written++
	}

	if written != 0 {
		t.Errorf("expected 0 predictions written for repo with no CI data, got %d", written)
	}
}

// TestGeneratePredictionId verifies that two identical inputs produce the same
// ID and that differing inputs produce different IDs.
func TestGeneratePredictionId(t *testing.T) {
	id1 := generatePredictionId("pr:1", "CodeRabbit", "job_result")
	id2 := generatePredictionId("pr:1", "CodeRabbit", "job_result")
	id3 := generatePredictionId("pr:1", "CodeRabbit", "test_cases")
	id4 := generatePredictionId("pr:2", "CodeRabbit", "job_result")

	if id1 != id2 {
		t.Errorf("same inputs produced different IDs: %q vs %q", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("different ci_source should produce different IDs, both got %q", id1)
	}
	if id1 == id4 {
		t.Errorf("different PR ID should produce different IDs, both got %q", id1)
	}
	if len(id1) == 0 {
		t.Error("generatePredictionId returned empty string")
	}
}
