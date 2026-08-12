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
	"encoding/json"
	"fmt"
	"net/http"
)

// QueryParams holds the normalized request body sent by the dashboard.
// All fields mirror what dashboard.js builds in buildDiagramRequest().
type QueryParams struct {
	// Owner is the list of GitHub/GitLab organisation names.
	Owner []string `json:"owner"`
	// Name is the list of repository names (parallel to Owner).
	Name []string `json:"name"`
	// From is the start of the time window as a Unix timestamp in seconds.
	From int64 `json:"from"`
	// To is the end of the time window as a Unix timestamp in seconds.
	To int64 `json:"to"`
	// BlueprintID is the DevLake blueprint ID for this team/product.
	BlueprintID string `json:"blueprintid"`
	// ConnectionID is the DevLake connection ID for the primary data source.
	ConnectionID string `json:"connectionid"`
	// JiraProject is the Jira project key (only for issue dashboards).
	JiraProject string `json:"jiraproject"`
	// Projects is used by issue dashboards in place of owner/name pairs.
	Projects []string `json:"projects"`
	// UserWhitelist restricts metrics to a specific set of contributors.
	UserWhitelist []string `json:"userwhitelist"`
	// TeamRepos is an explicit repo list used by some AR flows.
	TeamRepos []string `json:"teamrepos"`
	// ProjectName is the DevLake project_name scoping key (used by AI Review, AICS).
	ProjectName string `json:"projectname"`
}

// DecodeQueryParams reads and validates the POST body into a QueryParams value.
// Returns an error suitable for returning as a 400 to the caller.
func DecodeQueryParams(r *http.Request) (QueryParams, error) {
	var p QueryParams
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		return p, fmt.Errorf("decoding request body: %w", err)
	}
	return p, nil
}
