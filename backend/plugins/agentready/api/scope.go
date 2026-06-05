package api

import (
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

func GetScopeDispatcher(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	scopeIdWithSuffix := strings.TrimLeft(input.Params["scopeId"], "/")
	if scopeId, found := strings.CutSuffix(scopeIdWithSuffix, "/latest-sync-state"); found {
		input.Params["scopeId"] = scopeId
		return GetScopeLatestSyncState(input)
	}
	return GetScope(input)
}

func GetScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.GetScopeDetail(input)
}

func PatchScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.Patch(input)
}

func DeleteScope(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	input.Params["scopeId"] = strings.TrimLeft(input.Params["scopeId"], "/")
	return dsHelper.ScopeApi.Delete(input)
}

func GetScopeList(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.GetPage(input)
}

func PutScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ScopeApi.PutMultiple(input)
}
