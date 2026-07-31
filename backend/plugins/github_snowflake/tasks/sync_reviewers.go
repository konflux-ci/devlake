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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

var SyncReviewersMeta = plugin.SubTaskMeta{
	Name:             "syncReviewers",
	EntryPoint:       SyncReviewers,
	EnabledByDefault: true,
	Description:      "Sync GitHub requested reviewers from Snowflake into _tool_github_reviewers",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

func SyncReviewers(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*GithubSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	repoId := data.Options.GithubId

	var timeAfter *time.Time
	syncPolicy := subtaskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil {
		timeAfter = syncPolicy.TimeAfter
	}

	query, args := buildReviewersQuery(repoId, timeAfter)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for reviewers")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			reviewerId    int64
			pullRequestId int64
			username      *string
			name          *string
		)
		if scanErr := rows.Scan(&reviewerId, &pullRequestId, &username, &name); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan reviewer row")
		}

		reviewer := &githubmodels.GithubReviewer{
			ConnectionId:  connectionId,
			ReviewerId:    int(reviewerId),
			PullRequestId: int(pullRequestId),
			Username:      nullStr(username),
			Name:          nullStr(name),
			NoPKModel:     common.NewNoPKModel(),
		}
		if dbErr := db.CreateOrUpdate(reviewer); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating reviewer rows")
	}

	logger.Info("synced %d reviewers from Snowflake", count)
	return nil
}

func buildReviewersQuery(repoId int, timeAfter *time.Time) (string, []interface{}) {
	// Latest non-removed user request per (pull_request_id, requested_id).
	query := `
SELECT
    h.REQUESTED_ID     AS reviewer_id,
    h.PULL_REQUEST_ID  AS pull_request_id,
    u.LOGIN            AS username,
    u.NAME             AS name
FROM REQUESTED_REVIEWER_HISTORY h
JOIN PULL_REQUEST pr
  ON pr.ID = h.PULL_REQUEST_ID
JOIN ISSUE i
  ON i.ID = pr.ISSUE_ID
LEFT JOIN "USER" u
  ON u.ID = h.REQUESTED_ID
WHERE i.REPOSITORY_ID = ?
  AND LOWER(h.REQUESTED_REVIEWER_TYPE) = 'user'
  AND (h.REMOVED IS NULL OR h.REMOVED = FALSE)
`
	args := []interface{}{repoId}
	if timeAfter != nil {
		query += "  AND h.CREATED_AT > ?\n"
		args = append(args, *timeAfter)
	}
	query += `
QUALIFY ROW_NUMBER() OVER (
    PARTITION BY h.PULL_REQUEST_ID, h.REQUESTED_ID
    ORDER BY h.CREATED_AT DESC NULLS LAST
) = 1
`
	return query, args
}
