package models

import (
	"github.com/apache/incubator-devlake/core/models/common"
)

type AgentReadyScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`

	Branch             string `mapstructure:"branch" json:"branch" gorm:"type:varchar(255)"`
	AssessmentFilePath string `mapstructure:"assessmentFilePath" json:"assessmentFilePath" gorm:"type:varchar(500)"`
	ExcludeRepos       string `mapstructure:"excludeRepos" json:"excludeRepos" gorm:"type:text"`
}

func (AgentReadyScopeConfig) TableName() string {
	return "_tool_agentready_scope_configs"
}

const DefaultAssessmentFilePath = ".agentready/assessment-latest.json"

func GetDefaultScopeConfig() *AgentReadyScopeConfig {
	return &AgentReadyScopeConfig{
		AssessmentFilePath: DefaultAssessmentFilePath,
	}
}
