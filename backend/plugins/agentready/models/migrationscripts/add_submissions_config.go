package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addSubmissionsConfig)(nil)

type addSubmissionsConfig struct{}

type agentReadyScopeConfig20260602 struct {
	SubmissionsRepo         string `gorm:"type:varchar(500)"`
	SubmissionsPath         string `gorm:"type:varchar(500)"`
	SubmissionsBranch       string `gorm:"type:varchar(255)"`
	SubmissionsConnectionId uint64
}

func (agentReadyScopeConfig20260602) TableName() string {
	return "_tool_agentready_scope_configs"
}

func (script *addSubmissionsConfig) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	return db.AutoMigrate(&agentReadyScopeConfig20260602{})
}

func (script *addSubmissionsConfig) Version() uint64 {
	return 20260602000001
}

func (script *addSubmissionsConfig) Name() string {
	return "agentready add submissions config fields"
}
