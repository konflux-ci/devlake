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
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

var MatchSuggestionDiffsMeta = plugin.SubTaskMeta{
	Name:             "matchSuggestionDiffs",
	EntryPoint:       MatchSuggestionDiffs,
	EnabledByDefault: true,
	Description:      "Match AI suggestion findings against PR commit diffs to detect acceptance",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewFindingsMeta},
}

// prCommit holds commit info for a PR
type prCommit struct {
	CommitSha    string    `gorm:"column:commit_sha"`
	AuthoredDate time.Time `gorm:"column:commit_authored_date"`
	Message      string    `gorm:"column:message"`
	AuthorName   string    `gorm:"column:author_name"`
}

// commitFileChange holds a file changed in a commit
type commitFileChange struct {
	CommitSha string `gorm:"column:commit_sha"`
	FilePath  string `gorm:"column:file_path"`
	Additions int    `gorm:"column:additions"`
	Deletions int    `gorm:"column:deletions"`
}

// suggestionFinding holds a finding with its context for matching
type suggestionFinding struct {
	models.AiReviewFinding
	ReviewCreatedDate time.Time `gorm:"column:review_created_date"`
	AiToolUser        string    `gorm:"column:ai_tool_user"`
}

// matchResult holds the result of matching a suggestion against commits
type matchResult struct {
	FindingId    string
	Matched      bool
	Method       string
	Score        int
	CommitSha    string
	MatchedFile  string
}

// MatchSuggestionDiffs compares AI suggestions against PR commit diffs to detect acceptance.
//
// It uses three matching strategies (in priority order):
//  1. Commit message matching: detects GitHub "Apply suggestion" button clicks
//  2. File path + temporal proximity: same file modified shortly after suggestion
//  3. Resolves file paths from raw data when the domain table lacks them
func MatchSuggestionDiffs(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	repoId := data.Options.RepoId
	logger.Info("Starting diff-based suggestion matching for repo: %s", repoId)

	// Step 1: Resolve file paths from raw data for findings that lack them
	enriched, err := enrichFindingFilePaths(db, data)
	if err != nil {
		logger.Warn(err, "file path enrichment had errors, continuing with available data")
	}
	logger.Info("Enriched %d findings with file paths from raw data", enriched)

	// Step 2: Get all suggestion findings for this repo
	findings, loadErr := loadSuggestionFindings(db, repoId)
	if loadErr != nil {
		return errors.Default.Wrap(loadErr, "failed to load suggestion findings")
	}
	if len(findings) == 0 {
		logger.Info("No suggestion findings to match, skipping")
		return nil
	}
	logger.Info("Found %d suggestion findings to match against commits", len(findings))

	// Step 3: Group findings by PR
	findingsByPR := make(map[string][]suggestionFinding)
	for _, f := range findings {
		findingsByPR[f.PullRequestId] = append(findingsByPR[f.PullRequestId], f)
	}

	// Step 4: For each PR, load commits and match
	totalMatched := 0
	reviewAcceptedCounts := make(map[string]int) // ai_review_id -> count of diff-accepted

	for prId, prFindings := range findingsByPR {
		// Get commits for this PR
		commits, loadErr := loadPRCommits(db, prId)
		if loadErr != nil {
			logger.Warn(loadErr, "failed to load commits for PR %s, skipping", prId)
			continue
		}
		if len(commits) == 0 {
			continue
		}

		// Get file changes for these commits
		commitShas := make([]string, len(commits))
		for i, c := range commits {
			commitShas[i] = c.CommitSha
		}
		fileChanges, loadErr := loadCommitFiles(db, commitShas)
		if loadErr != nil {
			logger.Warn(loadErr, "failed to load commit files for PR %s, skipping", prId)
			continue
		}

		// Match each finding
		for _, finding := range prFindings {
			result := matchFinding(finding, commits, fileChanges)
			if result.Matched {
				totalMatched++
				reviewAcceptedCounts[finding.AiReviewId]++

				updateErr := db.UpdateColumns(
					&models.AiReviewFinding{},
					[]dal.DalSet{
						{ColumnName: "suggestion_diff_matched", Value: true},
						{ColumnName: "suggestion_match_method", Value: result.Method},
						{ColumnName: "suggestion_match_score", Value: result.Score},
						{ColumnName: "matched_commit_sha", Value: result.CommitSha},
						{ColumnName: "matched_file_path", Value: result.MatchedFile},
					},
					dal.Where("id = ?", result.FindingId),
				)
				if updateErr != nil {
					logger.Warn(updateErr, "failed to update finding %s", result.FindingId)
				}
			}
		}
	}

	// Step 5: Update review-level diff-accepted counts
	for reviewId, count := range reviewAcceptedCounts {
		updateErr := db.UpdateColumns(
			&models.AiReview{},
			[]dal.DalSet{
				{ColumnName: "suggestions_diff_accepted", Value: count},
			},
			dal.Where("id = ?", reviewId),
		)
		if updateErr != nil {
			logger.Warn(updateErr, "failed to update diff accepted count for review %s", reviewId)
		}
	}

	logger.Info("Diff-based suggestion matching complete: %d/%d findings matched", totalMatched, len(findings))
	return nil
}

// loadSuggestionFindings gets all suggestion-type findings for the repo
func loadSuggestionFindings(db dal.Dal, repoId string) ([]suggestionFinding, error) {
	cursor, err := db.Cursor(
		dal.Select("f.*, ar.created_date as review_created_date, ar.ai_tool_user"),
		dal.From("_tool_aireview_findings f"),
		dal.Join("JOIN _tool_aireview_reviews ar ON f.ai_review_id = ar.id"),
		dal.Where("f.repo_id = ? AND f.type = ?", repoId, models.FindingTypeSuggestion),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	var findings []suggestionFinding
	for cursor.Next() {
		var f suggestionFinding
		if scanErr := db.Fetch(cursor, &f); scanErr != nil {
			return nil, scanErr
		}
		findings = append(findings, f)
	}
	return findings, nil
}

// loadPRCommits gets all commits for a PR, joined with commit metadata
func loadPRCommits(db dal.Dal, prId string) ([]prCommit, error) {
	cursor, err := db.Cursor(
		dal.Select("prc.commit_sha, prc.commit_authored_date, c.message, c.author_name"),
		dal.From("pull_request_commits prc"),
		dal.Join("LEFT JOIN commits c ON prc.commit_sha = c.sha"),
		dal.Where("prc.pull_request_id = ?", prId),
		dal.Orderby("prc.commit_authored_date ASC"),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	var commits []prCommit
	for cursor.Next() {
		var c prCommit
		if scanErr := db.Fetch(cursor, &c); scanErr != nil {
			return nil, scanErr
		}
		commits = append(commits, c)
	}
	return commits, nil
}

// loadCommitFiles gets file changes for a set of commits
func loadCommitFiles(db dal.Dal, commitShas []string) ([]commitFileChange, error) {
	if len(commitShas) == 0 {
		return nil, nil
	}
	cursor, err := db.Cursor(
		dal.Select("commit_sha, file_path, additions, deletions"),
		dal.From("commit_files"),
		dal.Where("commit_sha IN (?)", commitShas),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	var files []commitFileChange
	for cursor.Next() {
		var f commitFileChange
		if scanErr := db.Fetch(cursor, &f); scanErr != nil {
			return nil, scanErr
		}
		files = append(files, f)
	}
	return files, nil
}

// matchFinding attempts to match a suggestion finding against PR commits.
// Returns the best match found across all strategies.
func matchFinding(finding suggestionFinding, commits []prCommit, fileChanges []commitFileChange) matchResult {
	result := matchResult{FindingId: finding.Id}

	// Get the file path for this finding
	filePath := finding.MatchedFilePath
	if filePath == "" {
		filePath = finding.FilePath
	}

	// Build file change index: commitSha -> []filePath
	commitFiles := make(map[string][]commitFileChange)
	for _, fc := range fileChanges {
		commitFiles[fc.CommitSha] = append(commitFiles[fc.CommitSha], fc)
	}

	// Filter to commits after the suggestion was made
	suggestionTime := finding.ReviewCreatedDate
	if suggestionTime.IsZero() {
		suggestionTime = finding.CreatedDate
	}

	// Strategy 1: Check for "Apply suggestion" commit messages
	for _, commit := range commits {
		if commit.AuthoredDate.Before(suggestionTime) {
			continue
		}
		if isApplySuggestionCommit(commit.Message, finding.AiToolUser) {
			// Check if this commit touches the right file
			if filePath != "" {
				for _, fc := range commitFiles[commit.CommitSha] {
					if filePathsMatch(fc.FilePath, filePath) {
						result.Matched = true
						result.Method = "diff_commit_msg"
						result.Score = 95
						result.CommitSha = commit.CommitSha
						result.MatchedFile = fc.FilePath
						return result
					}
				}
			}
			// Even without file path match, "Apply suggestion" commit is strong signal
			if len(commitFiles[commit.CommitSha]) == 1 {
				result.Matched = true
				result.Method = "diff_commit_msg"
				result.Score = 85
				result.CommitSha = commit.CommitSha
				result.MatchedFile = commitFiles[commit.CommitSha][0].FilePath
				return result
			}
		}
	}

	// Strategy 2: File path + temporal proximity
	if filePath == "" {
		return result
	}

	for _, commit := range commits {
		if commit.AuthoredDate.Before(suggestionTime) {
			continue
		}

		timeDelta := commit.AuthoredDate.Sub(suggestionTime)
		for _, fc := range commitFiles[commit.CommitSha] {
			if !filePathsMatch(fc.FilePath, filePath) {
				continue
			}

			// File was modified after the suggestion — score based on temporal proximity
			score := calculateTemporalScore(timeDelta)
			if score > result.Score {
				result.Matched = true
				result.Method = "diff_file_temporal"
				result.Score = score
				result.CommitSha = commit.CommitSha
				result.MatchedFile = fc.FilePath
			}
		}
	}

	return result
}

// applySuggestionRe matches GitHub "Apply suggestion" commit messages and co-author lines
var applySuggestionRe = regexp.MustCompile(`(?i)(?:apply\s+suggest|co-authored-by:.*(?:coderabbit|qodo|gemini|cursor|copilot|bugbot))`)

// isApplySuggestionCommit checks if a commit message indicates a suggestion was applied
func isApplySuggestionCommit(message, aiToolUser string) bool {
	if message == "" {
		return false
	}
	if applySuggestionRe.MatchString(message) {
		return true
	}
	// Check for the specific AI tool user in co-authored-by
	if aiToolUser != "" && strings.Contains(strings.ToLower(message), strings.ToLower(aiToolUser)) {
		return true
	}
	return false
}

// filePathsMatch checks if two file paths refer to the same file.
// Handles cases where one path has a prefix (e.g., "a/pkg/foo.go" vs "pkg/foo.go")
func filePathsMatch(path1, path2 string) bool {
	if path1 == path2 {
		return true
	}
	// Normalize: strip leading a/ or b/ (git diff prefixes)
	p1 := strings.TrimPrefix(strings.TrimPrefix(path1, "a/"), "b/")
	p2 := strings.TrimPrefix(strings.TrimPrefix(path2, "a/"), "b/")
	if p1 == p2 {
		return true
	}
	// Check suffix match (one might have a longer prefix)
	return strings.HasSuffix(p1, "/"+p2) || strings.HasSuffix(p2, "/"+p1)
}

// calculateTemporalScore returns a confidence score based on how soon after the suggestion
// the commit was made. Shorter time = higher confidence.
func calculateTemporalScore(delta time.Duration) int {
	switch {
	case delta < 30*time.Minute:
		return 75 // Very likely applied
	case delta < 2*time.Hour:
		return 60 // Probably applied
	case delta < 24*time.Hour:
		return 45 // Possibly applied
	case delta < 72*time.Hour:
		return 30 // Weak signal
	default:
		return 0 // Too far apart
	}
}

// enrichFindingFilePaths resolves file paths from raw data for findings that lack them.
// It follows the same raw-data-access pattern as EnrichGithubReviewReactions.
func enrichFindingFilePaths(db dal.Dal, data *AiReviewTaskData) (int, errors.Error) {
	repoId := data.Options.RepoId

	// Find findings that need file path enrichment: have a review linked to a comment
	// with raw data, and currently have no file path
	var clauses []dal.Clause
	if data.Options.ProjectName != "" {
		clauses = []dal.Clause{
			dal.Select("f.id as finding_id, prc._raw_data_table, prc._raw_data_id"),
			dal.From("_tool_aireview_findings f"),
			dal.Join("JOIN _tool_aireview_reviews ar ON f.ai_review_id = ar.id"),
			dal.Join("JOIN pull_request_comments prc ON ar.review_id = prc.id"),
			dal.Join("JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Join("JOIN project_mapping pm ON pr.base_repo_id = pm.row_id"),
			dal.Where("pm.project_name = ? AND pm.`table` = ? AND f.file_path = '' AND prc._raw_data_table != ''",
				data.Options.ProjectName, "repos"),
		}
	} else {
		clauses = []dal.Clause{
			dal.Select("f.id as finding_id, prc._raw_data_table, prc._raw_data_id"),
			dal.From("_tool_aireview_findings f"),
			dal.Join("JOIN _tool_aireview_reviews ar ON f.ai_review_id = ar.id"),
			dal.Join("JOIN pull_request_comments prc ON ar.review_id = prc.id"),
			dal.Join("JOIN pull_requests pr ON prc.pull_request_id = pr.id"),
			dal.Where("pr.base_repo_id = ? AND (f.file_path = '' OR f.file_path IS NULL) AND prc._raw_data_table != ''", repoId),
		}
	}

	type findingRawLink struct {
		FindingId    string `gorm:"column:finding_id"`
		RawDataTable string `gorm:"column:_raw_data_table"`
		RawDataId    uint64 `gorm:"column:_raw_data_id"`
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return 0, errors.Default.Wrap(err, "failed to query findings for file path enrichment")
	}
	defer cursor.Close()

	// Group by raw table
	tableLinks := make(map[string][]findingRawLink)
	for cursor.Next() {
		var link findingRawLink
		if fetchErr := db.Fetch(cursor, &link); fetchErr != nil {
			return 0, errors.Default.Wrap(fetchErr, "failed to fetch finding raw link")
		}
		tableLinks[link.RawDataTable] = append(tableLinks[link.RawDataTable], link)
	}

	enriched := 0
	for rawTable, links := range tableLinks {
		rawIdToFindingId := make(map[uint64]string)
		rawIds := make([]uint64, 0, len(links))
		for _, link := range links {
			rawIdToFindingId[link.RawDataId] = link.FindingId
			rawIds = append(rawIds, link.RawDataId)
		}

		// Query raw table for file path
		// GitHub: $.path, GitLab: $.position.new_path
		pathExpr := "JSON_EXTRACT(CONVERT(data USING utf8mb4), '$.path')"
		if strings.Contains(rawTable, "gitlab") {
			pathExpr = "COALESCE(JSON_EXTRACT(CONVERT(data USING utf8mb4), '$.position.new_path'), JSON_EXTRACT(CONVERT(data USING utf8mb4), '$.position.old_path'))"
		}

		batchSize := 500
		for i := 0; i < len(rawIds); i += batchSize {
			end := i + batchSize
			if end > len(rawIds) {
				end = len(rawIds)
			}
			batch := rawIds[i:end]

			rows, queryErr := db.Cursor(
				dal.Select("id, "+pathExpr+" as file_path"),
				dal.From(rawTable),
				dal.Where("id IN (?)", batch),
			)
			if queryErr != nil {
				continue
			}

			for rows.Next() {
				var id uint64
				var filePath *string
				if scanErr := rows.Scan(&id, &filePath); scanErr != nil {
					continue
				}

				findingId, ok := rawIdToFindingId[id]
				if !ok || filePath == nil || *filePath == "" {
					continue
				}

				// JSON_EXTRACT returns quoted strings, strip quotes
				cleanPath := strings.Trim(*filePath, "\"")
				if cleanPath == "" || cleanPath == "null" {
					continue
				}

				updateErr := db.UpdateColumn(
					&models.AiReviewFinding{}, "matched_file_path", cleanPath,
					dal.Where("id = ?", findingId),
				)
				if updateErr == nil {
					enriched++
				}
			}
			rows.Close()
		}
	}

	return enriched, nil
}
