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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

type FileCoverageInput struct {
	CommitSha string `json:"commit_sha"`
	FlagName  string `json:"flag_name"`
	Branch    string `json:"branch"`
}

var CollectFileCoverageMeta = plugin.SubTaskMeta{
	Name:             "CollectFileCoverage",
	EntryPoint:       CollectFileCoverage,
	EnabledByDefault: true,
	Description:      "Collect file-level coverage data per flag per commit from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertFlagsMeta},
	DependencyTables: []string{models.CodecovCommit{}.TableName(), models.CodecovFlag{}.TableName()},
}

func CollectFileCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Extract owner and repo from FullName
	owner, repo, err := parseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Get commits
	var commits []models.CodecovCommit
	err = db.All(&commits, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName), dal.Orderby("commit_timestamp DESC"))
	if err != nil {
		return err
	}

	// Get all flags
	var flags []models.CodecovFlag
	err = db.All(&flags, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build iterator with all commit × flag combinations
	iterator := helper.NewQueueIterator()
	for _, commit := range commits {
		for _, flag := range flags {
			iterator.Push(&FileCoverageInput{
				CommitSha: commit.CommitSha,
				FlagName:  flag.FlagName,
				Branch:    commit.Branch,
			})
		}
		// Also add overall coverage (no flag)
		iterator.Push(&FileCoverageInput{
			CommitSha: commit.CommitSha,
			FlagName:  "", // Empty flag name for overall coverage
			Branch:    commit.Branch,
		})
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FILE_COVERAGES_TABLE,
		},
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("api/v2/github/%s/repos/%s/report", owner, repo),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*FileCoverageInput)
			query := url.Values{}
			query.Set("sha", input.CommitSha)
			query.Set("branch", input.Branch)
			if input.FlagName != "" {
				query.Set("flag", input.FlagName)
			}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var body json.RawMessage
			err := helper.UnmarshalResponse(res, &body)
			if err != nil {
				return nil, err
			}
			return []json.RawMessage{body}, nil
		},
		AfterResponse: func(res *http.Response) errors.Error {
			// Handle 404 for commits/flags that don't have coverage data
			if res.StatusCode == http.StatusNotFound {
				return nil // Skip this combination, continue
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}

