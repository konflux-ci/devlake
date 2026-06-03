package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/plugin"
)

func All() []plugin.MigrationScript {
	return []plugin.MigrationScript{
		&initSchema{},
		&addSubmissionsConfig{},
		&addSubmissionsBranch{},
	}
}
