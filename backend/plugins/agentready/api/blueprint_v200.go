package api

import (
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/domainlayer"
	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/plugin"
	helperapi "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
	"github.com/apache/incubator-devlake/plugins/agentready/tasks"
)

func MakeDataSourcePipelinePlanV200(
	subtaskMetas []plugin.SubTaskMeta,
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		return nil, nil, err
	}
	plan, err := makePipelinePlan(subtaskMetas, scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}
	scopes := makeScopes(scopeDetails)
	return plan, scopes, nil
}

func makePipelinePlan(
	subtaskMetas []plugin.SubTaskMeta,
	scopeDetails []*srvhelper.ScopeDetail[models.AgentReadyScope, models.AgentReadyScopeConfig],
	connection *models.AgentReadyConnection,
) (coreModels.PipelinePlan, errors.Error) {
	plan := make(coreModels.PipelinePlan, len(scopeDetails))
	for i, scopeDetail := range scopeDetails {
		scope, scopeConfig := scopeDetail.Scope, scopeDetail.ScopeConfig

		entities := []string{plugin.DOMAIN_TYPE_CODE}
		if scopeConfig != nil && len(scopeConfig.Entities) > 0 {
			entities = scopeConfig.Entities
		}

		task, err := helperapi.MakePipelinePlanTask(
			"agentready",
			subtaskMetas,
			entities,
			tasks.AgentReadyOptions{
				ConnectionId: connection.ID,
				FullName:     scope.FullName,
				ScopeConfig:  scopeConfig,
			},
		)
		if err != nil {
			return nil, err
		}
		plan[i] = coreModels.PipelineStage{task}
	}
	return plan, nil
}

func makeScopes(
	scopeDetails []*srvhelper.ScopeDetail[models.AgentReadyScope, models.AgentReadyScopeConfig],
) []plugin.Scope {
	scopes := make([]plugin.Scope, 0, len(scopeDetails))
	for _, scopeDetail := range scopeDetails {
		scope := scopeDetail.Scope
		scopes = append(scopes, &code.Repo{
			DomainEntity: domainlayer.DomainEntity{Id: scope.FullName},
			Name:         scope.Name,
		})
	}
	return scopes
}
