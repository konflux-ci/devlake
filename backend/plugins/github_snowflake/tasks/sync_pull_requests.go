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

var SyncPullRequestsMeta = plugin.SubTaskMeta{
	Name:             "syncPullRequests",
	EntryPoint:       SyncPullRequests,
	EnabledByDefault: true,
	Description:      "Sync GitHub pull requests from Snowflake into _tool_github_pull_requests",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE_REVIEW, plugin.DOMAIN_TYPE_CROSS},
}

func SyncPullRequests(subtaskCtx plugin.SubTaskContext) errors.Error {
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

	query, args := buildPullRequestsQuery(repoId, timeAfter)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for pull requests")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			githubId       int64
			repositoryId   int64
			headRepoId     *int64
			number         int64
			state          string
			title          *string
			body           *string
			createdAt      time.Time
			updatedAt      time.Time
			closedAt       *time.Time
			isDraft        *bool
			mergeCommitSha *string
			headRef        *string
			baseRef        *string
			headSha        *string
			baseSha        *string
			authorId       *int64
			authorName     *string
			mergedAt       *time.Time
			mergedById     *int64
			mergedByName   *string
		)
		if scanErr := rows.Scan(
			&githubId, &repositoryId, &headRepoId, &number, &state, &title, &body,
			&createdAt, &updatedAt, &closedAt, &isDraft, &mergeCommitSha,
			&headRef, &baseRef, &headSha, &baseSha,
			&authorId, &authorName, &mergedAt, &mergedById, &mergedByName,
		); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan pull request row")
		}

		draft := false
		if isDraft != nil {
			draft = *isDraft
		}
		merged := mergedAt != nil

		pr := &githubmodels.GithubPullRequest{
			ConnectionId:    connectionId,
			GithubId:        int(githubId),
			RepoId:          int(repositoryId),
			HeadRepoId:      nullInt(headRepoId),
			Number:          int(number),
			State:           state,
			Title:           nullStr(title),
			Body:            nullStr(body),
			GithubCreatedAt: createdAt,
			GithubUpdatedAt: updatedAt,
			ClosedAt:        closedAt,
			IsDraft:         draft,
			Merged:          merged,
			MergedAt:        mergedAt,
			MergeCommitSha:  nullStr(mergeCommitSha),
			HeadRef:         nullStr(headRef),
			BaseRef:         nullStr(baseRef),
			HeadCommitSha:   nullStr(headSha),
			BaseCommitSha:   nullStr(baseSha),
			AuthorId:        nullInt(authorId),
			AuthorName:      nullStr(authorName),
			MergedById:      nullInt(mergedById),
			MergedByName:    nullStr(mergedByName),
			Url:             derivePullRequestURL(fullName, int(number)),
			NoPKModel:       common.NewNoPKModel(),
		}
		if dbErr := db.CreateOrUpdate(pr); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating pull request rows")
	}

	logger.Info("synced %d pull requests from Snowflake", count)
	return nil
}

func buildPullRequestsQuery(repoId int, timeAfter *time.Time) (string, []interface{}) {
	query := `
SELECT
    pr.ID              AS github_id,
    i.REPOSITORY_ID    AS repo_id,
    pr.HEAD_REPO_ID    AS head_repo_id,
    i.NUMBER           AS number,
    i.STATE            AS state,
    i.TITLE            AS title,
    i.BODY             AS body,
    COALESCE(pr.CREATED_AT, i.CREATED_AT) AS created_at,
    pr.UPDATED_AT      AS updated_at,
    COALESCE(pr.CLOSED_AT, i.CLOSED_AT) AS closed_at,
    pr.DRAFT           AS is_draft,
    pr.MERGE_COMMIT_SHA,
    pr.HEAD_REF,
    pr.BASE_REF,
    pr.HEAD_SHA,
    pr.BASE_SHA,
    i.USER_ID          AS author_id,
    u.LOGIN            AS author_name,
    im.MERGED_AT,
    im.ACTOR_ID        AS merged_by_id,
    mu.LOGIN           AS merged_by_name
FROM PULL_REQUEST pr
JOIN ISSUE i
  ON i.ID = pr.ISSUE_ID
LEFT JOIN "USER" u
  ON u.ID = i.USER_ID
LEFT JOIN ISSUE_MERGED im
  ON im.ISSUE_ID = i.ID
LEFT JOIN "USER" mu
  ON mu.ID = im.ACTOR_ID
WHERE i.REPOSITORY_ID = ?
`
	args := []interface{}{repoId}
	if timeAfter != nil {
		query += "  AND pr.UPDATED_AT > ?\n"
		args = append(args, *timeAfter)
	}
	return query, args
}
