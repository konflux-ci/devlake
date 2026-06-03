package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addSubmissionsBranch)(nil)

type addSubmissionsBranch struct{}

type agentReadyScopeConfig20260603 struct {
	SubmissionsBranch string `gorm:"type:varchar(255)"`
}

func (agentReadyScopeConfig20260603) TableName() string {
	return "_tool_agentready_scope_configs"
}

func (script *addSubmissionsBranch) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&agentReadyScopeConfig20260603{})
}

func (script *addSubmissionsBranch) Version() uint64 {
	return 20260603000001
}

func (script *addSubmissionsBranch) Name() string {
	return "agentready add submissions_branch column"
}
