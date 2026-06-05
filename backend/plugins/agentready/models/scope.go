package models

import (
	"encoding/json"

	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

type AgentReadyScope struct {
	common.Scope `mapstructure:",squash"`
	FullName     string `gorm:"primaryKey;type:varchar(255)" json:"fullName" mapstructure:"fullName" validate:"required"`
	Name         string `gorm:"type:varchar(255)" json:"name" mapstructure:"name"`
	Id           string `gorm:"-" json:"id" mapstructure:"-"`
}

func (s AgentReadyScope) MarshalJSON() ([]byte, error) {
	type Alias AgentReadyScope
	alias := Alias(s)
	alias.Id = s.FullName
	return json.Marshal(alias)
}

func (AgentReadyScope) TableName() string {
	return "_tool_agentready_scopes"
}

func (s AgentReadyScope) ScopeId() string {
	return s.FullName
}

func (s AgentReadyScope) ScopeName() string {
	return s.Name
}

func (s AgentReadyScope) ScopeFullName() string {
	return s.FullName
}

func (s AgentReadyScope) ScopeParams() interface{} {
	return &AgentReadyApiParams{
		ConnectionId: s.ConnectionId,
		FullName:     s.FullName,
	}
}

type AgentReadyApiParams struct {
	ConnectionId uint64 `json:"connectionId"`
	FullName     string `json:"fullName"`
}

var _ plugin.ToolLayerScope = (*AgentReadyScope)(nil)
