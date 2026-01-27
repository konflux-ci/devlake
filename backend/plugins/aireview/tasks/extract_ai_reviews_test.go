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
	"regexp"
	"testing"

	"github.com/apache/incubator-devlake/plugins/aireview/models"
	"github.com/stretchr/testify/assert"
)

func TestDetectAiTool_CodeRabbit(t *testing.T) {
	tests := []struct {
		name       string
		accountId  string
		body       string
		wantTool   string
		wantIsAi   bool
	}{
		{
			name:      "CodeRabbit by username",
			accountId: "coderabbitai",
			body:      "Some review comment",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "CodeRabbit by pattern in body",
			accountId: "someuser",
			body:      "## Summary by CodeRabbit\n\nThis PR adds...",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "CodeRabbit walkthrough pattern",
			accountId: "bot",
			body:      "## Walkthrough\nThe changes introduce...",
			wantTool:  models.AiToolCodeRabbit,
			wantIsAi:  true,
		},
		{
			name:      "Not an AI review",
			accountId: "developer",
			body:      "LGTM, looks good to me!",
			wantTool:  "",
			wantIsAi:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			// Compile patterns
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotTool, gotIsAi := detectAiTool(taskData, tt.accountId, tt.body)
			assert.Equal(t, tt.wantTool, gotTool)
			assert.Equal(t, tt.wantIsAi, gotIsAi)
		})
	}
}

func TestParseReviewMetrics(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantComplexity   string
		wantEffort       int
		wantIssuesMin    int
		wantSuggestions  int
	}{
		{
			name:            "Simple review with time estimate",
			body:            "This is a simple change that takes ~12 minutes to review.",
			wantComplexity:  "simple",
			wantEffort:      12,
			wantIssuesMin:   0,
			wantSuggestions: 0,
		},
		{
			name:            "Complex review with issues",
			body:            "This is a complex change. Found a bug in the auth logic. Also there's an error in validation.",
			wantComplexity:  "complex",
			wantEffort:      30,
			wantIssuesMin:   2,
			wantSuggestions: 0,
		},
		{
			name:            "Review with suggestions",
			body:            "I suggest refactoring this. You should consider using a map. I would recommend adding tests.",
			wantComplexity:  "",
			wantEffort:      0,
			wantIssuesMin:   0,
			wantSuggestions: 3,
		},
		{
			name:            "Review with line changes",
			body:            "Changes: +50 −36 lines modified",
			wantComplexity:  "",
			wantEffort:      0,
			wantIssuesMin:   0,
			wantSuggestions: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)

			if tt.wantComplexity != "" {
				assert.Equal(t, tt.wantComplexity, metrics.Complexity)
			}
			if tt.wantEffort > 0 {
				assert.Equal(t, tt.wantEffort, metrics.EffortMinutes)
			}
			assert.GreaterOrEqual(t, metrics.IssuesFound, tt.wantIssuesMin)
			assert.GreaterOrEqual(t, metrics.SuggestionsCount, tt.wantSuggestions)
		})
	}
}

func TestDetectRiskLevel(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantLevel string
		wantScore int
	}{
		{
			name:      "High risk - security",
			body:      "CRITICAL: This introduces a security vulnerability",
			wantLevel: models.RiskLevelHigh,
			wantScore: 80,
		},
		{
			name:      "Medium risk - warning",
			body:      "Warning: This change may have moderate impact",
			wantLevel: models.RiskLevelMedium,
			wantScore: 50,
		},
		{
			name:      "Low risk - minor",
			body:      "Minor suggestion: consider renaming this variable",
			wantLevel: models.RiskLevelLow,
			wantScore: 20,
		},
		{
			name:      "Default to low",
			body:      "Looks good, no issues found",
			wantLevel: models.RiskLevelLow,
			wantScore: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskData := &AiReviewTaskData{
				Options: &AiReviewOptions{
					ScopeConfig: models.GetDefaultScopeConfig(),
				},
			}
			err := CompilePatterns(taskData)
			assert.NoError(t, err)

			gotLevel, gotScore := detectRiskLevel(taskData, tt.body)
			assert.Equal(t, tt.wantLevel, gotLevel)
			assert.Equal(t, tt.wantScore, gotScore)
		})
	}
}

func TestDetectReviewState(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		status    string
		wantState string
	}{
		{
			name:      "Approved in body",
			body:      "LGTM, approved!",
			status:    "",
			wantState: models.ReviewStateApproved,
		},
		{
			name:      "Changes requested",
			body:      "Please make changes requested above",
			status:    "",
			wantState: models.ReviewStateChangesRequested,
		},
		{
			name:      "Approved via status",
			body:      "Some comment",
			status:    "APPROVED",
			wantState: models.ReviewStateApproved,
		},
		{
			name:      "Default to commented",
			body:      "Here are some thoughts",
			status:    "",
			wantState: models.ReviewStateCommented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotState := detectReviewState(tt.body, tt.status)
			assert.Equal(t, tt.wantState, gotState)
		})
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantContain string
	}{
		{
			name:        "Extract summary section",
			body:        "## Summary\nThis PR adds a new feature.\n\nDetails follow...",
			wantContain: "This PR adds a new feature",
		},
		{
			name:        "Extract walkthrough section",
			body:        "## Walkthrough\nThe changes introduce new handlers.\n\n## Details",
			wantContain: "changes introduce new handlers",
		},
		{
			name:        "Truncate long body without summary",
			body:        "This is a very long comment that goes on and on...",
			wantContain: "This is a very long comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.body)
			assert.Contains(t, got, tt.wantContain)
		})
	}
}

func TestGenerateReviewId(t *testing.T) {
	// Test deterministic ID generation
	id1 := generateReviewId("pr-123", "comment-456", "coderabbit")
	id2 := generateReviewId("pr-123", "comment-456", "coderabbit")
	id3 := generateReviewId("pr-123", "comment-789", "coderabbit")

	assert.Equal(t, id1, id2, "Same inputs should produce same ID")
	assert.NotEqual(t, id1, id3, "Different inputs should produce different IDs")
	assert.True(t, len(id1) > 0, "ID should not be empty")
	assert.Contains(t, id1, "aireview:", "ID should have correct prefix")
}

func TestDetectSourcePlatform(t *testing.T) {
	tests := []struct {
		name     string
		prId     string
		wantPlat string
	}{
		{
			name:     "GitHub PR",
			prId:     "github:GithubPullRequest:1:12345",
			wantPlat: "github",
		},
		{
			name:     "GitLab MR",
			prId:     "gitlab:GitlabMergeRequest:1:67890",
			wantPlat: "gitlab",
		},
		{
			name:     "Unknown platform",
			prId:     "bitbucket:PullRequest:1:11111",
			wantPlat: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectSourcePlatform(tt.prId)
			assert.Equal(t, tt.wantPlat, got)
		})
	}
}

func TestCompilePatterns(t *testing.T) {
	t.Run("Default config compiles successfully", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: models.GetDefaultScopeConfig(),
			},
		}
		err := CompilePatterns(taskData)
		assert.NoError(t, err)
		assert.NotNil(t, taskData.CodeRabbitUsernameRegex)
		assert.NotNil(t, taskData.CodeRabbitPatternRegex)
		assert.NotNil(t, taskData.RiskHighPatternRegex)
	})

	t.Run("Invalid regex returns error", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: &models.AiReviewScopeConfig{
					CodeRabbitEnabled: true,
					CodeRabbitPattern: "[invalid(regex",
				},
			},
		}
		err := CompilePatterns(taskData)
		assert.Error(t, err)
	})

	t.Run("Nil config uses defaults", func(t *testing.T) {
		taskData := &AiReviewTaskData{
			Options: &AiReviewOptions{
				ScopeConfig: nil,
			},
		}
		err := CompilePatterns(taskData)
		assert.NoError(t, err)
		assert.NotNil(t, taskData.Options.ScopeConfig)
	})
}

func TestCodeRabbitPatternMatching(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)(coderabbit|walkthrough|summary by coderabbit)`)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"CodeRabbit mention", "This is a CodeRabbit review", true},
		{"Walkthrough section", "## Walkthrough", true},
		{"Summary by CodeRabbit", "Summary by CodeRabbit:", true},
		{"Case insensitive", "CODERABBIT", true},
		{"No match", "Regular comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pattern.MatchString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
