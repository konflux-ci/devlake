package models

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

type AgentReadyMetric struct {
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

func (AgentReadyMetric) TableName() string {
	return "_tool_agentready_metrics"
}
