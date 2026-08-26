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

// repoAccountForConvert is the row projected by ConvertAccounts' query: every
// account referenced by the repo (from _tool_github_repo_accounts), enriched
// with profile detail from _tool_github_accounts when it was collected. The
// embedded NoPKModel carries the RawDataOrigin across to the domain row.
type repoAccountForConvert struct {
	Id        int
	Login     string
	Name      string
	Email     string
	AvatarUrl string
	Type      string
	common.NoPKModel
}

func init() {
	RegisterSubtaskMeta(&ConvertAccountsMeta)
}

var ConvertAccountsMeta = plugin.SubTaskMeta{
	Name:             "Convert Users",
	EntryPoint:       ConvertAccounts,
	EnabledByDefault: true,
	Description:      "Convert every account referenced by the repo (tool layer repo_accounts, enriched by github_accounts) into domain layer table accounts",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{
		models.GithubRepoAccount{}.TableName(), // cursor (every user referenced by the repo)
		models.GithubAccount{}.TableName(),     // left-join enrichment (profile detail, optional)
		models.GithubAccountOrg{}.TableName()}, // org pluck
	ProductTables: []string{crossdomain.Account{}.TableName()},
}

func ConvertAccounts(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubTaskData)

	accountIdGen := didgen.NewDomainIdGenerator(&models.GithubAccount{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[repoAccountForConvert]{
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
				// Enrichment fields (name/email/avatar/type) prefer ga: it's the richer,
				// individually-fetched profile. Raw-data provenance prefers the opposite —
				// _tool_github_repo_accounts, not ga — because ga (_tool_github_accounts)
				// is keyed by (connection_id, id) only, not repo-scoped, so a shared
				// account's ga row can carry a different repo's raw scope. Full-refresh
				// deletion matches domain rows by _raw_data_table/_raw_data_params against
				// the current subtask's own scope; stamping a shared account with the
				// wrong repo's provenance would make it deletable by that other repo's
				// full sync. repo_accounts is always present (FROM, not LEFT JOIN) and its
				// provenance is set by the repo-scoped extractor that created it, so it's
				// always correctly scoped to this conversion run.
				dal.Select(`_tool_github_repo_accounts.account_id AS id,
					_tool_github_repo_accounts.login AS login,
					COALESCE(ga.name, '') AS name,
					COALESCE(ga.email, '') AS email,
					COALESCE(ga.avatar_url, '') AS avatar_url,
					COALESCE(ga.type, '') AS type,
					COALESCE(_tool_github_repo_accounts._raw_data_params, ga._raw_data_params, '') AS _raw_data_params,
					COALESCE(_tool_github_repo_accounts._raw_data_table, ga._raw_data_table, '') AS _raw_data_table,
					COALESCE(_tool_github_repo_accounts._raw_data_id, ga._raw_data_id, 0) AS _raw_data_id,
					COALESCE(_tool_github_repo_accounts._raw_data_remark, ga._raw_data_remark, '') AS _raw_data_remark`),
				dal.From(&models.GithubRepoAccount{}),
				dal.Join(`left join _tool_github_accounts ga on (
					ga.connection_id = _tool_github_repo_accounts.connection_id
					AND ga.id = _tool_github_repo_accounts.account_id
				)`),
				dal.Where(
					`_tool_github_repo_accounts.repo_github_id = ?
						AND _tool_github_repo_accounts.connection_id = ?
						AND _tool_github_repo_accounts.account_id > 0`,
					data.Options.GithubId,
					data.Options.ConnectionId,
				),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					// Either side of the join can be the one that changed: a repo_account
					// row updates when the repo's membership changes, but the enrichment
					// profile (name/email/avatar/type — including the signals IsBot reads)
					// lives on ga and has its own updated_at.
					clauses = append(clauses, dal.Where(
						"(_tool_github_repo_accounts.updated_at >= ? OR ga.updated_at >= ?)",
						since, since,
					))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubUser *repoAccountForConvert) ([]interface{}, errors.Error) {
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

			domainUser := buildDomainAccount(accountIdGen.Generate(data.Options.ConnectionId, githubUser.Id), githubUser, orgStr)
			return []interface{}{
				domainUser,
			}, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

// isBotAccount reports whether a GitHub account identifies a bot, based on
// the API-reported account type or well-known login conventions ([bot]
// suffix for GitHub Apps, -bot/-robot suffixes, or exact logins
// copilot/dependabot/github-actions/codecov-commenter).
func isBotAccount(login string, accountType string) bool {
	if accountType == "Bot" {
		return true
	}
	lowerLogin := strings.ToLower(login)
	if strings.HasSuffix(lowerLogin, "[bot]") ||
		strings.HasSuffix(lowerLogin, "-bot") ||
		strings.HasSuffix(lowerLogin, "-robot") {
		return true
	}
	return lowerLogin == "copilot" ||
		lowerLogin == "dependabot" ||
		lowerLogin == "github-actions" ||
		lowerLogin == "codecov-commenter"
}

// buildDomainAccount converts a repo-referenced account row (enriched with
// profile detail when available) into a domain Account, flagging bot
// identities via isBotAccount.
func buildDomainAccount(id string, row *repoAccountForConvert, orgStr string) *crossdomain.Account {
	fullName := row.Name
	if fullName == "" {
		fullName = row.Login
	}
	return &crossdomain.Account{
		DomainEntity: domainlayer.DomainEntity{Id: id},
		Email:        row.Email,
		FullName:     fullName,
		UserName:     row.Login,
		AvatarUrl:    row.AvatarUrl,
		Organization: orgStr,
		IsBot:        isBotAccount(row.Login, row.Type),
	}
}
