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
)

var _ plugin.MigrationScript = (*fixPrimaryKeys)(nil)

type fixPrimaryKeys struct{}

func (*fixPrimaryKeys) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	// Clean up duplicate commits - keep only one record per (connection_id, repo_id, commit_sha)
	err := db.Exec(`
		DELETE c1 FROM _tool_codecov_commits c1
		INNER JOIN _tool_codecov_commits c2
		WHERE c1.id > c2.id
		AND c1.connection_id = c2.connection_id
		AND c1.repo_id = c2.repo_id
		AND c1.commit_sha = c2.commit_sha
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate commits, continuing anyway")
	}

	// Clean up duplicate coverages - keep only one record per (connection_id, repo_id, flag_name, branch, commit_sha)
	err = db.Exec(`
		DELETE c1 FROM _tool_codecov_coverages c1
		INNER JOIN _tool_codecov_coverages c2
		WHERE c1.id > c2.id
		AND c1.connection_id = c2.connection_id
		AND c1.repo_id = c2.repo_id
		AND c1.flag_name = c2.flag_name
		AND c1.branch = c2.branch
		AND c1.commit_sha = c2.commit_sha
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate coverages, continuing anyway")
	}

	// Clean up duplicate flags - keep only one record per (connection_id, repo_id, flag_name)
	err = db.Exec(`
		DELETE f1 FROM _tool_codecov_flags f1
		INNER JOIN _tool_codecov_flags f2
		WHERE f1.id > f2.id
		AND f1.connection_id = f2.connection_id
		AND f1.repo_id = f2.repo_id
		AND f1.flag_name = f2.flag_name
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate flags, continuing anyway")
	}

	// Clean up duplicate commit coverages - keep only one record per (connection_id, repo_id, commit_sha)
	err = db.Exec(`
		DELETE c1 FROM _tool_codecov_commit_coverages c1
		INNER JOIN _tool_codecov_commit_coverages c2
		WHERE c1.id > c2.id
		AND c1.connection_id = c2.connection_id
		AND c1.repo_id = c2.repo_id
		AND c1.commit_sha = c2.commit_sha
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate commit coverages, continuing anyway")
	}

	// Clean up duplicate comparisons - keep only one record per (connection_id, repo_id, commit_sha, flag_name)
	err = db.Exec(`
		DELETE c1 FROM _tool_codecov_comparisons c1
		INNER JOIN _tool_codecov_comparisons c2
		WHERE c1.id > c2.id
		AND c1.connection_id = c2.connection_id
		AND c1.repo_id = c2.repo_id
		AND c1.commit_sha = c2.commit_sha
		AND c1.flag_name = c2.flag_name
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate comparisons, continuing anyway")
	}

	// Clean up duplicate coverage trends - keep only one record per (connection_id, repo_id, flag_name, branch, date)
	err = db.Exec(`
		DELETE t1 FROM _tool_codecov_coverage_trends t1
		INNER JOIN _tool_codecov_coverage_trends t2
		WHERE t1.id > t2.id
		AND t1.connection_id = t2.connection_id
		AND t1.repo_id = t2.repo_id
		AND t1.flag_name = t2.flag_name
		AND t1.branch = t2.branch
		AND t1.date = t2.date
	`)
	if err != nil {
		basicRes.GetLogger().Warn(err, "failed to clean duplicate coverage trends, continuing anyway")
	}

	// Now drop the id column from tables that should use composite primary keys
	// Note: GORM will recreate these tables with proper structure on next sync
	tables := []string{
		"_tool_codecov_commits",
		"_tool_codecov_coverages",
		"_tool_codecov_flags",
		"_tool_codecov_commit_coverages",
		"_tool_codecov_comparisons",
		"_tool_codecov_coverage_trends",
	}

	for _, table := range tables {
		// Check if id column exists before trying to drop it
		err = db.Exec(`ALTER TABLE ` + table + ` DROP COLUMN IF EXISTS id`)
		if err != nil {
			basicRes.GetLogger().Warn(err, "failed to drop id column from %s, table may not exist or column already dropped", table)
		}
	}

	return nil
}

func (*fixPrimaryKeys) Version() uint64 {
	return 20251206000000
}

func (*fixPrimaryKeys) Name() string {
	return "Codecov fix primary keys - remove duplicates and drop id column"
}

