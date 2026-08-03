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
	"database/sql"
	"fmt"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

// Raw table name constants used for StatefulDataConverter state params
// (same strings as plugins/github/tasks so state keys stay compatible).
const (
	RAW_PULL_REQUEST_TABLE = "github_api_pull_requests"
	RAW_PR_COMMIT_TABLE    = "github_api_pull_request_commits"
	RAW_PR_REVIEW_TABLE    = "github_api_pull_request_reviews"
	RAW_ACCOUNT_TABLE      = "github_api_accounts"
)

// GithubSnowflakeOptions contains all per-pipeline task options.
type GithubSnowflakeOptions struct {
	ConnectionId  uint64                          `json:"connectionId"  mapstructure:"connectionId"`
	GithubId      int                             `json:"githubId"      mapstructure:"githubId"`
	Name          string                          `json:"name"          mapstructure:"name"`
	FullName      string                          `json:"fullName"      mapstructure:"fullName"`
	ScopeConfigId uint64                          `json:"scopeConfigId" mapstructure:"scopeConfigId"`
	ScopeConfig   *githubmodels.GithubScopeConfig `json:"scopeConfig"   mapstructure:"scopeConfig"`
}

// GithubSnowflakeTaskData is passed to every subtask via taskCtx.GetData().
type GithubSnowflakeTaskData struct {
	Options     *GithubSnowflakeOptions
	SnowflakeDB *sql.DB
}

// GithubApiParams mirrors github/models.GithubApiParams so that RawDataSubTaskArgs
// produces the same _raw_data_params format for state management.
type GithubApiParams struct {
	ConnectionId uint64
	Name         string
}

// DecodeAndValidateTaskOptions decodes and validates options for the task.
func DecodeAndValidateTaskOptions(options map[string]interface{}) (*GithubSnowflakeOptions, errors.Error) {
	var op GithubSnowflakeOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid connectionId: %d", op.ConnectionId))
	}
	if op.GithubId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid githubId: %d", op.GithubId))
	}
	if op.Name == "" {
		op.Name = op.FullName
	}
	if op.FullName == "" {
		op.FullName = op.Name
	}
	if op.Name == "" {
		return nil, errors.BadInput.New("name (owner/repo full name) must not be empty")
	}
	if err := validateOwnerRepo(op.Name); err != nil {
		return nil, err
	}
	return &op, nil
}

// validateOwnerRepo checks that name is in "owner/repo" format.
func validateOwnerRepo(name string) errors.Error {
	if strings.Count(name, "/") != 1 {
		return errors.BadInput.New("name must be in owner/repo format")
	}
	owner, repo, _ := strings.Cut(name, "/")
	if owner == "" || repo == "" {
		return errors.BadInput.New("name must be in owner/repo format")
	}
	return nil
}

// RepoShortName returns the short name from owner/repo full name.
func RepoShortName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) == 0 {
		return fullName
	}
	return parts[len(parts)-1]
}
