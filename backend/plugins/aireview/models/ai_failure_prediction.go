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

package models

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

// AiFailurePrediction tracks the accuracy of AI predictions vs actual outcomes
// This is used to calculate the "AI Predicted Failure Avoidance" metric
type AiFailurePrediction struct {
	common.NoPKModel

	// Primary key
	Id string `gorm:"primaryKey;type:varchar(255)"`

	// Foreign key to pull_requests domain table
	PullRequestId string `gorm:"index;type:varchar(255)"`

	// Repository reference
	RepoId string `gorm:"index;type:varchar(255)"`

	// AI tool that made the prediction
	AiTool string `gorm:"type:varchar(100)"`

	// Prediction data
	WasFlaggedRisky bool      // Did AI flag this PR as risky?
	RiskScore       int       // Risk score assigned (0-100)
	FlaggedAt       time.Time // When AI made the assessment

	// Actual outcome data (tracked post-merge)
	PrMergedAt     *time.Time // When PR was merged
	HadCiFailure   bool       // Did CI fail after merge?
	CiFailureAt    *time.Time // When CI failure occurred
	HadBugReported bool       // Was a bug reported within window?
	BugReportedAt  *time.Time // When bug was reported
	BugIssueId     string     `gorm:"type:varchar(255)"` // Link to bug issue
	HadRollback    bool       // Was the change rolled back?
	RollbackAt     *time.Time // When rollback occurred

	// Classification for confusion matrix
	// TP: WasFlaggedRisky=true AND (HadCiFailure OR HadBugReported)
	// FP: WasFlaggedRisky=true AND NOT (HadCiFailure OR HadBugReported)
	// FN: WasFlaggedRisky=false AND (HadCiFailure OR HadBugReported)
	// TN: WasFlaggedRisky=false AND NOT (HadCiFailure OR HadBugReported)
	PredictionOutcome string `gorm:"type:varchar(20)"` // TP, FP, FN, TN

	// Time windows
	ObservationWindowDays int       // How many days after merge to track (default 14)
	ObservationEndDate    time.Time // When observation window ends

	// Metadata
	CreatedAt time.Time
	UpdatedAt *time.Time
}

func (AiFailurePrediction) TableName() string {
	return "_tool_aireview_failure_predictions"
}

// Prediction outcome constants
const (
	PredictionTP = "TP" // True Positive: AI flagged, failure occurred
	PredictionFP = "FP" // False Positive: AI flagged, no failure
	PredictionFN = "FN" // False Negative: AI didn't flag, failure occurred
	PredictionTN = "TN" // True Negative: AI didn't flag, no failure
)

// AiPredictionMetrics stores aggregated prediction metrics for reporting
type AiPredictionMetrics struct {
	common.NoPKModel

	// Primary key
	Id string `gorm:"primaryKey;type:varchar(255)"`

	// Scope
	RepoId string `gorm:"index;type:varchar(255)"`
	AiTool string `gorm:"type:varchar(100)"`

	// Time period
	PeriodStart time.Time `gorm:"index"`
	PeriodEnd   time.Time
	PeriodType  string `gorm:"type:varchar(20)"` // daily, weekly, monthly

	// Confusion matrix counts
	TruePositives  int
	FalsePositives int
	FalseNegatives int
	TrueNegatives  int

	// Calculated metrics
	Precision float64 // TP / (TP + FP)
	Recall    float64 // TP / (TP + FN)
	Accuracy  float64 // (TP + TN) / Total
	F1Score   float64 // 2 * (Precision * Recall) / (Precision + Recall)

	// Sample sizes
	TotalPrs    int
	FlaggedPrs  int
	FailedPrs   int
	ObservedPrs int // PRs that completed observation window

	// Thresholds and recommendations
	RecommendedAutonomyLevel string `gorm:"type:varchar(50)"` // auto_block, mandatory_review, advisory_only

	// Timestamps
	CalculatedAt time.Time
}

func (AiPredictionMetrics) TableName() string {
	return "_tool_aireview_prediction_metrics"
}

// Autonomy level constants
const (
	AutonomyAutoBlock       = "auto_block"       // Precision > 80%, Recall > 70%
	AutonomyMandatoryReview = "mandatory_review" // Precision 60-80%, Recall 50-70%
	AutonomyAdvisoryOnly    = "advisory_only"    // Precision < 60%, Recall < 50%
)
