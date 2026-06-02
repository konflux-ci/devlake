package models

import (
	"github.com/apache/incubator-devlake/core/models/common"
)

type AgentReadyFinding struct {
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

func (AgentReadyFinding) TableName() string {
	return "_tool_agentready_findings"
}

const (
	FindingStatusPass          = "pass"
	FindingStatusFail          = "fail"
	FindingStatusSkipped       = "skipped"
	FindingStatusError         = "error"
	FindingStatusNotApplicable = "not_applicable"

	TierEssential = 1
	TierCritical  = 2
	TierImportant = 3
	TierAdvanced  = 4
)
