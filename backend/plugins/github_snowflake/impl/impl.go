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

package impl

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coremodels "github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/snowflakehelper"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/apache/incubator-devlake/plugins/github_snowflake/api"
	"github.com/apache/incubator-devlake/plugins/github_snowflake/models"
	"github.com/apache/incubator-devlake/plugins/github_snowflake/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/github_snowflake/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginApi
	plugin.PluginSource
	plugin.CloseablePluginTask
} = (*GithubSnowflake)(nil)

// githubPluginStub is a minimal plugin.PluginMeta that lets didgen resolve
// types from plugins/github/models without requiring the full github plugin
// to be loaded alongside github_snowflake.
type githubPluginStub struct{}

func (githubPluginStub) Name() string { return "github" }
func (githubPluginStub) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/github"
}
func (githubPluginStub) Description() string { return "" }

// GithubSnowflake is the plugin implementation struct.
type GithubSnowflake struct{}

func (p GithubSnowflake) Connection() dal.Tabler {
	return &models.SnowflakeGithubConnection{}
}

func (p GithubSnowflake) Scope() plugin.ToolLayerScope {
	return &githubmodels.GithubRepo{}
}

func (p GithubSnowflake) ScopeConfig() dal.Tabler {
	return &githubmodels.GithubScopeConfig{}
}

func (p GithubSnowflake) Init(basicRes context.BasicRes) errors.Error {
	// Only register the github stub when the real github plugin is absent.
	if _, err := plugin.GetPlugin("github"); err != nil {
		_ = plugin.RegisterPlugin("github", githubPluginStub{})
	}
	api.Init(basicRes, p)
	return nil
}

func (p GithubSnowflake) Name() string {
	return "github_snowflake"
}

func (p GithubSnowflake) Description() string {
	return "Ingest GitHub data from a Snowflake replica (Fivetran) instead of the GitHub REST/GraphQL API"
}

func (p GithubSnowflake) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/github_snowflake"
}

func (p GithubSnowflake) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// Sync tasks: Snowflake → _tool_github_* tables
		tasks.SyncReposMeta,
		tasks.SyncPullRequestsMeta,
		tasks.SyncPrCommitsMeta,
		tasks.SyncPrReviewsMeta,
		tasks.SyncReviewersMeta,
		tasks.SyncAccountsMeta,
		// Convertor tasks: _tool_github_* → domain layer
		tasks.ConvertRepoMeta,
		tasks.ConvertPullRequestsMeta,
		tasks.ConvertPullRequestCommitsMeta,
		tasks.ConvertPullRequestReviewsMeta,
		tasks.ConvertReviewsMeta,
		tasks.ConvertAccountsMeta,
	}
}

func (p GithubSnowflake) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}

	connection := &models.SnowflakeGithubConnection{}
	connectionHelper := helper.NewConnectionHelper(taskCtx, nil, p.Name())
	if err := connectionHelper.FirstById(connection, op.ConnectionId); err != nil {
		return nil, errors.Default.Wrap(err, "unable to get github_snowflake connection")
	}

	// Ensure a GithubRepo scope record exists so convertors can find it.
	repo := &githubmodels.GithubRepo{}
	dbErr := taskCtx.GetDal().First(repo, dal.Where("connection_id = ? AND github_id = ?", op.ConnectionId, op.GithubId))
	if dbErr != nil && taskCtx.GetDal().IsErrorNotFound(dbErr) {
		repo = &githubmodels.GithubRepo{
			Scope:    coremodels.Scope{ConnectionId: op.ConnectionId},
			GithubId: op.GithubId,
			Name:     tasks.RepoShortName(op.Name),
			FullName: op.Name,
			HTMLUrl:  "https://github.com/" + op.Name,
			CloneUrl: "https://github.com/" + op.Name + ".git",
		}
		if createErr := taskCtx.GetDal().CreateIfNotExist(repo); createErr != nil {
			return nil, errors.Default.Wrap(createErr, "failed to create GithubRepo scope record")
		}
	} else if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to look up GithubRepo scope record")
	} else {
		// Inherit name from existing scope if options omitted details.
		if op.Name == "" && repo.FullName != "" {
			op.Name = repo.FullName
			op.FullName = repo.FullName
		}
	}

	if op.ScopeConfigId == 0 && repo.ScopeConfigId != 0 {
		op.ScopeConfigId = repo.ScopeConfigId
	}
	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig githubmodels.GithubScopeConfig
		if loadErr := taskCtx.GetDal().First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId)); loadErr != nil {
			return nil, errors.BadInput.Wrap(loadErr, "failed to load scope config")
		}
		op.ScopeConfig = &scopeConfig
	}
	if op.ScopeConfig == nil {
		op.ScopeConfig = new(githubmodels.GithubScopeConfig)
	}

	snowDB, openErr := snowflakehelper.Open(
		connection.Account,
		connection.User,
		connection.AuthType,
		connection.PrivateKey,
		connection.Database,
		connection.Schema,
		connection.Warehouse,
		connection.Role,
	)
	if openErr != nil {
		return nil, openErr
	}

	return &tasks.GithubSnowflakeTaskData{
		Options:     op,
		SnowflakeDB: snowDB,
	}, nil
}

// Close is called after all subtasks complete; it closes the Snowflake connection.
func (p GithubSnowflake) Close(taskCtx plugin.TaskContext) errors.Error {
	data, ok := taskCtx.GetData().(*tasks.GithubSnowflakeTaskData)
	if ok && data != nil && data.SnowflakeDB != nil {
		if err := data.SnowflakeDB.Close(); err != nil {
			return errors.Default.Wrap(err, "failed to close Snowflake connection")
		}
	}
	return nil
}

func (p GithubSnowflake) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.SnowflakeGithubConnection{},
	}
}

func (p GithubSnowflake) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p GithubSnowflake) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.GetConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
	}
}
