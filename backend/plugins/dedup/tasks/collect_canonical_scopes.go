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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/dedup/models"
)

// CollectCanonicalScopesMeta defines the subtask metadata.
var CollectCanonicalScopesMeta = plugin.SubTaskMeta{
	Name:             "collectCanonicalScopes",
	EntryPoint:       CollectCanonicalScopes,
	EnabledByDefault: true,
	Description:      "Identify the canonical connection for each physical repository URL and populate _tool_dedup_canonical_scopes",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE, plugin.DOMAIN_TYPE_TICKET},
}

// githubRepoRow holds the query result used to build canonical scope entries.
type githubRepoRow struct {
	HtmlUrl      string `gorm:"column:html_url"`
	GithubId     int    `gorm:"column:github_id"`
	ConnectionId uint64 `gorm:"column:connection_id"`
}

// CollectCanonicalScopes queries _tool_github_repos to find repositories that
// appear under multiple connections (same html_url). For each unique URL it
// picks the numerically lowest connection_id as the canonical connection,
// constructs the corresponding domain-layer repo ID, and upserts a row into
// _tool_dedup_canonical_scopes.
//
// The four deduplicating MySQL views (deduped_repos, deduped_pull_requests,
// deduped_issues, deduped_repo_commits) all join through this table, so
// running this subtask refreshes the deduplication layer after every
// GitHub collection run.
func CollectCanonicalScopes(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	// Find the row with the minimum connection_id for each html_url.
	// Using a JOIN rather than MIN(id) string comparison ensures numeric
	// ordering even when connection IDs have different digit counts.
	var rows []githubRepoRow
	err := db.All(
		&rows,
		dal.Select("r.html_url, r.github_id, r.connection_id"),
		dal.From("_tool_github_repos r"),
		dal.Join(`
			INNER JOIN (
				SELECT html_url, MIN(connection_id) AS min_conn_id
				FROM _tool_github_repos
				WHERE html_url != ''
				GROUP BY html_url
			) m ON r.html_url = m.html_url AND r.connection_id = m.min_conn_id
		`),
		dal.Where("r.html_url != ''"),
	)
	if err != nil {
		return errors.Default.Wrap(err, "failed to query canonical GitHub repos")
	}

	if len(rows) == 0 {
		logger.Info("no GitHub repos found; skipping canonical scope collection")
		return nil
	}

	// Clear existing entries so stale canonical mappings don't linger after
	// a connection is deleted.
	if err := db.Delete(&models.DedupCanonicalScope{}, dal.Where("entity_type = ?", "repo")); err != nil {
		return errors.Default.Wrap(err, "failed to clear existing canonical scopes")
	}

	scopes := make([]*models.DedupCanonicalScope, 0, len(rows))
	for _, row := range rows {
		// Mirror the domain ID format used by the GitHub pr_convertor / repo_convertor:
		// "github:GithubRepo:{connectionId}:{githubId}"
		canonicalId := fmt.Sprintf("github:GithubRepo:%d:%d", row.ConnectionId, row.GithubId)
		scopes = append(scopes, &models.DedupCanonicalScope{
			Url:         row.HtmlUrl,
			EntityType:  "repo",
			CanonicalId: canonicalId,
		})
	}

	if err := db.CreateOrUpdate(scopes); err != nil {
		return errors.Default.Wrap(err, "failed to upsert canonical scopes")
	}

	logger.Info("collected %d canonical repo scopes", len(scopes))
	return nil
}
