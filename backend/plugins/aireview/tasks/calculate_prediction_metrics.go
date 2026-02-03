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
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

var CalculatePredictionMetricsMeta = plugin.SubTaskMeta{
	Name:             "calculatePredictionMetrics",
	EntryPoint:       CalculatePredictionMetrics,
	EnabledByDefault: true,
	Description:      "Calculate aggregated AI prediction metrics (precision, recall, accuracy)",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
	Dependencies:     []*plugin.SubTaskMeta{&CalculateFailurePredictionsMeta},
}

// CalculatePredictionMetrics aggregates prediction data into metrics
func CalculatePredictionMetrics(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AiReviewTaskData)

	logger.Info("Calculating prediction metrics for repo: %s", data.Options.RepoId)

	// Get distinct AI tools used in this repo
	var aiTools []string
	err := db.All(&aiTools,
		dal.Select("DISTINCT ai_tool"),
		dal.From(&models.AiFailurePrediction{}),
		dal.Where("repo_id = ? AND prediction_outcome != ''", data.Options.RepoId),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to get AI tools")
	}

	// Calculate metrics for each AI tool and time period
	now := time.Now()
	periods := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{"daily", now.AddDate(0, 0, -1), now},
		{"weekly", now.AddDate(0, 0, -7), now},
		{"monthly", now.AddDate(0, -1, 0), now},
		{"rolling_60d", now.AddDate(0, 0, -60), now},
	}

	for _, aiTool := range aiTools {
		if aiTool == "" {
			continue
		}

		for _, period := range periods {
			metrics, calcErr := calculateMetricsForPeriod(db, data.Options.RepoId, aiTool, period.name, period.start, period.end)
			if calcErr != nil {
				logger.Warn(calcErr, "Failed to calculate metrics for %s/%s", aiTool, period.name)
				continue
			}

			if metrics.TotalPrs == 0 {
				continue
			}

			err := db.CreateOrUpdate(metrics)
			if err != nil {
				return errors.Default.Wrap(err, "failed to save prediction metrics")
			}
		}
	}

	logger.Info("Completed prediction metrics calculation")
	return nil
}

// calculateMetricsForPeriod calculates metrics for a specific time period
func calculateMetricsForPeriod(db dal.Dal, repoId, aiTool, periodType string, periodStart, periodEnd time.Time) (*models.AiPredictionMetrics, errors.Error) {
	// Query confusion matrix counts
	var counts struct {
		TruePositives  int `gorm:"column:tp"`
		FalsePositives int `gorm:"column:fp"`
		FalseNegatives int `gorm:"column:fn"`
		TrueNegatives  int `gorm:"column:tn"`
		TotalPrs       int `gorm:"column:total"`
		FlaggedPrs     int `gorm:"column:flagged"`
		FailedPrs      int `gorm:"column:failed"`
	}

	err := db.First(&counts,
		dal.Select(`
			SUM(CASE WHEN prediction_outcome = 'TP' THEN 1 ELSE 0 END) as tp,
			SUM(CASE WHEN prediction_outcome = 'FP' THEN 1 ELSE 0 END) as fp,
			SUM(CASE WHEN prediction_outcome = 'FN' THEN 1 ELSE 0 END) as fn,
			SUM(CASE WHEN prediction_outcome = 'TN' THEN 1 ELSE 0 END) as tn,
			COUNT(*) as total,
			SUM(CASE WHEN was_flagged_risky THEN 1 ELSE 0 END) as flagged,
			SUM(CASE WHEN had_ci_failure OR had_bug_reported OR had_rollback THEN 1 ELSE 0 END) as failed
		`),
		dal.From(&models.AiFailurePrediction{}),
		dal.Where("repo_id = ? AND ai_tool = ?", repoId, aiTool),
		dal.Where("pr_merged_at BETWEEN ? AND ?", periodStart, periodEnd),
		dal.Where("prediction_outcome != ''"),
	)
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query prediction counts")
	}

	// Calculate metrics
	tp := float64(counts.TruePositives)
	fp := float64(counts.FalsePositives)
	fn := float64(counts.FalseNegatives)
	tn := float64(counts.TrueNegatives)
	total := tp + fp + fn + tn

	var precision, recall, accuracy, f1Score float64

	if tp+fp > 0 {
		precision = tp / (tp + fp)
	}
	if tp+fn > 0 {
		recall = tp / (tp + fn)
	}
	if total > 0 {
		accuracy = (tp + tn) / total
	}
	if precision+recall > 0 {
		f1Score = 2 * (precision * recall) / (precision + recall)
	}

	// Determine recommended autonomy level
	autonomyLevel := determineAutonomyLevel(precision, recall)

	// Generate metrics ID
	metricsId := generateMetricsId(repoId, aiTool, periodType, periodStart)

	return &models.AiPredictionMetrics{
		Id:                       metricsId,
		RepoId:                   repoId,
		AiTool:                   aiTool,
		PeriodStart:              periodStart,
		PeriodEnd:                periodEnd,
		PeriodType:               periodType,
		TruePositives:            counts.TruePositives,
		FalsePositives:           counts.FalsePositives,
		FalseNegatives:           counts.FalseNegatives,
		TrueNegatives:            counts.TrueNegatives,
		Precision:                precision,
		Recall:                   recall,
		Accuracy:                 accuracy,
		F1Score:                  f1Score,
		TotalPrs:                 counts.TotalPrs,
		FlaggedPrs:               counts.FlaggedPrs,
		FailedPrs:                counts.FailedPrs,
		ObservedPrs:              int(total),
		RecommendedAutonomyLevel: autonomyLevel,
		CalculatedAt:             time.Now(),
	}, nil
}

// determineAutonomyLevel recommends AI autonomy based on metrics
func determineAutonomyLevel(precision, recall float64) string {
	// Decision framework from AI metrics spec:
	// > 80% precision AND > 70% recall → auto_block
	// 60-80% precision AND 50-70% recall → mandatory_review
	// < 60% precision OR < 50% recall → advisory_only

	if precision >= 0.80 && recall >= 0.70 {
		return models.AutonomyAutoBlock
	}
	if precision >= 0.60 && recall >= 0.50 {
		return models.AutonomyMandatoryReview
	}
	return models.AutonomyAdvisoryOnly
}

// generateMetricsId creates a deterministic ID for metrics
func generateMetricsId(repoId, aiTool, periodType string, periodStart time.Time) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s", repoId, aiTool, periodType, periodStart.Format("2006-01-02"))))
	return "aimetrics:" + hex.EncodeToString(hash[:16])
}
