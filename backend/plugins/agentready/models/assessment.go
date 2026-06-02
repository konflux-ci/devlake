package models

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

type AgentReadyAssessment struct {
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

func (AgentReadyAssessment) TableName() string {
	return "_tool_agentready_assessments"
}

const (
	CertPlatinum         = "Platinum"
	CertGold             = "Gold"
	CertSilver           = "Silver"
	CertBronze           = "Bronze"
	CertNeedsImprovement = "Needs Improvement"
	CertNone             = "None"
)
