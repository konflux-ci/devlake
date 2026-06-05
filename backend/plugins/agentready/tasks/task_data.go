package tasks

import (
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

type AgentReadyOptions struct {
	ConnectionId uint64                        `json:"connectionId"`
	FullName     string                        `json:"fullName"`
	ScopeConfig  *models.AgentReadyScopeConfig `json:"scopeConfig"`
}

type AgentReadyTaskData struct {
	Options    *AgentReadyOptions
	Connection *models.AgentReadyConnection
}
