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

package tasks

import (
	"fmt"
	"strings"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// ParseFullName splits a "owner/repo" string into its components.
func ParseFullName(fullName string) (owner, repo string, err errors.Error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.BadInput.New("invalid fullName format, expected 'owner/repo'")
	}
	return parts[0], parts[1], nil
}

// RepoAPIPrefix builds the Codecov API v2 path prefix for a repo:
//
//	api/v2/{service}/{owner}/repos/{repo}
func RepoAPIPrefix(service, owner, repo string) string {
	return fmt.Sprintf("api/v2/%s/%s/repos/%s", service, owner, repo)
}

// deleteConvertedRaw removes raw rows for this connection/repo after a successful
// convert. Collectors skip from _tool_* so these rows are not needed as a skip-list.
func deleteConvertedRaw(taskCtx plugin.SubTaskContext, rawTable string, params CodecovApiParams) errors.Error {
	db := taskCtx.GetDal()
	table := "_raw_" + rawTable
	if !db.HasTable(table) {
		return nil
	}
	paramsJSON := plugin.MarshalScopeParams(params)
	err := db.Delete(&helper.RawData{}, dal.From(table), dal.Where("params = ?", paramsJSON))
	if err != nil {
		return errors.Default.Wrap(err, "failed to delete converted raw rows from "+table)
	}
	taskCtx.GetLogger().Info("deleted converted raw rows from %s", table)
	return nil
}
