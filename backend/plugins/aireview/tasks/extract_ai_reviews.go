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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

var ExtractAiReviewsMeta = plugin.SubTaskMeta{
	Name:             "extractAiReviews",
	EntryPoint:       ExtractAiReviews,
	EnabledByDefault: true,
	Description:      "Extract AI-generated reviews from pull request comments",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

// ExtractAiReviews identifies and extracts AI-generated reviews from PR comments
func ExtractAiReviews(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	// Build query based on mode (projectName or repoId)
	var clauses []dal.Clause

	if data.Options.ProjectName != "" {
		logger.Info("Starting AI review extraction for project: %s", data.Options.ProjectName)
		// Project mode: join with project_mappings to get all repos in project
		clauses = []dal.Clause{
			dal.Select("prc.*, pr.base_repo_id, pr.status as pr_status, pr.merged_date, pr.url as pr_url"),
			dal.From("pull_request_comments prc"),
			dal.Join("LEFT JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Join("LEFT JOIN project_mapping pm ON pr.base_repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ?", data.Options.ProjectName, "repos"),
		}
	} else {
		logger.Info("Starting AI review extraction for repo: %s", data.Options.RepoId)
		// Single repo mode
		clauses = []dal.Clause{
			dal.Select("prc.*, pr.base_repo_id, pr.status as pr_status, pr.merged_date, pr.url as pr_url"),
			dal.From("pull_request_comments prc"),
			dal.Join("LEFT JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Where("pr.base_repo_id = ?", data.Options.RepoId),
		}
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query pull request comments")
	}
	defer cursor.Close()

	// Track processed reviews to avoid duplicates
	processedReviews := make(map[string]bool)
	batchSize := 100
	batch := make([]*models.AiReview, 0, batchSize)

	for cursor.Next() {
		var comment struct {
			code.PullRequestComment
			BaseRepoId string     `gorm:"column:base_repo_id"`
			PrStatus   string     `gorm:"column:pr_status"`
			MergedDate *time.Time `gorm:"column:merged_date"`
			PrUrl      string     `gorm:"column:pr_url"`
		}

		if err := db.Fetch(cursor, &comment); err != nil {
			return errors.Default.Wrap(err, "failed to fetch comment")
		}

		// Check if this is an AI-generated review
		aiTool, isAiReview := detectAiTool(data, comment.AccountId, comment.Body)
		if !isAiReview {
			continue
		}

		// Generate unique ID for this review
		reviewId := generateReviewId(comment.PullRequestId, comment.Id, aiTool)
		if processedReviews[reviewId] {
			continue
		}
		processedReviews[reviewId] = true

		// Parse the review content for metrics
		reviewMetrics := parseReviewMetrics(comment.Body)

		// Detect risk level
		riskLevel, riskScore := detectRiskLevel(data, comment.Body)

		// Determine repo ID (from query result in project mode, from options in repo mode)
		repoId := comment.BaseRepoId
		if repoId == "" {
			repoId = data.Options.RepoId
		}

		// Create AI review record
		aiReview := &models.AiReview{
			Id:               reviewId,
			PullRequestId:    comment.PullRequestId,
			RepoId:           repoId,
			AiTool:           aiTool,
			AiToolUser:       comment.AccountId,
			ReviewId:         comment.Id,
			Body:             comment.Body,
			Summary:          extractSummary(comment.Body),
			CreatedDate:      comment.CreatedDate,
			RiskLevel:        riskLevel,
			RiskScore:        riskScore,
			RiskConfidence:   reviewMetrics.Confidence,
			IssuesFound:      reviewMetrics.IssuesFound,
			SuggestionsCount: reviewMetrics.SuggestionsCount,
			FilesReviewed:    reviewMetrics.FilesReviewed,
			LinesReviewed:    reviewMetrics.LinesReviewed,
			EffortComplexity: reviewMetrics.Complexity,
			EffortMinutes:    reviewMetrics.EffortMinutes,
			ReviewState:      detectReviewState(comment.Body, comment.Status),
			SourcePlatform:   detectSourcePlatform(comment.PullRequestId),
			SourceUrl:        comment.PrUrl,
		}

		batch = append(batch, aiReview)

		if len(batch) >= batchSize {
			if err := saveBatch(db, batch); err != nil {
				return err
			}
			batch = make([]*models.AiReview, 0, batchSize)
		}
	}

	// Save remaining batch
	if len(batch) > 0 {
		if err := saveBatch(db, batch); err != nil {
			return err
		}
	}

	logger.Info("Completed AI review extraction: %d reviews found", len(processedReviews))
	return nil
}

// detectAiTool checks if the comment is from an AI review tool
func detectAiTool(data *AiReviewTaskData, accountId, body string) (string, bool) {
	// Check CodeRabbit
	if data.Options.ScopeConfig.CodeRabbitEnabled {
		if data.CodeRabbitUsernameRegex != nil && data.CodeRabbitUsernameRegex.MatchString(accountId) {
			return models.AiToolCodeRabbit, true
		}
		if data.CodeRabbitPatternRegex != nil && data.CodeRabbitPatternRegex.MatchString(body) {
			return models.AiToolCodeRabbit, true
		}
	}

	// Check Cursor Bugbot
	if data.Options.ScopeConfig.CursorBugbotEnabled {
		if data.CursorBugbotUsernameRegex != nil && data.CursorBugbotUsernameRegex.MatchString(accountId) {
			return models.AiToolCursorBugbot, true
		}
		if data.CursorBugbotPatternRegex != nil && data.CursorBugbotPatternRegex.MatchString(body) {
			return models.AiToolCursorBugbot, true
		}
	}

	return "", false
}

// generateReviewId creates a deterministic ID for an AI review
func generateReviewId(prId, commentId, aiTool string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", prId, commentId, aiTool)))
	return "aireview:" + hex.EncodeToString(hash[:16])
}

// ReviewMetrics holds parsed metrics from review content
type ReviewMetrics struct {
	IssuesFound      int
	SuggestionsCount int
	FilesReviewed    int
	LinesReviewed    int
	Complexity       string
	EffortMinutes    int
	Confidence       int
}

// parseReviewMetrics extracts metrics from review body
func parseReviewMetrics(body string) ReviewMetrics {
	metrics := ReviewMetrics{
		Confidence: 70, // Default confidence
	}

	// Parse effort/complexity (CodeRabbit format)
	complexityRe := regexp.MustCompile(`(?i)(simple|moderate|complex|trivial)`)
	if match := complexityRe.FindString(body); match != "" {
		metrics.Complexity = strings.ToLower(match)
		switch metrics.Complexity {
		case "trivial", "simple":
			metrics.EffortMinutes = 5
		case "moderate":
			metrics.EffortMinutes = 15
		case "complex":
			metrics.EffortMinutes = 30
		}
	}

	// Parse time estimate (e.g., "~12 minutes")
	timeRe := regexp.MustCompile(`~?(\d+)\s*minutes?`)
	if match := timeRe.FindStringSubmatch(body); len(match) > 1 {
		if val, err := strconv.Atoi(match[1]); err == nil {
			metrics.EffortMinutes = val
		}
	}

	// Count issue patterns
	issuePatterns := []string{
		`(?i)\b(bug|error|issue|problem|warning)\b`,
		`(?i)❌`,
		`(?i)⚠️`,
	}
	for _, pattern := range issuePatterns {
		re := regexp.MustCompile(pattern)
		metrics.IssuesFound += len(re.FindAllString(body, -1))
	}

	// Count suggestions
	suggestionRe := regexp.MustCompile(`(?i)(suggest|recommend|consider|should|could)`)
	metrics.SuggestionsCount = len(suggestionRe.FindAllString(body, -1))

	// Count file references
	fileRe := regexp.MustCompile(`\b[\w/]+\.(go|ts|js|py|java|rs|cpp|c|h)\b`)
	files := make(map[string]bool)
	for _, match := range fileRe.FindAllString(body, -1) {
		files[match] = true
	}
	metrics.FilesReviewed = len(files)

	// Parse lines changed (e.g., "+50 −36")
	linesRe := regexp.MustCompile(`\+(\d+)\s*[−-](\d+)`)
	if match := linesRe.FindStringSubmatch(body); len(match) > 2 {
		added, err1 := strconv.Atoi(match[1])
		removed, err2 := strconv.Atoi(match[2])
		if err1 == nil && err2 == nil {
			metrics.LinesReviewed = added + removed
		}
	}

	return metrics
}

// extractSummary extracts a summary from the review body
func extractSummary(body string) string {
	// Look for summary sections
	summaryRe := regexp.MustCompile(`(?is)(summary|overview|walkthrough)[:\s]*(.{1,500})`)
	if match := summaryRe.FindStringSubmatch(body); len(match) > 2 {
		summary := strings.TrimSpace(match[2])
		// Truncate at first double newline or 500 chars
		if idx := strings.Index(summary, "\n\n"); idx > 0 {
			summary = summary[:idx]
		}
		return summary
	}

	// Fallback: first 500 chars
	if len(body) > 500 {
		return body[:500] + "..."
	}
	return body
}

// detectRiskLevel analyzes the review body for risk indicators
func detectRiskLevel(data *AiReviewTaskData, body string) (string, int) {
	// Check patterns in order of severity
	if data.RiskHighPatternRegex != nil && data.RiskHighPatternRegex.MatchString(body) {
		return models.RiskLevelHigh, 80
	}
	if data.RiskMediumPatternRegex != nil && data.RiskMediumPatternRegex.MatchString(body) {
		return models.RiskLevelMedium, 50
	}
	if data.RiskLowPatternRegex != nil && data.RiskLowPatternRegex.MatchString(body) {
		return models.RiskLevelLow, 20
	}

	// Default to low risk if no patterns match
	return models.RiskLevelLow, 10
}

// detectReviewState determines the review outcome
func detectReviewState(body, status string) string {
	body = strings.ToLower(body)

	if strings.Contains(body, "approved") || strings.Contains(body, "lgtm") {
		return models.ReviewStateApproved
	}
	if strings.Contains(body, "changes requested") || strings.Contains(body, "request changes") {
		return models.ReviewStateChangesRequested
	}

	// Check status field
	switch strings.ToUpper(status) {
	case "APPROVED":
		return models.ReviewStateApproved
	case "CHANGES_REQUESTED":
		return models.ReviewStateChangesRequested
	}

	return models.ReviewStateCommented
}

// detectSourcePlatform determines if the PR is from GitHub or GitLab
func detectSourcePlatform(prId string) string {
	if strings.HasPrefix(prId, "github:") {
		return "github"
	}
	if strings.HasPrefix(prId, "gitlab:") {
		return "gitlab"
	}
	return "unknown"
}

// saveBatch saves a batch of AI reviews to the database
func saveBatch(db dal.Dal, batch []*models.AiReview) errors.Error {
	for _, review := range batch {
		err := db.CreateOrUpdate(review)
		if err != nil {
			return errors.Default.Wrap(err, "failed to save AI review")
		}
	}
	return nil
}
