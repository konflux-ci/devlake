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
	"github.com/apache/incubator-devlake/core/plugin"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

var SyncPrReviewsMeta = plugin.SubTaskMeta{
	Name:             "syncPrReviews",
	EntryPoint:       SyncPrReviews,
	EnabledByDefault: true,
	Description:      "Sync GitHub PR reviews from Snowflake into _tool_github_pull_request_reviews",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW},
}

func SyncPrReviews(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*GithubSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	repoId := data.Options.GithubId
	fullName := data.Options.Name

	var timeAfter *time.Time
	syncPolicy := subtaskCtx.TaskContext().SyncPolicy()
	if syncPolicy != nil {
		timeAfter = syncPolicy.TimeAfter
	}

	query, args := buildPrReviewsQuery(repoId, timeAfter)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for PR reviews")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			githubId      int64
			pullRequestId int64
			body          *string
			state         *string
			commitSha     *string
			submittedAt   *time.Time
			authorUserId  *int64
			authorLogin   *string
		)
		if scanErr := rows.Scan(
			&githubId, &pullRequestId, &body, &state, &commitSha,
			&submittedAt, &authorUserId, &authorLogin,
		); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan PR review row")
		}

		review := &githubmodels.GithubPrReview{
			ConnectionId:   connectionId,
			GithubId:       int(githubId),
			PullRequestId:  int(pullRequestId),
			Body:           nullStr(body),
			State:          nullStr(state),
			CommitSha:      nullStr(commitSha),
			GithubSubmitAt: submittedAt,
			AuthorUserId:   nullInt(authorUserId),
			AuthorUsername: nullStr(authorLogin),
			NoPKModel:      toolLayerNoPKModel(RAW_PR_REVIEW_TABLE, connectionId, fullName),
		}
		if dbErr := db.CreateOrUpdate(review); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating PR review rows")
	}

	logger.Info("synced %d PR reviews from Snowflake", count)
	return nil
}

func buildPrReviewsQuery(repoId int, timeAfter *time.Time) (string, []interface{}) {
	query := `
SELECT
    r.ID               AS github_id,
    r.PULL_REQUEST_ID  AS pull_request_id,
    r.BODY,
    r.STATE,
    r.COMMIT_SHA,
    r.SUBMITTED_AT,
    r.USER_ID          AS author_user_id,
    u.LOGIN            AS author_username
FROM PULL_REQUEST_REVIEW r
JOIN PULL_REQUEST pr
  ON pr.ID = r.PULL_REQUEST_ID
JOIN ISSUE i
  ON i.ID = pr.ISSUE_ID
LEFT JOIN "USER" u
  ON u.ID = r.USER_ID
WHERE i.REPOSITORY_ID = ?
`
	args := []interface{}{repoId}
	if timeAfter != nil {
		query += "  AND r.SUBMITTED_AT > ?\n"
		args = append(args, *timeAfter)
	}
	return query, args
}
