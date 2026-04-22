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
	"github.com/apache/incubator-devlake/plugins/langfuse/api"
	"github.com/apache/incubator-devlake/plugins/langfuse/models"
	"github.com/apache/incubator-devlake/plugins/langfuse/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/langfuse/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginApi
	plugin.PluginModel
	plugin.PluginMigration
	plugin.PluginSource
	plugin.DataSourcePluginBlueprintV200
} = (*Langfuse)(nil)

type Langfuse struct{}

func (p Langfuse) Connection() dal.Tabler      { return &models.LangfuseConnection{} }
func (p Langfuse) Scope() plugin.ToolLayerScope { return &models.LangfuseProject{} }
func (p Langfuse) ScopeConfig() dal.Tabler      { return &models.LangfuseScopeConfig{} }

func (p Langfuse) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p Langfuse) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.LangfuseConnection{},
		&models.LangfuseProject{},
		&models.LangfuseScopeConfig{},
		&models.LangfuseTrace{},
		&models.LangfuseObservation{},
	}
}

func (p Langfuse) Description() string  { return "Collect and visualize LLM agent traces from Langfuse" }
func (p Langfuse) Name() string         { return "langfuse" }
func (p Langfuse) RootPkgPath() string  { return "github.com/apache/incubator-devlake/plugins/langfuse" }

func (p Langfuse) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p Langfuse) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectTracesMeta,
		tasks.ExtractTracesMeta,
		tasks.CollectObservationsMeta,
		tasks.ExtractObservationsMeta,
	}
}

func (p Langfuse) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeAndValidateTaskOptions(options)
	if err != nil {
		return nil, err
	}

	db := taskCtx.GetDal()
	connection := &models.LangfuseConnection{}
	err = db.First(connection, dal.Where("id = ?", op.ConnectionId))
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "unable to get Langfuse connection")
	}

	apiClient, err := helper.NewApiClientFromConnection(taskCtx.GetContext(), taskCtx, connection)
	if err != nil {
		return nil, err
	}

	asyncApiClient, err := helper.CreateAsyncApiClient(
		taskCtx,
		apiClient,
		&helper.ApiRateLimitCalculator{UserRateLimitPerHour: 1000},
	)
	if err != nil {
		return nil, err
	}

	project := &models.LangfuseProject{}
	_ = db.First(project, dal.Where("connection_id = ? AND project_id = ?", op.ConnectionId, op.ProjectId))

	return &tasks.LangfuseTaskData{
		Options:   op,
		ApiClient: asyncApiClient,
		Project:   project,
	}, nil
}

func (p Langfuse) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
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
		"connections/:connectionId/scopes/:scopeId": {
			"GET":    api.GetScope,
			"PATCH":  api.PatchScope,
			"DELETE": api.DeleteScope,
		},
		"connections/:connectionId/scopes/:scopeId/latest-sync-state": {
			"GET": api.GetScopeLatestSyncState,
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
	}
}

func (p Langfuse) MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	scopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	return api.MakeDataSourcePipelinePlanV200(p.SubTaskMetas(), connectionId, scopes)
}
