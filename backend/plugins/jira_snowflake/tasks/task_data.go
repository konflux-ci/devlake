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

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	jiramodels "github.com/apache/incubator-devlake/plugins/jira/models"
)

// JiraSnowflakeOptions contains all per-pipeline task options.
type JiraSnowflakeOptions struct {
	ConnectionId uint64 `json:"connectionId"  mapstructure:"connectionId"`
	BoardId      uint64 `json:"boardId"       mapstructure:"boardId"`
	// ProjectKeys lists all Jira project keys that belong to this board,
	// e.g. ["KONFLUX", "HELM"]. Required because a board may span multiple projects.
	ProjectKeys   []string                    `json:"projectKeys"   mapstructure:"projectKeys"`
	ScopeConfigId uint64                      `json:"scopeConfigId" mapstructure:"scopeConfigId"`
	ScopeConfig   *jiramodels.JiraScopeConfig `json:"scopeConfig"   mapstructure:"scopeConfig"`
}

// JiraSnowflakeTaskData is passed to every subtask via taskCtx.GetData().
type JiraSnowflakeTaskData struct {
	Options     *JiraSnowflakeOptions
	SnowflakeDB *sql.DB
}

// JiraApiParams mirrors jira/models.JiraApiParams so that RawDataSubTaskArgs
// produces the same _raw_data_params format for state management.
type JiraApiParams struct {
	ConnectionId uint64
	BoardId      uint64
}

// DecodeAndValidateTaskOptions decodes and validates options for the task.
func DecodeAndValidateTaskOptions(options map[string]interface{}) (*JiraSnowflakeOptions, errors.Error) {
	var op JiraSnowflakeOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid connectionId: %d", op.ConnectionId))
	}
	if op.BoardId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid boardId: %d", op.BoardId))
	}
	if len(op.ProjectKeys) == 0 {
		return nil, errors.BadInput.New("projectKeys must not be empty")
	}
	return &op, nil
}
