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

package models

import (
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

type LangfuseConn struct {
	helper.RestConnection `mapstructure:",squash"`
	helper.BasicAuth      `mapstructure:",squash"`
}

func (c *LangfuseConn) PrepareApiClient(apiClient plugin.ApiClient) errors.Error {
	apiClient.SetHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	return nil
}

func (c LangfuseConn) Sanitize() LangfuseConn {
	c.Password = ""
	return c
}

type LangfuseConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	LangfuseConn          `mapstructure:",squash"`
}

func (LangfuseConnection) TableName() string {
	return "_tool_langfuse_connections"
}

func (c LangfuseConnection) Sanitize() LangfuseConnection {
	c.LangfuseConn = c.LangfuseConn.Sanitize()
	return c
}

func (c *LangfuseConnection) MergeFromRequest(target *LangfuseConnection, body map[string]interface{}) error {
	password := target.Password
	if err := helper.DecodeMapStruct(body, target, true); err != nil {
		return err
	}
	if target.Password == "" {
		target.Password = password
	}
	return nil
}
