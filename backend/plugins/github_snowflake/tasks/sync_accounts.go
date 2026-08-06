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
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

var SyncAccountsMeta = plugin.SubTaskMeta{
	Name:             "syncAccounts",
	EntryPoint:       SyncAccounts,
	EnabledByDefault: true,
	Description:      "Sync GitHub users referenced by repo PRs/reviews from Snowflake into _tool_github_accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
}

func SyncAccounts(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*GithubSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	repoId := data.Options.GithubId
	fullName := data.Options.Name

	query, args := buildAccountsQuery(repoId)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for accounts")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			id    int64
			login *string
			name  *string
			email *string
		)
		if scanErr := rows.Scan(&id, &login, &name, &email); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan account row")
		}

		loginStr := nullStr(login)
		account := &githubmodels.GithubAccount{
			ConnectionId: connectionId,
			Id:           int(id),
			Login:        loginStr,
			Name:         nullStr(name),
			Email:        nullStr(email),
			NoPKModel:    toolLayerNoPKModel(RAW_ACCOUNT_TABLE, connectionId, fullName),
		}
		if loginStr != "" {
			account.HtmlUrl = "https://github.com/" + loginStr
			account.Url = "https://api.github.com/users/" + loginStr
		}
		if dbErr := db.CreateOrUpdate(account); dbErr != nil {
			return dbErr
		}

		repoAccount := &githubmodels.GithubRepoAccount{
			ConnectionId: connectionId,
			AccountId:    int(id),
			RepoGithubId: repoId,
			Login:        loginStr,
			NoPKModel:    toolLayerNoPKModel(RAW_ACCOUNT_TABLE, connectionId, fullName),
		}
		if dbErr := db.CreateOrUpdate(repoAccount); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating account rows")
	}

	logger.Info("synced %d accounts from Snowflake", count)
	return nil
}

func buildAccountsQuery(repoId int) (string, []interface{}) {
	// Distinct users referenced as PR authors, mergers, review authors, or requested reviewers.
	query := `
WITH repo_users AS (
    SELECT i.USER_ID AS user_id
    FROM PULL_REQUEST pr
    JOIN ISSUE i ON i.ID = pr.ISSUE_ID
    WHERE i.REPOSITORY_ID = ?
      AND i.USER_ID IS NOT NULL

    UNION

    SELECT im.ACTOR_ID AS user_id
    FROM PULL_REQUEST pr
    JOIN ISSUE i ON i.ID = pr.ISSUE_ID
    JOIN ISSUE_MERGED im ON im.ISSUE_ID = i.ID
    WHERE i.REPOSITORY_ID = ?
      AND im.ACTOR_ID IS NOT NULL

    UNION

    SELECT r.USER_ID AS user_id
    FROM PULL_REQUEST_REVIEW r
    JOIN PULL_REQUEST pr ON pr.ID = r.PULL_REQUEST_ID
    JOIN ISSUE i ON i.ID = pr.ISSUE_ID
    WHERE i.REPOSITORY_ID = ?
      AND r.USER_ID IS NOT NULL

    UNION

    SELECT h.REQUESTED_ID AS user_id
    FROM REQUESTED_REVIEWER_HISTORY h
    JOIN PULL_REQUEST pr ON pr.ID = h.PULL_REQUEST_ID
    JOIN ISSUE i ON i.ID = pr.ISSUE_ID
    WHERE i.REPOSITORY_ID = ?
      AND LOWER(h.REQUESTED_REVIEWER_TYPE) = 'user'
      AND h.REQUESTED_ID IS NOT NULL
)
SELECT
    u.ID,
    u.LOGIN,
    u.NAME,
    ue.EMAIL
FROM "USER" u
JOIN repo_users ru ON ru.user_id = u.ID
LEFT JOIN USER_EMAIL ue
  ON ue.USER_ID = u.ID
QUALIFY ROW_NUMBER() OVER (
    PARTITION BY u.ID
    ORDER BY ue.EMAIL NULLS LAST
) = 1
`
	return query, []interface{}{repoId, repoId, repoId, repoId}
}
