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

type ComparisonInput struct {
	CommitSha string `json:"commit_sha"`
	ParentSha string `json:"parent_sha"`
	FlagName  string `json:"flag_name"`
}

var CollectComparisonMeta = plugin.SubTaskMeta{
	Name:             "CollectComparison",
	EntryPoint:       CollectComparison,
	EnabledByDefault: true,
	Description:      "Collect comparison data (modified/patch coverage) per flag from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertFlagsMeta},
	DependencyTables: []string{models.CodecovCommit{}.TableName(), models.CodecovFlag{}.TableName()},
}

func CollectComparison(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Extract owner and repo from FullName
	owner, repo, err := parseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Get commits ordered by timestamp, then compare each with previous
	var commits []models.CodecovCommit
	err = db.All(&commits, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName), dal.Orderby("commit_timestamp ASC"))
	if err != nil {
		return err
	}

	// Get all flags
	var flags []models.CodecovFlag
	err = db.All(&flags, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build comparison pairs for each commit × flag combination
	iterator := helper.NewQueueIterator()
	for i := 1; i < len(commits); i++ {
		// Add comparison for each flag
		for _, flag := range flags {
			iterator.Push(&ComparisonInput{
				CommitSha: commits[i].CommitSha,
				ParentSha: commits[i-1].CommitSha,
				FlagName:  flag.FlagName,
			})
		}
		// Also add overall comparison (no flag)
		iterator.Push(&ComparisonInput{
			CommitSha: commits[i].CommitSha,
			ParentSha: commits[i-1].CommitSha,
			FlagName:  "", // Empty flag name for overall comparison
		})
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMPARISONS_TABLE,
		},
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("api/v2/github/%s/repos/%s/compare", owner, repo),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*ComparisonInput)
			query := url.Values{}
			query.Set("base", input.ParentSha)
			query.Set("head", input.CommitSha)
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
			// Handle 404 for comparisons that don't exist
			if res.StatusCode == http.StatusNotFound {
				return nil // Skip this comparison, continue
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
