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

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
)

// AiReviewOptions contains the configuration options for the plugin
type AiReviewOptions struct {
	// Project name (for project metric mode - analyzes all repos in project)
	ProjectName string `json:"projectName"`

	// Repository to analyze (domain layer ID) - optional if projectName is provided
	RepoId string `json:"repoId"`

	// Scope config ID reference
	ScopeConfigId uint64 `json:"scopeConfigId"`

	// Inline scope config
	ScopeConfig *models.AiReviewScopeConfig `json:"scopeConfig"`

	// Source platform filter (optional: github, gitlab, or empty for all)
	SourcePlatform string `json:"sourcePlatform"`

	// Time filter
	TimeAfter string `json:"timeAfter"`
}

// AiReviewTaskData contains shared data for subtasks
type AiReviewTaskData struct {
	Options *AiReviewOptions

	// Compiled regex patterns
	CodeRabbitUsernameRegex   *regexp.Regexp
	CodeRabbitPatternRegex    *regexp.Regexp
	CursorBugbotUsernameRegex *regexp.Regexp
	CursorBugbotPatternRegex  *regexp.Regexp
	AiCommitPatternsRegex     []*regexp.Regexp
	AiPrLabelPatternRegex     *regexp.Regexp
	RiskHighPatternRegex      *regexp.Regexp
	RiskMediumPatternRegex    *regexp.Regexp
	RiskLowPatternRegex       *regexp.Regexp
	BugLinkPatternRegex       *regexp.Regexp
}

// DecodeTaskOptions decodes and validates task options
func DecodeTaskOptions(options map[string]interface{}) (*AiReviewOptions, errors.Error) {
	var op AiReviewOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode aireview options")
	}
	return &op, nil
}

// ValidateTaskOptions validates the task options
func ValidateTaskOptions(op *AiReviewOptions) errors.Error {
	if op.RepoId == "" && op.ProjectName == "" {
		return errors.BadInput.New("either repoId or projectName is required")
	}
	return nil
}

// CompilePatterns compiles all regex patterns from scope config
func CompilePatterns(taskData *AiReviewTaskData) errors.Error {
	config := taskData.Options.ScopeConfig
	if config == nil {
		config = models.GetDefaultScopeConfig()
		taskData.Options.ScopeConfig = config
	}

	var err error

	// CodeRabbit patterns
	if config.CodeRabbitEnabled && config.CodeRabbitUsername != "" {
		taskData.CodeRabbitUsernameRegex, err = regexp.Compile("(?i)" + regexp.QuoteMeta(config.CodeRabbitUsername))
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid codeRabbitUsername pattern")
		}
	}
	if config.CodeRabbitEnabled && config.CodeRabbitPattern != "" {
		taskData.CodeRabbitPatternRegex, err = regexp.Compile(config.CodeRabbitPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid codeRabbitPattern")
		}
	}

	// Cursor Bugbot patterns
	if config.CursorBugbotEnabled && config.CursorBugbotUsername != "" {
		taskData.CursorBugbotUsernameRegex, err = regexp.Compile("(?i)" + regexp.QuoteMeta(config.CursorBugbotUsername))
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid cursorBugbotUsername pattern")
		}
	}
	if config.CursorBugbotEnabled && config.CursorBugbotPattern != "" {
		taskData.CursorBugbotPatternRegex, err = regexp.Compile(config.CursorBugbotPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid cursorBugbotPattern")
		}
	}

	// Risk patterns
	if config.RiskHighPattern != "" {
		taskData.RiskHighPatternRegex, err = regexp.Compile(config.RiskHighPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid riskHighPattern")
		}
	}
	if config.RiskMediumPattern != "" {
		taskData.RiskMediumPatternRegex, err = regexp.Compile(config.RiskMediumPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid riskMediumPattern")
		}
	}
	if config.RiskLowPattern != "" {
		taskData.RiskLowPatternRegex, err = regexp.Compile(config.RiskLowPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid riskLowPattern")
		}
	}

	// Bug link pattern
	if config.BugLinkPattern != "" {
		taskData.BugLinkPatternRegex, err = regexp.Compile(config.BugLinkPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid bugLinkPattern")
		}
	}

	// AI PR label pattern
	if config.AiPrLabelPattern != "" {
		taskData.AiPrLabelPatternRegex, err = regexp.Compile(config.AiPrLabelPattern)
		if err != nil {
			return errors.BadInput.Wrap(err, "invalid aiPrLabelPattern")
		}
	}

	return nil
}
