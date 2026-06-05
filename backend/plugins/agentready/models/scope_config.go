package models

import (
	"github.com/apache/incubator-devlake/core/models/common"
)

type AgentReadyScopeConfig struct {
	common.ScopeConfig `mapstructure:",squash" json:",inline" gorm:"embedded"`
}

func (AgentReadyScopeConfig) TableName() string {
	return "_tool_agentready_scope_configs"
}
