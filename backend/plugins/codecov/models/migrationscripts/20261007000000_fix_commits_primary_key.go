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
)

type fixCommitsPrimaryKey struct{}

// Up deduplicates _tool_codecov_commits and adds the missing composite
// primary key (connection_id, repo_id, commit_sha).
//
// Migration 20260615 dropped the old auto_increment id and called
// AutoMigrateTables, but MySQL's AutoMigrate does not reliably add a
// composite PK to an existing table. The result: no PK, so
// CreateOrUpdate always inserts, causing ~32x row duplication.
func (u *fixCommitsPrimaryKey) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	logger := basicRes.GetLogger()

	// 1. Check if PK already exists (idempotent)
	rows, err := db.RawCursor(`
		SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = '_tool_codecov_commits'
		  AND CONSTRAINT_TYPE = 'PRIMARY KEY'
	`)
	if err == nil && rows != nil {
		defer rows.Close()
		if rows.Next() {
			var pkCount int
			if scanErr := rows.Scan(&pkCount); scanErr == nil && pkCount > 0 {
				logger.Info("[fix-commits-pk] PK already exists, skipping")
				return nil
			}
		}
	}

	// 2. Deduplicate — keep the row with the latest updated_at per (connection_id, repo_id, commit_sha)
	logger.Info("[fix-commits-pk] Deduplicating _tool_codecov_commits")

	err = db.Exec(`
		CREATE TABLE _tool_codecov_commits_dedup LIKE _tool_codecov_commits
	`)
	if err != nil {
		return errors.Default.Wrap(err, "failed to create dedup table")
	}

	err = db.Exec(`
		INSERT INTO _tool_codecov_commits_dedup
		SELECT t.*
		FROM _tool_codecov_commits t
		INNER JOIN (
			SELECT connection_id, repo_id, commit_sha, MAX(updated_at) AS max_updated
			FROM _tool_codecov_commits
			GROUP BY connection_id, repo_id, commit_sha
		) keep ON t.connection_id = keep.connection_id
			AND t.repo_id = keep.repo_id
			AND t.commit_sha = keep.commit_sha
			AND t.updated_at = keep.max_updated
	`)
	if err != nil {
		_ = db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`)
		return errors.Default.Wrap(err, "failed to copy unique rows")
	}

	// 3. Atomic rename
	err = db.Exec(`
		RENAME TABLE _tool_codecov_commits TO _tool_codecov_commits_old,
		             _tool_codecov_commits_dedup TO _tool_codecov_commits
	`)
	if err != nil {
		_ = db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_dedup`)
		return errors.Default.Wrap(err, "failed to rename tables")
	}

	_ = db.Exec(`DROP TABLE IF EXISTS _tool_codecov_commits_old`)

	// 4. Add composite primary key
	logger.Info("[fix-commits-pk] Adding PRIMARY KEY (connection_id, repo_id, commit_sha)")
	err = db.Exec(`
		ALTER TABLE _tool_codecov_commits
		ADD PRIMARY KEY (connection_id, repo_id, commit_sha)
	`)
	if err != nil {
		return errors.Default.Wrap(err, "failed to add primary key")
	}

	logger.Info("[fix-commits-pk] Done — duplicates removed, PK added")
	return nil
}

func (*fixCommitsPrimaryKey) Version() uint64 {
	return 20261007000000
}

func (*fixCommitsPrimaryKey) Name() string {
	return "Codecov fix missing primary key on commits table and remove duplicates"
}
