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

var CollectTracesMeta = plugin.SubTaskMeta{
	Name:             "CollectTraces",
	EntryPoint:       CollectTraces,
	EnabledByDefault: true,
	Description:      "Collect traces from Langfuse API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

func CollectTraces(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LangfuseTaskData)

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LangfuseApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_TRACES_TABLE,
		},
		ApiClient:   data.ApiClient,
		UrlTemplate: "api/public/traces",
		PageSize:    100,
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			query := url.Values{}
			query.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))
			query.Set("limit", fmt.Sprintf("%d", reqData.Pager.Size))
			syncPolicy := taskCtx.TaskContext().SyncPolicy()
			if syncPolicy != nil && syncPolicy.TimeAfter != nil {
				query.Set("fromTimestamp", syncPolicy.TimeAfter.Format("2006-01-02T15:04:05Z"))
			}
			return query, nil
		},
		GetTotalPages: func(res *http.Response, args *helper.ApiCollectorArgs) (int, errors.Error) {
			var body struct {
				Meta struct {
					TotalPages int `json:"totalPages"`
				} `json:"meta"`
			}
			err := helper.UnmarshalResponse(res, &body)
			if err != nil {
				return 0, err
			}
			return body.Meta.TotalPages, nil
		},
		ResponseParser: func(res *http.Response) ([]json.RawMessage, errors.Error) {
			var body struct {
				Data []json.RawMessage `json:"data"`
			}
			err := helper.UnmarshalResponse(res, &body)
			if err != nil {
				return nil, err
			}
			return body.Data, nil
		},
	})
	if err != nil {
		return err
	}
	return collector.Execute()
}
