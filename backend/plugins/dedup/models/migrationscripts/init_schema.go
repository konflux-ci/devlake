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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*initSchema)(nil)

type initSchema struct{}

// dedupCanonicalScope20260616 is the versioned inline struct used by AutoMigrateTables.
type dedupCanonicalScope20260616 struct {
	Url         string `gorm:"primaryKey;type:varchar(500)"`
	EntityType  string `gorm:"primaryKey;type:varchar(100)"`
	CanonicalId string `gorm:"type:varchar(255);index"`
}

func (dedupCanonicalScope20260616) TableName() string {
	return "_tool_dedup_canonical_scopes"
}

func (script *initSchema) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Create the canonical scopes mapping table.
	if err := migrationhelper.AutoMigrateTables(basicRes, &dedupCanonicalScope20260616{}); err != nil {
		return err
	}

	// Create deduplicating views. Each view surfaces only the records that
	// belong to the canonical connection (lowest connection_id) for each
	// physical repository URL.

	// deduped_repos: one row per physical repository URL.
	if err := db.Exec(`
		CREATE OR REPLACE VIEW deduped_repos AS
		SELECT r.*
		FROM repos r
		WHERE r.id IN (
			SELECT canonical_id
			FROM _tool_dedup_canonical_scopes
			WHERE entity_type = 'repo'
		)
	`); err != nil {
		return err
	}

	// deduped_pull_requests: PRs belonging to canonical repos only.
	// base_repo_id on pull_requests is the domain repo ID, which is canonical
	// for the chosen connection.
	if err := db.Exec(`
		CREATE OR REPLACE VIEW deduped_pull_requests AS
		SELECT pr.*
		FROM pull_requests pr
		WHERE pr.base_repo_id IN (
			SELECT canonical_id
			FROM _tool_dedup_canonical_scopes
			WHERE entity_type = 'repo'
		)
	`); err != nil {
		return err
	}

	// deduped_issues: Issues linked via board_issues to canonical boards.
	// For GitHub, board IDs are identical to repo domain IDs, so filtering
	// board_issues.board_id against canonical_ids correctly selects only the
	// canonical copy of each issue.
	if err := db.Exec(`
		CREATE OR REPLACE VIEW deduped_issues AS
		SELECT DISTINCT i.*
		FROM issues i
		INNER JOIN board_issues bi ON bi.issue_id = i.id
		WHERE bi.board_id IN (
			SELECT canonical_id
			FROM _tool_dedup_canonical_scopes
			WHERE entity_type = 'repo'
		)
	`); err != nil {
		return err
	}

	// deduped_repo_commits: repo-commit links for canonical repos only.
	if err := db.Exec(`
		CREATE OR REPLACE VIEW deduped_repo_commits AS
		SELECT rc.*
		FROM repo_commits rc
		WHERE rc.repo_id IN (
			SELECT canonical_id
			FROM _tool_dedup_canonical_scopes
			WHERE entity_type = 'repo'
		)
	`); err != nil {
		return err
	}

	return nil
}

func (script *initSchema) Version() uint64 {
	return 20260616000001
}

func (script *initSchema) Name() string {
	return "dedup init schema"
}
