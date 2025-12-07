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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

var CollectCommitsMeta = plugin.SubTaskMeta{
	Name:             "CollectCommits",
	EntryPoint:       CollectCommits,
	EnabledByDefault: true,
	Description:      "Collect commits data from Codecov API for main/master branch",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

func CollectCommits(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	logger := taskCtx.GetLogger()

	// Extract owner and repo from FullName
	owner, repo, err := parseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Determine branch (main or master) from scope config or default to main
	branch := "main"
	if data.Options.ScopeConfig != nil {
		// Could add branch config here if needed
	}

	logger.Info("[Codecov] Collecting ALL commits for %s/%s branch=%s", owner, repo, branch)

	apiCollector, err := helper.NewStatefulApiCollector(helper.RawDataSubTaskArgs{
		Ctx: taskCtx,
		Params: CodecovApiParams{
			ConnectionId: data.Options.ConnectionId,
			Name:         data.Options.FullName,
		},
		Table: RAW_COMMITS_TABLE,
	})
	if err != nil {
		return err
	}

	err = apiCollector.InitCollector(helper.ApiCollectorArgs{
		ApiClient:   data.ApiClient,
		Incremental: true, // ALWAYS preserve historical data
		PageSize:    100,
		UrlTemplate: fmt.Sprintf("api/v2/github/%s/repos/%s/commits", owner, repo),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("branch", branch)
			query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
			query.Set("page_size", fmt.Sprintf("%v", reqData.Pager.Size))
			// NOTE: Removed updated_at filter to always collect ALL historical commits
			// The stateful collector and incremental mode will handle deduplication
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var response struct {
				Count      int               `json:"count"`
				Next       *string           `json:"next"`
				Previous   *string           `json:"previous"`
				Results    []json.RawMessage `json:"results"`
				TotalPages int               `json:"total_pages"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
		GetTotalPages: func(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
			// Parse Codecov API pagination response
			var response struct {
				Count      int     `json:"count"`
				Next       *string `json:"next"`
				TotalPages int     `json:"total_pages"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return 0, nil // Continue until no results
			}
			// If total_pages is provided, use it
			if response.TotalPages > 0 {
				return response.TotalPages, nil
			}
			// Otherwise calculate from count
			if response.Count > 0 && args.PageSize > 0 {
				totalPages := (response.Count + args.PageSize - 1) / args.PageSize
				return totalPages, nil
			}
			return 0, nil // 0 means unknown, continue until no results
		},
	})
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}
