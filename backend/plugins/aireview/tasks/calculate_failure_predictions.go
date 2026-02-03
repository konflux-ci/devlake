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
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

var CalculateFailurePredictionsMeta = plugin.SubTaskMeta{
	Name:             "calculateFailurePredictions",
	EntryPoint:       CalculateFailurePredictions,
	EnabledByDefault: true,
	Description:      "Calculate AI failure prediction outcomes based on PR outcomes",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractAiReviewsMeta},
}

// CalculateFailurePredictions tracks AI prediction accuracy against actual outcomes
func CalculateFailurePredictions(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	observationDays := data.Options.ScopeConfig.ObservationWindowDays
	if observationDays == 0 {
		observationDays = 14 // Default 14 days
	}

	logger.Info("Calculating failure predictions for repo: %s (window: %d days)", data.Options.RepoId, observationDays)

	// Get merged PRs with AI reviews
	cursor, err := db.Cursor(
		dal.Select("pr.*, ar.ai_tool, ar.risk_score, ar.risk_level, ar.created_date as review_created_date"),
		dal.From("pull_requests pr"),
		dal.Join("LEFT JOIN _tool_aireview_reviews ar ON pr.id = ar.pull_request_id"),
		dal.Where("pr.base_repo_id = ? AND pr.status = ?", data.Options.RepoId, "MERGED"),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query merged PRs")
	}
	defer cursor.Close()

	batchSize := 100
	batch := make([]*models.AiFailurePrediction, 0, batchSize)
	processedPrs := 0

	for cursor.Next() {
		var prData struct {
			code.PullRequest
			AiTool            string     `gorm:"column:ai_tool"`
			RiskScore         int        `gorm:"column:risk_score"`
			RiskLevel         string     `gorm:"column:risk_level"`
			ReviewCreatedDate *time.Time `gorm:"column:review_created_date"`
		}

		if err := db.Fetch(cursor, &prData); err != nil {
			return errors.Default.Wrap(err, "failed to fetch PR data")
		}

		// Skip if no merge date
		if prData.MergedDate == nil {
			continue
		}

		// Calculate observation window
		observationEnd := prData.MergedDate.Add(time.Duration(observationDays) * 24 * time.Hour)
		now := time.Now()

		// Check if observation window has completed
		observationComplete := now.After(observationEnd)

		// Determine if AI flagged as risky
		wasFlaggedRisky := prData.RiskLevel == models.RiskLevelHigh ||
			prData.RiskLevel == models.RiskLevelCritical ||
			prData.RiskScore >= 70

		// Check for CI failures (from cicd domain tables)
		hadCiFailure, ciFailureAt := checkCiFailures(db, prData.Id, *prData.MergedDate, observationEnd)

		// Check for bugs reported (from issues domain tables)
		hadBugReported, bugReportedAt, bugIssueId := checkBugReports(db, prData.Id, *prData.MergedDate, observationEnd)

		// Check for rollbacks
		hadRollback, rollbackAt := checkRollbacks(db, prData.Id, *prData.MergedDate, observationEnd)

		// Determine outcome (only if observation complete)
		predictionOutcome := ""
		if observationComplete {
			actualFailure := hadCiFailure || hadBugReported || hadRollback
			predictionOutcome = calculateOutcome(wasFlaggedRisky, actualFailure)
		}

		// Generate prediction ID
		predictionId := generatePredictionId(prData.Id, prData.AiTool)

		prediction := &models.AiFailurePrediction{
			Id:                    predictionId,
			PullRequestId:         prData.Id,
			RepoId:                data.Options.RepoId,
			AiTool:                prData.AiTool,
			WasFlaggedRisky:       wasFlaggedRisky,
			RiskScore:             prData.RiskScore,
			PrMergedAt:            prData.MergedDate,
			HadCiFailure:          hadCiFailure,
			CiFailureAt:           ciFailureAt,
			HadBugReported:        hadBugReported,
			BugReportedAt:         bugReportedAt,
			BugIssueId:            bugIssueId,
			HadRollback:           hadRollback,
			RollbackAt:            rollbackAt,
			PredictionOutcome:     predictionOutcome,
			ObservationWindowDays: observationDays,
			ObservationEndDate:    observationEnd,
			CreatedAt:             time.Now(),
		}

		if prData.ReviewCreatedDate != nil {
			prediction.FlaggedAt = *prData.ReviewCreatedDate
		}

		batch = append(batch, prediction)
		processedPrs++

		if len(batch) >= batchSize {
			if err := savePredictionsBatch(db, batch); err != nil {
				return err
			}
			batch = make([]*models.AiFailurePrediction, 0, batchSize)
		}
	}

	// Save remaining batch
	if len(batch) > 0 {
		if err := savePredictionsBatch(db, batch); err != nil {
			return err
		}
	}

	logger.Info("Completed failure prediction calculation: %d PRs processed", processedPrs)
	return nil
}

// checkCiFailures checks for CI failures after PR merge
func checkCiFailures(db dal.Dal, prId string, mergedAt, observationEnd time.Time) (bool, *time.Time) {
	// Query cicd_pipeline_commits joined with cicd_pipelines for failures
	var result struct {
		FailedAt *time.Time `gorm:"column:finished_date"`
	}

	err := db.First(&result,
		dal.Select("cp.finished_date"),
		dal.From("cicd_pipeline_commits cpc"),
		dal.Join("JOIN cicd_pipelines cp ON cpc.pipeline_id = cp.id"),
		dal.Where("cpc.commit_sha IN (SELECT merge_commit_sha FROM pull_requests WHERE id = ?)", prId),
		dal.Where("cp.status = ?", "FAILURE"),
		dal.Where("cp.finished_date BETWEEN ? AND ?", mergedAt, observationEnd),
		dal.Orderby("cp.finished_date ASC"),
		dal.Limit(1),
	)

	if err != nil || result.FailedAt == nil {
		return false, nil
	}

	return true, result.FailedAt
}

// checkBugReports checks for bug reports linked to the PR
func checkBugReports(db dal.Dal, prId string, mergedAt, observationEnd time.Time) (bool, *time.Time, string) {
	// Query issues that reference this PR
	var result struct {
		IssueId   string     `gorm:"column:id"`
		CreatedAt *time.Time `gorm:"column:created_date"`
	}

	// Check pull_request_issues table for linked issues
	err := db.First(&result,
		dal.Select("i.id, i.created_date"),
		dal.From("pull_request_issues pri"),
		dal.Join("JOIN issues i ON pri.issue_id = i.id"),
		dal.Where("pri.pull_request_id = ?", prId),
		dal.Where("i.type = ?", "BUG"),
		dal.Where("i.created_date BETWEEN ? AND ?", mergedAt, observationEnd),
		dal.Orderby("i.created_date ASC"),
		dal.Limit(1),
	)

	if err != nil || result.CreatedAt == nil {
		return false, nil, ""
	}

	return true, result.CreatedAt, result.IssueId
}

// checkRollbacks checks for rollback commits
func checkRollbacks(db dal.Dal, _ string, mergedAt, observationEnd time.Time) (bool, *time.Time) {
	// Look for revert commits in the repository
	var result struct {
		RollbackAt *time.Time `gorm:"column:authored_date"`
	}

	// Query commits with "revert" in message that reference the merged commit
	err := db.First(&result,
		dal.Select("c.authored_date"),
		dal.From("commits c"),
		dal.Join("JOIN pull_request_commits prc ON c.sha = prc.commit_sha"),
		dal.Where("c.message LIKE ?", "%revert%"),
		dal.Where("c.authored_date BETWEEN ? AND ?", mergedAt, observationEnd),
		dal.Orderby("c.authored_date ASC"),
		dal.Limit(1),
	)

	if err != nil || result.RollbackAt == nil {
		return false, nil
	}

	return true, result.RollbackAt
}

// calculateOutcome determines the prediction outcome (TP, FP, FN, TN)
func calculateOutcome(wasFlaggedRisky, actualFailure bool) string {
	if wasFlaggedRisky && actualFailure {
		return models.PredictionTP // True Positive
	}
	if wasFlaggedRisky && !actualFailure {
		return models.PredictionFP // False Positive
	}
	if !wasFlaggedRisky && actualFailure {
		return models.PredictionFN // False Negative
	}
	return models.PredictionTN // True Negative
}

// generatePredictionId creates a deterministic ID for a prediction
func generatePredictionId(prId, aiTool string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", prId, aiTool)))
	return "aipred:" + hex.EncodeToString(hash[:16])
}

// savePredictionsBatch saves a batch of predictions to the database
func savePredictionsBatch(db dal.Dal, batch []*models.AiFailurePrediction) errors.Error {
	for _, prediction := range batch {
		err := db.CreateOrUpdate(prediction)
		if err != nil {
			return errors.Default.Wrap(err, "failed to save failure prediction")
		}
	}
	return nil
}
