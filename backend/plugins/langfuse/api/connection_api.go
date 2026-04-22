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
	gocontext "context"
	"net/http"
	"net/url"

	"github.com/mitchellh/mapstructure"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/langfuse/models"
	"github.com/apache/incubator-devlake/server/api/shared"
)

type LangfuseTestConnResponse struct {
	shared.ApiBody
}

func TestConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	var conn models.LangfuseConn
	e := mapstructure.Decode(input.Body, &conn)
	if e != nil {
		return nil, errors.Convert(e)
	}
	testResult, err := testConnection(gocontext.TODO(), conn)
	if err != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, err)
	}
	return &plugin.ApiResourceOutput{Body: testResult, Status: http.StatusOK}, nil
}

func PostConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Post(input)
}

func PatchConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Patch(input)
}

func DeleteConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.Delete(input)
}

func ListConnections(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetAll(input)
}

func GetConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return dsHelper.ConnApi.GetDetail(input)
}

func TestExistingConnection(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection, err := dsHelper.ConnApi.GetMergedConnection(input)
	if err != nil {
		return nil, errors.Convert(err)
	}
	testResult, testErr := testConnection(gocontext.TODO(), connection.LangfuseConn)
	if testErr != nil {
		return nil, plugin.WrapTestConnectionErrResp(basicRes, testErr)
	}
	return &plugin.ApiResourceOutput{Body: testResult, Status: http.StatusOK}, nil
}

func testConnection(ctx gocontext.Context, conn models.LangfuseConn) (*LangfuseTestConnResponse, errors.Error) {
	if vld != nil {
		if err := vld.Struct(conn); err != nil {
			return nil, errors.Convert(err)
		}
	}

	apiClient, err := api.NewApiClientFromConnection(ctx, basicRes, &conn)
	if err != nil {
		return nil, err
	}

	res, err := apiClient.Get("api/public/traces", url.Values{"limit": []string{"1"}}, nil)
	if err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to connect to Langfuse")
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, errors.HttpStatus(http.StatusBadRequest).New("authentication failed, check public_key (username) and secret_key (password)")
	}
	if res.StatusCode != http.StatusOK {
		return nil, errors.HttpStatus(res.StatusCode).New("unexpected status from Langfuse API")
	}

	return &LangfuseTestConnResponse{
		ApiBody: shared.ApiBody{
			Success: true,
			Message: "success",
		},
	}, nil
}
