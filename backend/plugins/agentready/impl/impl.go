package impl

import (
	"encoding/json"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/api"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
	"github.com/apache/incubator-devlake/plugins/agentready/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/agentready/tasks"
)

var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.PluginApi
	plugin.MetricPluginBlueprintV200
} = (*AgentReady)(nil)

// AgentReady is the DevLake plugin that collects and analyzes AI readiness
// assessments from repositories.
type AgentReady struct{}

func (p AgentReady) Init(basicRes context.BasicRes) errors.Error {
	api.Init(basicRes, p)
	return nil
}

func (p AgentReady) Description() string {
	return "Collect and analyze AI readiness assessments from repositories"
}

func (p AgentReady) Name() string {
	return "agentready"
}

func (p AgentReady) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/agentready"
}

func (p AgentReady) RequiredDataEntities() ([]map[string]interface{}, errors.Error) {
	return []map[string]interface{}{
		{
			"model": "repos",
			"requiredFields": map[string]string{
				"id":   "string",
				"name": "string",
			},
		},
	}, nil
}

func (p AgentReady) IsProjectMetric() bool {
	return true
}

func (p AgentReady) RunAfter() ([]string, errors.Error) {
	return []string{"github", "gitlab"}, nil
}

func (p AgentReady) Settings() interface{} {
	return nil
}

func (p AgentReady) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.AgentReadyAssessment{},
		&models.AgentReadyFinding{},
		&models.AgentReadyMetric{},
		&models.AgentReadyScopeConfig{},
	}
}

func (p AgentReady) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectAssessmentsMeta,
		tasks.ExtractAssessmentsMeta,
		tasks.CalculateMetricsMeta,
	}
}

func (p AgentReady) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	logger := taskCtx.GetLogger()
	logger.Debug("Preparing AgentReady task data: %v", options)

	op, err := tasks.DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}

	err = tasks.ValidateTaskOptions(op)
	if err != nil {
		return nil, err
	}

	if op.ScopeConfig == nil && op.ScopeConfigId != 0 {
		var scopeConfig models.AgentReadyScopeConfig
		db := taskCtx.GetDal()
		dbErr := db.First(&scopeConfig, dal.Where("id = ?", op.ScopeConfigId))
		if dbErr != nil && !db.IsErrorNotFound(dbErr) {
			return nil, errors.BadInput.Wrap(dbErr, "failed to get scopeConfig")
		}
		op.ScopeConfig = &scopeConfig
	}

	if op.ScopeConfig == nil {
		op.ScopeConfig = models.GetDefaultScopeConfig()
	}

	return &tasks.AgentReadyTaskData{
		Options: op,
	}, nil
}

func (p AgentReady) ApiResources() map[string]map[string]plugin.ApiResourceHandler {
	return map[string]map[string]plugin.ApiResourceHandler{
		"assessments": {
			"GET": api.GetAssessments,
		},
		"assessments/:id": {
			"GET": api.GetAssessment,
		},
		"assessments/:id/findings": {
			"GET": api.GetAssessmentFindings,
		},
		"stats": {
			"GET": api.GetStats,
		},
		"scope-configs": {
			"GET":  api.GetScopeConfigs,
			"POST": api.CreateScopeConfig,
		},
		"scope-configs/:id": {
			"GET":    api.GetScopeConfig,
			"PATCH":  api.UpdateScopeConfig,
			"DELETE": api.DeleteScopeConfig,
		},
	}
}

func (p AgentReady) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

func (p AgentReady) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	op := &tasks.AgentReadyOptions{}
	if options != nil && string(options) != "\"\"" {
		err := json.Unmarshal(options, op)
		if err != nil {
			return nil, errors.Default.WrapRaw(err)
		}
	}

	opts := map[string]interface{}{
		"projectName": projectName,
	}
	if op.ScopeConfigId != 0 {
		opts["scopeConfigId"] = op.ScopeConfigId
	}

	plan := coreModels.PipelinePlan{
		{
			{
				Plugin:  "agentready",
				Options: opts,
				Subtasks: []string{
					tasks.CollectAssessmentsMeta.Name,
					tasks.ExtractAssessmentsMeta.Name,
					tasks.CalculateMetricsMeta.Name,
				},
			},
		},
	}
	return plan, nil
}
