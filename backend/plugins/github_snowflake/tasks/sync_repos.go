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

var SyncReposMeta = plugin.SubTaskMeta{
	Name:             "syncRepos",
	EntryPoint:       SyncRepos,
	EnabledByDefault: true,
	Description:      "Sync GitHub repositories from Snowflake into _tool_github_repos",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_CODE_REVIEW, plugin.DOMAIN_TYPE_CROSS},
}

func SyncRepos(subtaskCtx plugin.SubTaskContext) errors.Error {
	data := subtaskCtx.GetData().(*GithubSnowflakeTaskData)
	db := subtaskCtx.GetDal()
	logger := subtaskCtx.GetLogger()

	connectionId := data.Options.ConnectionId
	githubId := data.Options.GithubId

	query, args := buildReposQuery(githubId)
	rows, goErr := data.SnowflakeDB.QueryContext(subtaskCtx.GetContext(), query, args...)
	if goErr != nil {
		return errors.Default.Wrap(goErr, "failed to query Snowflake for repositories")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			id          int64
			name        string
			fullName    string
			description *string
			language    *string
			ownerId     *int64
			createdAt   *time.Time
		)
		if scanErr := rows.Scan(&id, &name, &fullName, &description, &language, &ownerId, &createdAt); scanErr != nil {
			return errors.Default.Wrap(scanErr, "failed to scan repository row")
		}

		repo := &githubmodels.GithubRepo{
			Scope: common.Scope{
				ConnectionId:  connectionId,
				ScopeConfigId: data.Options.ScopeConfigId,
			},
			GithubId:    int(id),
			Name:        name,
			FullName:    fullName,
			Description: nullStr(description),
			Language:    nullStr(language),
			OwnerId:     nullInt(ownerId),
			HTMLUrl:     deriveRepoHTMLUrl(fullName),
			CloneUrl:    deriveRepoCloneUrl(fullName),
			CreatedDate: createdAt,
		}
		if dbErr := db.CreateOrUpdate(repo); dbErr != nil {
			return dbErr
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return errors.Default.Wrap(err, "error iterating repository rows")
	}

	logger.Info("synced %d repositories from Snowflake", count)
	return nil
}

func buildReposQuery(githubId int) (string, []interface{}) {
	query := `
SELECT
    ID,
    NAME,
    FULL_NAME,
    DESCRIPTION,
    LANGUAGE,
    OWNER_ID,
    CREATED_AT
FROM REPOSITORY
WHERE ID = ?
`
	return query, []interface{}{githubId}
}
