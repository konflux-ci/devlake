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
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

type CommitInput struct {
	CommitSha string `json:"commit_sha"`
}

var CollectCommitTotalsMeta = plugin.SubTaskMeta{
	Name:             "CollectCommitTotals",
	EntryPoint:       CollectCommitTotals,
	EnabledByDefault: true,
	Description:      "Collect commit totals (overall coverage) from Codecov API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{models.CodecovCommit{}.TableName()},
}

func CollectCommitTotals(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Extract owner and repo from FullName
	owner, repo, err := parseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Create iterator from extracted commits (tool layer)
	clauses := []dal.Clause{
		dal.Select("commit_sha AS commit_sha"),
		dal.From(&models.CodecovCommit{}),
		dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()

	iterator, err := helper.NewDalCursorIterator(db, cursor, reflect.TypeOf(CommitInput{}))
	if err != nil {
		return err
	}

	collector, err := helper.NewApiCollector(helper.ApiCollectorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_TOTALS_TABLE,
		},
		ApiClient:   data.ApiClient,
		Input:       iterator,
		UrlTemplate: fmt.Sprintf("api/v2/github/%s/repos/%s/totals/", owner, repo),
		Query: func(reqData *helper.RequestData) (url.Values, errors.Error) {
			input := reqData.Input.(*CommitInput)
			query := url.Values{}
			query.Set("sha", input.CommitSha)
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
			// Handle 404 for commits that don't have coverage data
			if res.StatusCode == http.StatusNotFound {
				return nil // Skip this commit, continue
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
