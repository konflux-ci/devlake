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
	logger := taskCtx.GetLogger()

	// Extract owner and repo from FullName
	owner, repo, err := parseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	// Get existing commit coverages to skip already collected data (OPTIMIZATION)
	var existingCoverages []models.CodecovCommitCoverage
	err = db.All(&existingCoverages, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build a set of already collected commit SHAs
	collectedSet := make(map[string]bool)
	for _, cov := range existingCoverages {
		collectedSet[cov.CommitSha] = true
	}

	// Get all commits and filter to only new ones
	var allCommits []models.CodecovCommit
	err = db.All(&allCommits, dal.Where("connection_id = ? AND repo_id = ?", data.Options.ConnectionId, data.Options.FullName))
	if err != nil {
		return err
	}

	// Build iterator with only NEW commits
	iterator := helper.NewQueueIterator()
	skippedCount := 0
	addedCount := 0
	for _, commit := range allCommits {
		if !collectedSet[commit.CommitSha] {
			iterator.Push(&CommitInput{
				CommitSha: commit.CommitSha,
			})
			addedCount++
		} else {
			skippedCount++
		}
	}

	logger.Info("[Codecov] CommitTotals: Skipped %d already collected, collecting %d new", skippedCount, addedCount)

	// If nothing new to collect, return early
	if addedCount == 0 {
		logger.Info("[Codecov] CommitTotals: All data already collected, skipping API calls")
		return nil
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
		Incremental: true, // ALWAYS preserve historical data
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
			// Safety check: if status is 404 or 500+, return empty array to skip
			if res.StatusCode == http.StatusNotFound || res.StatusCode >= http.StatusInternalServerError {
				return []json.RawMessage{}, nil
			}
			var body json.RawMessage
			err := helper.UnmarshalResponse(res, &body)
			if err != nil {
				return nil, err
			}
			return []json.RawMessage{body}, nil
		},
		AfterResponse: func(res *http.Response) errors.Error {
			if res.StatusCode == http.StatusUnauthorized {
				return errors.Unauthorized.New("authentication failed, please check your AccessToken")
			}
			// Skip 404 (no coverage) and 500 (server error) without retrying
			if res.StatusCode == http.StatusNotFound || res.StatusCode >= http.StatusInternalServerError {
				return helper.ErrIgnoreAndContinue
			}
			return nil
		},
	})

	if err != nil {
		return err
	}

	return collector.Execute()
}
