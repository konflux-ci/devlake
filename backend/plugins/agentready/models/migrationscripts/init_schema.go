package migrationscripts

import (
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
)

var _ plugin.MigrationScript = (*initSchema)(nil)

type initSchema struct{}

type agentReadyAssessment20260511 struct {
	common.NoPKModel
	Id                 string    `gorm:"primaryKey;type:varchar(255)"`
	RepoId             string    `gorm:"index;type:varchar(255)"`
	RepoName           string    `gorm:"type:varchar(255)"`
	ConnectionId       uint64    `gorm:"index"`
	Provider           string    `gorm:"type:varchar(50)"`
	SchemaVersion      string    `gorm:"type:varchar(20)"`
	OverallScore       float64   `gorm:"type:float"`
	CertificationLevel string    `gorm:"type:varchar(50)"`
	AttributesAssessed int       `gorm:"type:int"`
	AttributesTotal    int       `gorm:"type:int"`
	Branch             string    `gorm:"type:varchar(255)"`
	CommitHash         string    `gorm:"type:varchar(40)"`
	DurationSeconds    float64   `gorm:"type:float"`
	AssessedAt         time.Time `gorm:"index"`
	CollectedAt        time.Time
	RawJSON            string `gorm:"type:longtext"`
}

func (agentReadyAssessment20260511) TableName() string {
	return "_tool_agentready_assessments"
}

type agentReadyFinding20260511 struct {
	common.NoPKModel
	Id                 string   `gorm:"primaryKey;type:varchar(255)"`
	AssessmentId       string   `gorm:"index;type:varchar(255)"`
	RepoId             string   `gorm:"index;type:varchar(255)"`
	AttributeId        string   `gorm:"type:varchar(255)"`
	AttributeName      string   `gorm:"type:varchar(255)"`
	Category           string   `gorm:"type:varchar(255)"`
	Tier               int      `gorm:"type:int"`
	Status             string   `gorm:"type:varchar(50)"`
	Score              *float64 `gorm:"type:float"`
	MeasuredValue      string   `gorm:"type:text"`
	Threshold          string   `gorm:"type:text"`
	Evidence           string   `gorm:"type:text"`
	RemediationSummary string   `gorm:"type:text"`
	RemediationSteps   string   `gorm:"type:text"`
	DefaultWeight      float64  `gorm:"type:float"`
}

func (agentReadyFinding20260511) TableName() string {
	return "_tool_agentready_findings"
}

type agentReadyMetric20260511 struct {
	common.NoPKModel
	Id             string    `gorm:"primaryKey;type:varchar(255)"`
	RepoId         string    `gorm:"index;type:varchar(255)"`
	AssessedAt     time.Time `gorm:"index"`
	PassCount      int       `gorm:"type:int"`
	FailCount      int       `gorm:"type:int"`
	SkipCount      int       `gorm:"type:int"`
	Tier1PassRate  float64   `gorm:"type:float"`
	Tier2PassRate  float64   `gorm:"type:float"`
	Tier3PassRate  float64   `gorm:"type:float"`
	Tier4PassRate  float64   `gorm:"type:float"`
	CategoryScores string    `gorm:"type:text"`
}

func (agentReadyMetric20260511) TableName() string {
	return "_tool_agentready_metrics"
}

type agentReadyScopeConfig20260511 struct {
	common.ScopeConfig
	Branch             string `gorm:"type:varchar(255)"`
	AssessmentFilePath string `gorm:"type:varchar(500)"`
	ExcludeRepos       string `gorm:"type:text"`
}

func (agentReadyScopeConfig20260511) TableName() string {
	return "_tool_agentready_scope_configs"
}

func (script *initSchema) Up(basicRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		basicRes,
		&agentReadyAssessment20260511{},
		&agentReadyFinding20260511{},
		&agentReadyMetric20260511{},
		&agentReadyScopeConfig20260511{},
	)
}

func (script *initSchema) Version() uint64 {
	return 20260511000001
}

func (script *initSchema) Name() string {
	return "agentready init schema"
}
