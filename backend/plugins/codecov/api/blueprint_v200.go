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

package api

import (
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/srvhelper"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

// MakeDataSourcePipelinePlanV200 creates a pipeline plan for Codecov
// Since we don't have tasks yet, this returns an empty plan but properly handles scopes
func MakeDataSourcePipelinePlanV200(
	connectionId uint64,
	bpScopes []*coreModels.BlueprintScope,
) (coreModels.PipelinePlan, []plugin.Scope, errors.Error) {
	// load connection, scope and scopeConfig from the db
	connection, err := dsHelper.ConnSrv.FindByPk(connectionId)
	if err != nil {
		return nil, nil, err
	}
	scopeDetails, err := dsHelper.ScopeSrv.MapScopeDetails(connectionId, bpScopes)
	if err != nil {
		// Provide a more helpful error message if scope is not found
		if lakeErr := errors.AsLakeErrorType(err); lakeErr != nil && lakeErr.As(errors.NotFound) != nil {
			return nil, nil, errors.BadInput.Wrap(err, "One or more scopes were not found in the database. Please save the scopes to the connection first before adding them to a project.")
		}
		return nil, nil, err
	}

	// For now, return empty plan since we don't have tasks yet
	plan := make(coreModels.PipelinePlan, 0)

	// Return scopes for project mapping
	scopes, err := makeScopesV200(scopeDetails, connection)
	if err != nil {
		return nil, nil, err
	}

	return plan, scopes, nil
}

func makeScopesV200(
	scopeDetails []*srvhelper.ScopeDetail[models.CodecovRepo, models.CodecovScopeConfig],
	connection *models.CodecovConnection,
) ([]plugin.Scope, errors.Error) {
	scopes := make([]plugin.Scope, 0, len(scopeDetails))

	for _, scopeDetail := range scopeDetails {
		scope := scopeDetail.Scope
		// Return the CodecovRepo itself as it implements plugin.Scope
		scopes = append(scopes, scope)
	}

	return scopes, nil
}
