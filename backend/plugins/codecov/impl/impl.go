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
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/codecov/api"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
	"github.com/apache/incubator-devlake/plugins/codecov/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/codecov/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
} = (*Codecov)(nil)

type Codecov struct{}

func (p Codecov) Connection() dal.Tabler {
	return &models.CodecovConnection{}
}

func (p Codecov) Scope() plugin.ToolLayerScope {
	return &models.CodecovRepo{}
}

func (p Codecov) ScopeConfig() dal.Tabler {
	return &models.CodecovScopeConfig{}
}

func (p Codecov) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Codecov) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.CodecovConnection{},
		&models.CodecovRepo{},
		&models.CodecovScopeConfig{},
		&models.CodecovFlag{},
		&models.CodecovCommit{},
		&models.CodecovCoverage{},
		&models.CodecovCoverageTrend{},
		&models.CodecovCommitCoverage{},
	}
}

func (p Codecov) Description() string {
	return "To collect and enrich data from Codecov"
}

func (p Codecov) Name() string {
	return "codecov"
}

func (p Codecov) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/codecov"
}

func (p Codecov) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Codecov) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"test": {
			"POST": api.TestConnection,
		},
		"connections": {
			"POST": api.PostConnections,
			"GET":  api.ListConnections,
		},
		"connections/:connectionId": {
			"GET":    api.GetConnection,
			"PATCH":  api.PatchConnection,
			"DELETE": api.DeleteConnection,
		},
		"connections/:connectionId/test": {
			"POST": api.TestExistingConnection,
		},
		"connections/:connectionId/scopes": {
			"GET": api.GetScopes,
			"PUT": api.PutScopes,
		},
		"connections/:connectionId/scopes/*scopeId": {
			// Behind 'GetScopeDispatcher', there are two paths so far:
			// GetScopeLatestSyncState "connections/:connectionId/scopes/:scopeId/latest-sync-state"
			// GetScope "connections/:connectionId/scopes/:scopeId"
			// Because there may be slash in scopeId (codecovId like "owner/repo"), so we handle it manually.
			"GET":    api.GetScopeDispatcher,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scope-configs": {
			"POST": api.PostScopeConfig,
			"GET":  api.GetScopeConfigList,
		},
		"connections/:connectionId/scope-configs/:id": {
			"GET":    api.GetScopeConfig,
			"PATCH":  api.PatchScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
		"connections/:connectionId/remote-scopes": {
			"GET": api.RemoteScopes,
		},
		"connections/:connectionId/search-remote-scopes": {
			"GET": api.SearchRemoteScopes,
		},
	}
}

func (p Codecov) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		// Step 1: Collect and convert flags first (needed for flag-based coverage collection)
		tasks.CollectFlagsMeta,
		tasks.ConvertFlagsMeta,
		// Step 2: Collect commits
		tasks.CollectCommitsMeta,
		tasks.ExtractCommitsMeta,
		// Step 3: Collect coverage data (depends on flags and commits)
		tasks.CollectCommitTotalsMeta,
		tasks.CollectCommitCoverageMeta,
		tasks.CollectComparisonMeta,
		tasks.CollectFlagCoverageTrendMeta,
		// Step 4: Convert coverage data
		tasks.ConvertComparisonMeta,
		tasks.ConvertCoverageMeta,
		tasks.ConvertCommitCoverageMeta,
		tasks.ConvertCoverageTrendMeta,
	}
}

func (p Codecov) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}

	db := taskCtx.GetDal()
	connection := &models.CodecovConnection{}
	err = db.First(connection, dal.Where("id = ?", op.ConnectionId))
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "unable to get Codecov connection by the given connection ID")
	}

	// create synchronize api client
	apiClient, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	// create async api client
	asyncApiClient, err := helper.CreateAsyncApiClient(
		taskCtx,
		apiClient,
		nil, // no rate limiter for now
	)
	if err != nil {
		return nil, err
	}

	return &tasks.CodecovTaskData{
		Options:   op,
		ApiClient: asyncApiClient,
	}, nil
}

func (p Codecov) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}
