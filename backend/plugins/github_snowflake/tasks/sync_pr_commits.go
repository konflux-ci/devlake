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

var SyncPrCommitsMeta = plugin.SubTaskMeta{
	Name:             "syncPrCommits",
	EntryPoint:       SyncPrCommits,
	EnabledByDefault: true,
	Description:      "Sync GitHub PR commits from Snowflake into _tool_github_pull_request_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW, plugin.DOMAIN_TYPE_CROSS},
}

func SyncPrCommits(subtaskCtx plugin.SubTaskContext) errors.Error {
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

	query, args := buildPrCommitsQuery(repoId, timeAfter)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for PR commits")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			commitSha     string
			pullRequestId int64
			authorName    *string
			authorEmail   *string
			authoredDate  time.Time
		)
		if scanErr := rows.Scan(&commitSha, &pullRequestId, &authorName, &authorEmail, &authoredDate); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan PR commit row")
		}

		prCommit := &githubmodels.GithubPrCommit{
			ConnectionId:       connectionId,
			CommitSha:          commitSha,
			PullRequestId:      int(pullRequestId),
			CommitAuthorName:   nullStr(authorName),
			CommitAuthorEmail:  nullStr(authorEmail),
			CommitAuthoredDate: authoredDate,
			NoPKModel:          common.NewNoPKModel(),
		}
		if dbErr := db.CreateOrUpdate(prCommit); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating PR commit rows")
	}

	logger.Info("synced %d PR commits from Snowflake", count)
	return nil
}

func buildPrCommitsQuery(repoId int, timeAfter *time.Time) (string, []interface{}) {
	// INNER JOIN PULL_REQUEST drops orphaned historical COMMIT_PULL_REQUEST links.
	query := `
SELECT
    cpr.COMMIT_SHA,
    cpr.PULL_REQUEST_ID,
    c.AUTHOR_NAME,
    c.AUTHOR_EMAIL,
    c.AUTHOR_DATE
FROM COMMIT_PULL_REQUEST cpr
JOIN COMMIT c
  ON c.SHA = cpr.COMMIT_SHA
JOIN PULL_REQUEST pr
  ON pr.ID = cpr.PULL_REQUEST_ID
JOIN ISSUE i
  ON i.ID = pr.ISSUE_ID
WHERE i.REPOSITORY_ID = ?
`
	args := []interface{}{repoId}
	if timeAfter != nil {
		query += "  AND c.AUTHOR_DATE > ?\n"
		args = append(args, *timeAfter)
	}
	return query, args
}
