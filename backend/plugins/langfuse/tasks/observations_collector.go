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
	"reflect"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/langfuse/models"
)

var CollectObservationsMeta = plugin.SubTaskMeta{
	Name:             "CollectObservations",
	EntryPoint:       CollectObservations,
	EnabledByDefault: true,
	Description:      "Collect observations from Langfuse API per trace",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{models.LangfuseTrace{}.TableName()},
}

type TraceInput struct {
	TraceId string `json:"traceId" gorm:"column:trace_id"`
}

func CollectObservations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LangfuseTaskData)
	db := taskCtx.GetDal()

	cursor, err := db.Cursor(
		dal.Select("trace_id"),
		dal.From(models.LangfuseTrace{}.TableName()),
		dal.Where("connection_id = ? AND project_id = ?",
			data.Options.ConnectionId, data.Options.ProjectId),
	)
	if err != nil {
		return err
	}

	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(TraceInput{}))
	if err != nil {
		return err
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LangfuseApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_OBSERVATIONS_TABLE,
		},
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: "api/public/observations",
		PageSize:    100,
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*TraceInput)
			query := url.Values{}
			query.Set("traceId", input.TraceId)
			query.Set("page", fmt.Sprintf("%d", reqData.Pager.Page))
			query.Set("limit", fmt.Sprintf("%d", reqData.Pager.Size))
			return query, nil
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
