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

// AiReviewFinding represents a specific issue or suggestion from an AI review
type AiReviewFinding struct {
	common.NoPKModel

	// Primary key
	Id string `gorm:"primaryKey;type:varchar(255)"`

	// Foreign key to AiReview
	AiReviewId string `gorm:"index;type:varchar(255)"`

	// Foreign key to pull_requests domain table
	PullRequestId string `gorm:"index;type:varchar(255)"`

	// Repository reference
	RepoId string `gorm:"index;type:varchar(255)"`

	// AI tool information
	AiTool string `gorm:"type:varchar(100)"`

	// Finding classification
	Category string `gorm:"type:varchar(100)"` // security, performance, best_practice, bug, style
	Severity string `gorm:"type:varchar(50)"`  // info, warning, error, critical
	Type     string `gorm:"type:varchar(100)"` // suggestion, issue, comment

	// Finding details
	Title       string `gorm:"type:varchar(500)"`
	Description string `gorm:"type:text"`
	FilePath    string `gorm:"type:varchar(500)"`
	LineStart   int
	LineEnd     int
	CommitSha   string `gorm:"type:varchar(255)"`

	// Code context
	CodeSnippet       string `gorm:"type:text"` // Original code
	SuggestedCode     string `gorm:"type:text"` // Suggested fix
	SuggestionApplied bool   // Whether the suggestion was applied

	// Resolution tracking
	IsResolved   bool
	ResolvedAt   *time.Time
	ResolvedBy   string `gorm:"type:varchar(255)"`
	Resolution   string `gorm:"type:varchar(100)"` // fixed, wont_fix, false_positive
	ResponseTime int    // Minutes to resolution

	// Timestamps
	CreatedDate time.Time `gorm:"index"`

	// Source information
	SourceCommentId string `gorm:"type:varchar(255)"`
}

func (AiReviewFinding) TableName() string {
	return "_tool_aireview_findings"
}

// Category constants
const (
	FindingCategorySecurity        = "security"
	FindingCategoryPerformance     = "performance"
	FindingCategoryBestPractice    = "best_practice"
	FindingCategoryBug             = "bug"
	FindingCategoryStyle           = "style"
	FindingCategoryDocumentation   = "documentation"
	FindingCategoryMaintainability = "maintainability"
)

// Severity constants
const (
	FindingSeverityInfo     = "info"
	FindingSeverityWarning  = "warning"
	FindingSeverityError    = "error"
	FindingSeverityCritical = "critical"
)

// Finding type constants
const (
	FindingTypeSuggestion = "suggestion"
	FindingTypeIssue      = "issue"
	FindingTypeComment    = "comment"
	FindingTypeApproval   = "approval"
)

// Resolution constants
const (
	ResolutionFixed         = "fixed"
	ResolutionWontFix       = "wont_fix"
	ResolutionFalsePositive = "false_positive"
)
