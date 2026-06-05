package models

import (
	"testing"
)

func TestConnectionTableName(t *testing.T) {
	conn := AgentReadyConnection{}
	want := "_tool_agentready_connections"
	if got := conn.TableName(); got != want {
		t.Errorf("TableName() = %q, want %q", got, want)
	}
}

func TestConnectionSanitize(t *testing.T) {
	conn := AgentReadyConnection{
		Project:            "my-project",
		GitHubConnectionId: 42,
		SubmissionsRepo:    "ambient-code/agentready",
		SubmissionsPath:    "submissions",
		Branch:             "main",
	}
	sanitized := conn.Sanitize()
	if sanitized.Project != "my-project" {
		t.Errorf("Sanitize() changed Project")
	}
	if sanitized.SubmissionsRepo != "ambient-code/agentready" {
		t.Errorf("Sanitize() changed SubmissionsRepo")
	}
}
