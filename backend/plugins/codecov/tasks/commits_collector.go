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
		PageSize:    100,
		UrlTemplate: fmt.Sprintf("api/v2/github/%s/repos/%s/commits", owner, repo),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("branch", branch)
			query.Set("page", fmt.Sprintf("%v", reqData.Pager.Page))
			query.Set("page_size", fmt.Sprintf("%v", reqData.Pager.Size))
			if apiCollector.GetSince() != nil {
				query.Set("updated_at", fmt.Sprintf(">%s", apiCollector.GetSince().Format("2006-01-02T15:04:05Z")))
			}
			return query, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var response struct {
				Results []json.RawMessage `json:"results"`
			}
			err := helper.UnmarshalResponse(res, &response)
			if err != nil {
				return nil, err
			}
			return response.Results, nil
		},
		GetTotalPages: func(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
			// Codecov API may return total count in headers or response
			// For now, we'll use pagination until no more results
			return 0, nil // 0 means unknown, collector will continue until no results
		},
	})
	if err != nil {
		return err
	}

	return apiCollector.Execute()
}
