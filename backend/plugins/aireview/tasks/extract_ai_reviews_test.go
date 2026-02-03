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
		wantNotContain string
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
			body:        "This is a very long comment that goes on and on with more content.",
			wantContain: "This is a very long comment",
		},
		{
			name:        "Clean HTML tags",
			body:        "<table><tr><td>Some content here that is long enough to be extracted as a summary paragraph</td></tr></table>",
			wantContain: "Some content here",
			wantNotContain: "<table>",
		},
		{
			name:        "Preserve markdown quote blocks",
			body:        "> This is a quote\n>Another line\n\nActual content here is important",
			wantContain: "> This is a quote",
		},
		{
			name:        "Handle escaped newlines",
			body:        "First line\\nSecond line\\nThird line with content",
			wantContain: "Second line",
		},
		{
			name:        "Extract Qodo focus areas",
			body:        "## PR Reviewer Guide\n\n**Estimated effort to review**: 3\n\n**Recommended focus areas for review**:\n\n**Over-broad replace** - The sed replacements may break production.",
			wantContain: "Over-broad replace",
		},
		{
			name:        "Convert details/summary to markdown",
			body:        "Main content<details><summary>Hidden</summary>Secret stuff</details>More content",
			wantContain: "**Hidden**",
			wantNotContain: "<details>",
		},
		{
			name:        "Preserve markdown links",
			body:        "Check [this link](https://example.com) for details.",
			wantContain: "[this link]",
		},
		{
			name:        "Handle real Qodo format",
			body:        "## PR Reviewer Guide\n\n<table>\n<tr><td>\n\n**Ticket compliance**\n\n</td></tr>\n<tr><td><strong>Estimated effort to review</strong>: 2</td></tr>\n</table>",
			wantContain: "Effort: 2/5",
			wantNotContain: "<table>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSummary(tt.body)
			assert.Contains(t, got, tt.wantContain)
			if tt.wantNotContain != "" {
				assert.NotContains(t, got, tt.wantNotContain)
			}
		})
	}
}

func TestHtmlToMarkdown(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantContain    string
		wantNotContain string
	}{
		{
			name:           "Convert strong to bold markdown",
			input:          "<strong>Bold</strong> text",
			wantContain:    "**Bold**",
			wantNotContain: "<strong>",
		},
		{
			name:        "Handle escaped newlines",
			input:       "Line1\\nLine2\\nLine3",
			wantContain: "Line1\nLine2\nLine3",
		},
		{
			name:           "Convert details/summary to markdown",
			input:          "Before<details><summary>Title</summary>Hidden content</details>After",
			wantContain:    "**Title**",
			wantNotContain: "<details>",
		},
		{
			name:        "Preserve markdown quote blocks",
			input:       "> Quoted text\n> More quoted",
			wantContain: "> Quoted text",
		},
		{
			name:        "Preserve bold markers",
			input:       "This is **bold** and __also bold__",
			wantContain: "**bold**",
		},
		{
			name:        "Preserve markdown headers",
			input:       "## This is a header\n\nParagraph text",
			wantContain: "## This is a header",
		},
		{
			name:        "Preserve markdown links",
			input:       "See [documentation](https://docs.example.com) here",
			wantContain: "[documentation](https://docs.example.com)",
		},
		{
			name:        "Convert HTML links to markdown",
			input:       "See <a href='https://docs.example.com'>documentation</a> here",
			wantContain: "[documentation](https://docs.example.com)",
		},
		{
			name:           "Remove HTML tables but keep content",
			input:          "<table><tr><td>Cell content</td></tr></table>",
			wantContain:    "Cell content",
			wantNotContain: "<table>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := htmlToMarkdown(tt.input)
			assert.Contains(t, got, tt.wantContain)
			if tt.wantNotContain != "" {
				assert.NotContains(t, got, tt.wantNotContain)
			}
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

func TestDetectAiTool_Qodo(t *testing.T) {
	tests := []struct {
		name      string
		accountId string
		body      string
		wantTool  string
		wantIsAi  bool
	}{
		{
			name:      "Qodo by username",
			accountId: "qodo-merge",
			body:      "Some review comment",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by username with prefix",
			accountId: "rhdh-qodo-merge",
			body:      "Some review comment",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by PR Reviewer Guide pattern",
			accountId: "somebot",
			body:      "## PR Reviewer Guide 🔍\n\n⏱️ Estimated effort to review: 3 🔵🔵🔵⚪⚪",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo by effort estimate pattern",
			accountId: "bot",
			body:      "⏱️ Estimated effort to review: 2 🔵🔵⚪⚪⚪",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Qodo mentioned in body",
			accountId: "someuser",
			body:      "This is a Qodo review with security findings",
			wantTool:  models.AiToolQodo,
			wantIsAi:  true,
		},
		{
			name:      "Not Qodo - regular comment",
			accountId: "developer",
			body:      "LGTM, looks good!",
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

func TestBuildCommentUrl(t *testing.T) {
	tests := []struct {
		name      string
		prUrl     string
		commentId string
		want      string
	}{
		{
			name:      "GitHub comment",
			prUrl:     "https://github.com/owner/repo/pull/123",
			commentId: "github:GithubPrComment:1:456789",
			want:      "https://github.com/owner/repo/pull/123#issuecomment-456789",
		},
		{
			name:      "GitLab comment",
			prUrl:     "https://gitlab.com/owner/repo/-/merge_requests/123",
			commentId: "gitlab:GitlabMrComment:1:456789",
			want:      "https://gitlab.com/owner/repo/-/merge_requests/123#note_456789",
		},
		{
			name:      "Empty PR URL",
			prUrl:     "",
			commentId: "github:GithubPrComment:1:456789",
			want:      "",
		},
		{
			name:      "Malformed comment ID",
			prUrl:     "https://github.com/owner/repo/pull/123",
			commentId: "invalid",
			want:      "https://github.com/owner/repo/pull/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommentUrl(tt.prUrl, tt.commentId)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQodoPatternMatching(t *testing.T) {
	pattern := regexp.MustCompile(`(?i)(qodo|pr reviewer guide|estimated effort to review)`)

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"Qodo mention", "This is a Qodo review", true},
		{"PR Reviewer Guide header", "## PR Reviewer Guide 🔍", true},
		{"Effort estimate", "⏱️ Estimated effort to review: 3", true},
		{"Case insensitive", "QODO", true},
		{"No match", "Regular comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pattern.MatchString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseReviewMetrics_EffortRating(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		wantRating     int
		wantComplexity string
	}{
		{
			name:           "CodeRabbit effort rating with complexity",
			body:           "🎯 3 (Moderate) | ⏱️ ~20 minutes",
			wantRating:     3,
			wantComplexity: "moderate",
		},
		{
			name:           "CodeRabbit effort rating only",
			body:           "Review effort: 🎯 4",
			wantRating:     4,
			wantComplexity: "complex", // inferred from rating >= 4
		},
		{
			name:           "CodeRabbit simple rating",
			body:           "🎯 1 (Simple)",
			wantRating:     1,
			wantComplexity: "simple",
		},
		{
			name:           "Qodo effort estimate format",
			body:           "⏱️ Estimated effort to review: 3 🔵🔵🔵⚪⚪",
			wantRating:     3,
			wantComplexity: "moderate", // inferred from rating 3
		},
		{
			name:           "Qodo table format",
			body:           "<tr><td><strong>Estimated effort to review</strong>: 2</td></tr>",
			wantRating:     2,
			wantComplexity: "simple", // inferred from rating <= 2
		},
		{
			name:           "No effort rating",
			body:           "This is a regular review comment",
			wantRating:     0,
			wantComplexity: "",
		},
		{
			name:           "Effort rating 5 (complex)",
			body:           "🎯 5 (Very Complex)",
			wantRating:     5,
			wantComplexity: "complex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)
			assert.Equal(t, tt.wantRating, metrics.EffortRating)
			if tt.wantComplexity != "" {
				assert.Equal(t, tt.wantComplexity, metrics.Complexity)
			}
		})
	}
}

func TestParseReviewMetrics_PreMergeChecks(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		wantPassed       int
		wantFailed       int
		wantInconclusive int
	}{
		{
			name:             "CodeRabbit pre-merge checks all present",
			body:             "Pre-merge checks: 2 passed, 1 failed, 1 inconclusive",
			wantPassed:       2,
			wantFailed:       1,
			wantInconclusive: 1,
		},
		{
			name:             "CodeRabbit with emojis",
			body:             "✅ 3 checks passed | ❌ 1 check failed",
			wantPassed:       3,
			wantFailed:       1,
			wantInconclusive: 0,
		},
		{
			name:             "Only passed checks",
			body:             "All checks passed: 5 passed",
			wantPassed:       5,
			wantFailed:       0,
			wantInconclusive: 0,
		},
		{
			name:             "Inconclusive only",
			body:             "Status: 2 inconclusive",
			wantPassed:       0,
			wantFailed:       0,
			wantInconclusive: 2,
		},
		{
			name:             "No check information",
			body:             "This is a regular review",
			wantPassed:       0,
			wantFailed:       0,
			wantInconclusive: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parseReviewMetrics(tt.body)
			assert.Equal(t, tt.wantPassed, metrics.PreMergeChecksPassed)
			assert.Equal(t, tt.wantFailed, metrics.PreMergeChecksFailed)
			assert.Equal(t, tt.wantInconclusive, metrics.PreMergeChecksInconclusive)
		})
	}
}

func TestParseReviewMetrics_CodeRabbitRealWorld(t *testing.T) {
	// Test with realistic CodeRabbit output format
	realCodeRabbitBody := `## Summary by CodeRabbit

🎯 3 (Moderate) | ⏱️ ~20 minutes

**Pre-merge checks**: 2 passed, 1 inconclusive

### Walkthrough
The changes introduce new handlers for authentication.

### Changes
- Added JWT validation
- Updated middleware configuration
`

	metrics := parseReviewMetrics(realCodeRabbitBody)

	assert.Equal(t, 3, metrics.EffortRating)
	assert.Equal(t, 20, metrics.EffortMinutes)
	assert.Equal(t, "moderate", metrics.Complexity)
	assert.Equal(t, 2, metrics.PreMergeChecksPassed)
	assert.Equal(t, 1, metrics.PreMergeChecksInconclusive)
}
