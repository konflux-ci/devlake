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
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/didgen"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
)

var ConvertPullRequestCommitsMeta = plugin.SubTaskMeta{
	Name:             "convertPrCommits",
	EntryPoint:       ConvertPullRequestCommits,
	EnabledByDefault: true,
	Description:      "Convert tool layer PR commits into domain pull_request_commits",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS, plugin.DOMAIN_TYPE_CODE_REVIEW},
	DependencyTables: []string{
		githubmodels.GithubPrCommit{}.TableName(),
		githubmodels.GithubPullRequest{}.TableName(),
	},
	ProductTables: []string{code.PullRequestCommit{}.TableName()},
}

func ConvertPullRequestCommits(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	data := taskCtx.GetData().(*GithubSnowflakeTaskData)
	logger := taskCtx.GetLogger()
	repoId := data.Options.GithubId

	pullIdGen := didgen.NewDomainIdGenerator(&githubmodels.GithubPullRequest{})

	converter, err := api.NewStatefulDataConverter(&api.StatefulDataConverterArgs[githubmodels.GithubPrCommit]{
		SubtaskCommonArgs: &api.SubtaskCommonArgs{
			SubTaskContext: taskCtx,
			Table:          RAW_PR_COMMIT_TABLE,
			Params: GithubApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.Name,
			},
		},
		Input: func(stateManager *api.SubtaskStateManager) (dal.Rows, errors.Error) {
			clauses := []dal.Clause{
				dal.From(&githubmodels.GithubPrCommit{}),
				dal.Join(`left join _tool_github_pull_requests on _tool_github_pull_requests.github_id = _tool_github_pull_request_commits.pull_request_id AND _tool_github_pull_requests.connection_id = _tool_github_pull_request_commits.connection_id`),
				dal.Where("_tool_github_pull_requests.repo_id = ? and _tool_github_pull_requests.connection_id = ?", repoId, data.Options.ConnectionId),
				dal.Orderby("pull_request_id ASC"),
			}
			if stateManager.IsIncremental() {
				since := stateManager.GetSince()
				if since != nil {
					clauses = append(clauses, dal.Where("_tool_github_pull_requests.github_updated_at >= ?", since))
				}
			}
			return db.Cursor(clauses...)
		},
		Convert: func(githubPullRequestCommit *githubmodels.GithubPrCommit) ([]interface{}, errors.Error) {
			domainPrCommit := &code.PullRequestCommit{
				CommitSha:          githubPullRequestCommit.CommitSha,
				PullRequestId:      pullIdGen.Generate(data.Options.ConnectionId, githubPullRequestCommit.PullRequestId),
				CommitAuthorName:   githubPullRequestCommit.CommitAuthorName,
				CommitAuthorEmail:  githubPullRequestCommit.CommitAuthorEmail,
				CommitAuthoredDate: githubPullRequestCommit.CommitAuthoredDate,
			}
			return []interface{}{domainPrCommit}, nil
		},
	})
	if err != nil {
		return err
	}

	if !converter.IsIncremental() {
		logger.Debug("deleting outdated domain pull_request_commits for repo %d", repoId)
		if dbErr := db.Delete(&code.PullRequestCommit{}, dal.Where("_raw_data_params = ?", converter.GetRawDataParams())); dbErr != nil {
			return dbErr
		}
	}

	return converter.Execute()
}
