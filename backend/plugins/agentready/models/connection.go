package models

import (
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

type AgentReadyConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	Project               string `mapstructure:"project" json:"project" validate:"required" gorm:"column:project;type:varchar(200)"`
	GitHubConnectionId    uint64 `mapstructure:"githubConnectionId" json:"githubConnectionId" gorm:"column:github_connection_id;not null"`
	SubmissionsRepo       string `mapstructure:"submissionsRepo" json:"submissionsRepo" validate:"required" gorm:"column:submissions_repo;type:varchar(255);not null"`
	SubmissionsPath       string `mapstructure:"submissionsPath" json:"submissionsPath" gorm:"column:submissions_path;type:varchar(255);default:submissions"`
	Branch                string `mapstructure:"branch" json:"branch" gorm:"column:branch;type:varchar(255)"`
}

func (AgentReadyConnection) TableName() string {
	return "_tool_agentready_connections"
}

func (c AgentReadyConnection) Sanitize() AgentReadyConnection {
	return c
}
