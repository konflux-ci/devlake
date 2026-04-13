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
	"time"

	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

func TestIsApplySuggestionCommit(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		aiToolUser string
		want       bool
	}{
		{
			name:       "GitHub Apply suggestion button",
			message:    "Apply suggestions from code review\n\nCo-authored-by: coderabbitai[bot] <136622811+coderabbitai[bot]@users.noreply.github.com>",
			aiToolUser: "coderabbitai[bot]",
			want:       true,
		},
		{
			name:       "Short Apply suggestion message",
			message:    "Apply suggestion",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Co-authored-by CodeRabbit",
			message:    "Fix nil check\n\nCo-authored-by: coderabbitai[bot] <noreply@github.com>",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Co-authored-by Qodo",
			message:    "Add validation\n\nCo-authored-by: qodo-merge-pro[bot] <noreply@github.com>",
			aiToolUser: "",
			want:       true,
		},
		{
			name:       "Regular commit, no suggestion",
			message:    "Fix bug in handler\n\nAdded nil check to prevent panic.",
			aiToolUser: "coderabbitai[bot]",
			want:       false,
		},
		{
			name:       "Empty message",
			message:    "",
			aiToolUser: "coderabbitai[bot]",
			want:       false,
		},
		{
			name:       "Commit mentioning AI tool user",
			message:    "Applied fix suggested by coderabbitai[bot]",
			aiToolUser: "coderabbitai[bot]",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isApplySuggestionCommit(tt.message, tt.aiToolUser)
			if got != tt.want {
				t.Errorf("isApplySuggestionCommit(%q, %q) = %v, want %v", tt.message, tt.aiToolUser, got, tt.want)
			}
		})
	}
}

func TestFilePathsMatch(t *testing.T) {
	tests := []struct {
		name  string
		path1 string
		path2 string
		want  bool
	}{
		{"exact match", "pkg/server/server.go", "pkg/server/server.go", true},
		{"with a/ prefix", "a/pkg/server/server.go", "pkg/server/server.go", true},
		{"with b/ prefix", "b/pkg/server/server.go", "pkg/server/server.go", true},
		{"both prefixed", "a/pkg/server/server.go", "b/pkg/server/server.go", true},
		{"suffix match", "src/pkg/server/server.go", "pkg/server/server.go", true},
		{"different files", "pkg/server/server.go", "pkg/server/config.go", false},
		{"different dirs", "pkg/server/server.go", "cmd/server/server.go", false},
		{"empty paths", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filePathsMatch(tt.path1, tt.path2)
			if got != tt.want {
				t.Errorf("filePathsMatch(%q, %q) = %v, want %v", tt.path1, tt.path2, got, tt.want)
			}
		})
	}
}

func TestCalculateTemporalScore(t *testing.T) {
	tests := []struct {
		name  string
		delta time.Duration
		want  int
	}{
		{"10 minutes", 10 * time.Minute, 75},
		{"1 hour", 1 * time.Hour, 60},
		{"12 hours", 12 * time.Hour, 45},
		{"48 hours", 48 * time.Hour, 30},
		{"1 week", 7 * 24 * time.Hour, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTemporalScore(tt.delta)
			if got != tt.want {
				t.Errorf("calculateTemporalScore(%v) = %d, want %d", tt.delta, got, tt.want)
			}
		})
	}
}

func TestMatchFinding(t *testing.T) {
	baseTime := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		finding     suggestionFinding
		commits     []prCommit
		fileChanges []commitFileChange
		wantMatched bool
		wantMethod  string
		wantMinScore int
	}{
		{
			name: "Apply suggestion commit with matching file",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:       "f1",
					FilePath: "pkg/server/server.go",
					Type:     models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
				AiToolUser:        "coderabbitai[bot]",
			},
			commits: []prCommit{
				{CommitSha: "abc123", AuthoredDate: baseTime.Add(20 * time.Minute), Message: "Apply suggestion from code review\n\nCo-authored-by: coderabbitai[bot] <noreply@github.com>"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "abc123", FilePath: "pkg/server/server.go", Additions: 3, Deletions: 1},
			},
			wantMatched:  true,
			wantMethod:   "diff_commit_msg",
			wantMinScore: 90,
		},
		{
			name: "File modified shortly after suggestion",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:              "f2",
					FilePath:        "pkg/config/config.go",
					MatchedFilePath: "pkg/config/config.go",
					Type:            models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "def456", AuthoredDate: baseTime.Add(15 * time.Minute), Message: "Fix config validation"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "def456", FilePath: "pkg/config/config.go", Additions: 5, Deletions: 2},
			},
			wantMatched:  true,
			wantMethod:   "diff_file_temporal",
			wantMinScore: 70,
		},
		{
			name: "File modified much later — weak signal",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:       "f3",
					FilePath: "pkg/handler.go",
					Type:     models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "ghi789", AuthoredDate: baseTime.Add(48 * time.Hour), Message: "Refactor handler"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "ghi789", FilePath: "pkg/handler.go", Additions: 20, Deletions: 10},
			},
			wantMatched:  true,
			wantMethod:   "diff_file_temporal",
			wantMinScore: 25,
		},
		{
			name: "No matching file in commits",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:       "f4",
					FilePath: "pkg/server/server.go",
					Type:     models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "jkl012", AuthoredDate: baseTime.Add(10 * time.Minute), Message: "Fix tests"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "jkl012", FilePath: "pkg/server/server_test.go", Additions: 10, Deletions: 5},
			},
			wantMatched: false,
		},
		{
			name: "Commit before suggestion — should not match",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:       "f5",
					FilePath: "pkg/server/server.go",
					Type:     models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
			},
			commits: []prCommit{
				{CommitSha: "mno345", AuthoredDate: baseTime.Add(-1 * time.Hour), Message: "Initial commit"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "mno345", FilePath: "pkg/server/server.go", Additions: 100, Deletions: 0},
			},
			wantMatched: false,
		},
		{
			name: "No file path — only commit message can match",
			finding: suggestionFinding{
				AiReviewFinding: models.AiReviewFinding{
					Id:   "f6",
					Type: models.FindingTypeSuggestion,
				},
				ReviewCreatedDate: baseTime,
				AiToolUser:        "coderabbitai[bot]",
			},
			commits: []prCommit{
				{CommitSha: "pqr678", AuthoredDate: baseTime.Add(5 * time.Minute), Message: "Apply suggestion\n\nCo-authored-by: coderabbitai[bot] <noreply>"},
			},
			fileChanges: []commitFileChange{
				{CommitSha: "pqr678", FilePath: "pkg/handler.go", Additions: 2, Deletions: 1},
			},
			wantMatched:  true,
			wantMethod:   "diff_commit_msg",
			wantMinScore: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchFinding(tt.finding, tt.commits, tt.fileChanges)
			if result.Matched != tt.wantMatched {
				t.Errorf("matchFinding() matched = %v, want %v", result.Matched, tt.wantMatched)
			}
			if tt.wantMatched {
				if result.Method != tt.wantMethod {
					t.Errorf("matchFinding() method = %q, want %q", result.Method, tt.wantMethod)
				}
				if result.Score < tt.wantMinScore {
					t.Errorf("matchFinding() score = %d, want >= %d", result.Score, tt.wantMinScore)
				}
			}
		})
	}
}
