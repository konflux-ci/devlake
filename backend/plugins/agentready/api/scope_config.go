package api

import (
	"net/http"
	"strconv"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

func GetScopeConfigs(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var configs []models.AgentReadyScopeConfig
	err := db.All(&configs, dal.From(&models.AgentReadyScopeConfig{}))
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to query scope configs")
	}
	return &plugin.ApiResourceOutput{
		Body:   configs,
		Status: http.StatusOK,
	}, nil
}

func CreateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var config models.AgentReadyScopeConfig
	err := helper.Decode(input.Body, &config, nil)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode scope config")
	}
	if config.AssessmentFilePath == "" {
		config.AssessmentFilePath = models.DefaultAssessmentFilePath
	}
	dbErr := db.Create(&config)
	if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to create scope config")
	}
	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusCreated,
	}, nil
}

func GetScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	id, parseErr := strconv.ParseUint(input.Params["id"], 10, 64)
	if parseErr != nil {
		return nil, errors.BadInput.Wrap(parseErr, "invalid scope config id")
	}
	var config models.AgentReadyScopeConfig
	err := db.First(&config, dal.Where("id = ?", id))
	if err != nil {
		return nil, errors.Default.Wrap(err, "scope config not found")
	}
	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusOK,
	}, nil
}

func UpdateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	id, parseErr := strconv.ParseUint(input.Params["id"], 10, 64)
	if parseErr != nil {
		return nil, errors.BadInput.Wrap(parseErr, "invalid scope config id")
	}
	var config models.AgentReadyScopeConfig
	err := db.First(&config, dal.Where("id = ?", id))
	if err != nil {
		return nil, errors.Default.Wrap(err, "scope config not found")
	}
	decodeErr := helper.Decode(input.Body, &config, nil)
	if decodeErr != nil {
		return nil, errors.BadInput.Wrap(decodeErr, "failed to decode update")
	}
	dbErr := db.Update(&config)
	if dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to update scope config")
	}
	return &plugin.ApiResourceOutput{
		Body:   config,
		Status: http.StatusOK,
	}, nil
}

func DeleteScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	id, parseErr := strconv.ParseUint(input.Params["id"], 10, 64)
	if parseErr != nil {
		return nil, errors.BadInput.Wrap(parseErr, "invalid scope config id")
	}
	err := db.Delete(&models.AgentReadyScopeConfig{}, dal.Where("id = ?", id))
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to delete scope config")
	}
	return &plugin.ApiResourceOutput{
		Status: http.StatusNoContent,
	}, nil
}
