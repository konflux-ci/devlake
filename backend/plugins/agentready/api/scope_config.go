package api

import (
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

func CreateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Post(input)
}

func UpdateScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Patch(input)
}

func GetScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetDetail(input)
}

func DeleteScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.Delete(input)
}

func GetScopeConfigList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetAll(input)
}

func GetProjectsByScopeConfig(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeConfigApi.GetProjectsByScopeConfig(input)
}
