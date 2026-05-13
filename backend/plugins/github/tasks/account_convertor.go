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
	"reflect"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/github/models"
)

func init() {
	RegisterSubtaskMeta(&ConvertAccountsMeta)
}

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Users",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert tool layer table github_accounts into  domain layer table accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.GithubAccount{}.TableName(),     // cursor
		models.GithubRepoAccount{}.TableName(), // cursor
		models.GithubAccountOrg{}.TableName()}, // account id gen
	ProductTables: []string{crossdomain.Account{}.TableName()},
}

type GithubAccountWithOrg struct {
	models.GithubAccount
	Login string `json:"login" gorm:"type:varchar(255)"`
	common.NoPKModel
}

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)

	cursor, err := db.Cursor(
		dal.Select("_tool_github_accounts.*"),
		dal.From(&models.GithubAccount{}),
		dal.Where(
			"repo_github_id = ? and _tool_github_accounts.connection_id=?",
			data.Options.GithubId,
			data.Options.ConnectionId,
		),
		dal.Join(`left join _tool_github_repo_accounts gra on (
			_tool_github_accounts.connection_id = gra.connection_id
			AND _tool_github_accounts.id = gra.account_id
		)`),
	)
	if err != nil {
		return err
	}
	defer cursor.Close()

	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewDataConverter(api.DataConverterArgs{
		InputRowType: reflect.TypeOf(models.GithubAccount{}),
		Input:        cursor,
		RawDataSubTaskArgs: api.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
			Table: RAW_ACCOUNT_TABLE,
		},
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			githubUser := inputRow.(*models.GithubAccount)

			// query related orgs
			var orgs []string
			err := db.Pluck(`org_login`, &orgs,
				dal.From(&models.GithubAccountOrg{}),
				dal.Where(`account_id = ? and connection_id = ?`, githubUser.Id, data.Options.ConnectionId),
			)
			if err != nil {
				return nil, err
			}
			var orgStr string
			if len(orgs) == 0 {
				orgStr = ``
			} else {
				orgStr = strings.Join(orgs, `,`)
				if len(orgStr) > 255 {
					orgStr = orgStr[:255]
				}
			}

			domainUser := &crossdomain.Account{
				DomainEntity: domainlayer.DomainEntity{Id: accountIdGen.Generate(data.Options.ConnectionId, githubUser.Id)},
				Email:        githubUser.Email,
				FullName:     githubUser.Name,
				UserName:     githubUser.Login,
				AvatarUrl:    githubUser.AvatarUrl,
				Organization: orgStr,
			}
			return []interface{}{
				domainUser,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	err = converter.Execute()
	if err != nil {
		return err
	}

	// Second pass: create domain Account rows for repo accounts that have no
	// matching _tool_github_accounts record. This typically happens for GitHub
	// App bot accounts (e.g. "dependabot[bot]", "renovate[bot]") because the
	// /users/<login> API returns 404 for bots, so no GithubAccount row is
	// created during collection.
	return convertOrphanedRepoAccounts(taskCtx, db, data, accountIdGen)
}

// convertOrphanedRepoAccounts finds GithubRepoAccount entries that have no
// corresponding GithubAccount row (typically bot accounts) and creates minimal
// domain Account records for them so that dashboard JOINs on accounts don't
// lose bot identity.
func convertOrphanedRepoAccounts(taskCtx plugin.SubTaskContext, db dal.Dal, data *GithubTaskData, accountIdGen *didgen.DomainIdGenerator) errors.Error {
	cursor, err := db.Cursor(
		dal.Select("_tool_github_repo_accounts.*"),
		dal.From(&models.GithubRepoAccount{}),
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
		var orphan models.GithubRepoAccount
		err = db.Fetch(cursor, &orphan)
		if err != nil {
			return err
		}
		logger.Info("creating domain account for orphaned repo account (likely bot): login=%s, id=%d", orphan.Login, orphan.AccountId)
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

	return nil
}
