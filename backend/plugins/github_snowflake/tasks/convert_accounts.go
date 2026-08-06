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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "convertAccounts",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert tool layer github_accounts into domain accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		githubmodels.GithubAccount{}.TableName(),
		githubmodels.GithubRepoAccount{}.TableName(),
	},
	ProductTables: []string{crossdomain.Account{}.TableName()},
}

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubSnowflakeTaskData)

	accountIdGen := didgen.NewDomainIdGenerator(&githubmodels.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[githubmodels.GithubAccount]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_ACCOUNT_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.Select("_tool_github_accounts.*"),
				dal.From(&githubmodels.GithubAccount{}),
				dal.Where(
					"repo_github_id = ? and _tool_github_accounts.connection_id=?",
					data.Options.GithubId,
					data.Options.ConnectionId,
				),
				dal.Join(`left join _tool_github_repo_accounts gra on (
					_tool_github_accounts.connection_id = gra.connection_id
					AND _tool_github_accounts.id = gra.account_id
				)`),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_accounts.updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubUser *githubmodels.GithubAccount) ([]interface{}, errors.Error) {
			// Orgs are not synced from Snowflake in MVP — leave Organization empty.
			domainUser := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{Id: accountIdGen.Generate(data.Options.ConnectionId, githubUser.Id)},
				Email:        githubUser.Email,
				FullName:     githubUser.Name,
				UserName:     githubUser.Login,
				AvatarUrl:    githubUser.AvatarUrl,
			}
			return []interface{}{domainUser}, nil
		},
	})
	if err != nil {
		return err
	}

	err = converter.Execute()
	if err != nil {
		return err
	}

	return convertOrphanedRepoAccounts(taskCtx, db, data, accountIdGen)
}

func convertOrphanedRepoAccounts(taskCtx plugin.SubTaskContext, db dal.Dal, data *GithubSnowflakeTaskData, accountIdGen *didgen.DomainIdGenerator) errors.Error {
	cursor, err := db.Cursor(
		dal.Select("_tool_github_repo_accounts.*"),
		dal.From(&githubmodels.GithubRepoAccount{}),
		dal.Where(
			"_tool_github_repo_accounts.repo_github_id = ? AND _tool_github_repo_accounts.connection_id = ?",
			data.Options.GithubId,
			data.Options.ConnectionId,
		),
		dal.Join(`LEFT JOIN _tool_github_accounts ga ON (
			_tool_github_repo_accounts.connection_id = ga.connection_id
			AND _tool_github_repo_accounts.account_id = ga.id
		)`),
		dal.Where("ga.id IS NULL"),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	logger := taskCtx.GetLogger()
	for cursor.Next() {
		var orphan githubmodels.GithubRepoAccount
		err = db.Fetch(cursor, &orphan)
		if err != nil {
			return err
		}
		logger.Info("creating domain account for orphaned repo account: login=%s, id=%d", orphan.Login, orphan.AccountId)
		domainUser := &crossdomain.Account{
			DomainEntity: domainlayer.DomainEntity{
				Id: accountIdGen.Generate(data.Options.ConnectionId, orphan.AccountId),
			},
			UserName: orphan.Login,
			FullName: orphan.Login,
		}
		err = db.CreateOrUpdate(domainUser)
		if err != nil {
			return err
		}
	}
	if err := cursor.Err(); err != nil {
		return errors.Default.Wrap(err, "iterating repo accounts cursor")
	}

	return nil
}
