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

import "github.com/apache/incubator-devlake/core/models/common"

// DedupCanonicalScope maps a repository URL to the canonical domain ID to use
// for deduplication. When the same physical repository is collected via multiple
// connections, only the connection with the numerically lowest connection_id is
// considered canonical. The MySQL views (deduped_repos, deduped_pull_requests,
// deduped_issues, deduped_repo_commits) filter through this table.
type DedupCanonicalScope struct {
	common.NoPKModel
	// Url is the repository html_url, used as the cross-connection natural key.
	Url string `gorm:"primaryKey;type:varchar(500)"`
	// EntityType is the kind of entity (currently only "repo").
	EntityType string `gorm:"primaryKey;type:varchar(100)"`
	// CanonicalId is the domain-layer ID for the canonical record, e.g.
	// "github:GithubRepo:1:498260751".
	CanonicalId string `gorm:"type:varchar(255);index"`
}

// TableName returns the database table name.
func (DedupCanonicalScope) TableName() string {
	return "_tool_dedup_canonical_scopes"
}
