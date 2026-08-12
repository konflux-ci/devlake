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

package pr

import (
	"github.com/apache/incubator-devlake/plugins/metrics/api"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
)

// toParams converts the decoded request body into query Params.
// Repos are stored in DevLake as "owner/name" (e.g. "redhat-developer/rhdh"),
// so we zip the parallel Owner and Name arrays.  If Owner is absent we fall
// back to Name as-is (handles teamrepos which are already full identifiers).
func toParams(p api.QueryParams) qpr.Params {
	var repos []string
	for i, name := range p.Name {
		if i < len(p.Owner) && p.Owner[i] != "" {
			repos = append(repos, p.Owner[i]+"/"+name)
		} else {
			repos = append(repos, name)
		}
	}
	if len(repos) == 0 {
		repos = p.TeamRepos
	}
	return qpr.Params{
		BlueprintID: p.BlueprintID,
		Repos:       repos,
		From:        p.From,
		To:          p.To,
		Whitelist:   p.UserWhitelist,
	}
}
