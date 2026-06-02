package tasks

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

type AgentReadyOptions struct {
	ProjectName   string                        `json:"projectName"`
	RepoId        string                        `json:"repoId"`
	TimeAfter     string                        `json:"timeAfter"`
	Branch        string                        `json:"branch"`
	ScopeConfigId uint64                        `json:"scopeConfigId"`
	ScopeConfig   *models.AgentReadyScopeConfig `json:"scopeConfig"`
}

type AgentReadyTaskData struct {
	Options *AgentReadyOptions
}

type RepoInfo struct {
	DomainRepoId      string
	Provider          string
	ConnectionId      uint64
	FullName          string
	GitlabId          int
	PathWithNamespace string
	DefaultBranch     string
	Endpoint          string
	Token             string
}

func DecodeTaskOptions(options map[string]interface{}) (*AgentReadyOptions, errors.Error) {
	var op AgentReadyOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode agentready options")
	}
	return &op, nil
}

func ValidateTaskOptions(op *AgentReadyOptions) errors.Error {
	if op.RepoId == "" && op.ProjectName == "" {
		return errors.BadInput.New("either repoId or projectName is required")
	}
	return nil
}

// ParseDomainRepoId splits a domain repo ID in "provider:scopeType:connectionId:scopeId" format
// (e.g. "github:GithubRepo:1:12345"). The scopeType segment is unused.
func ParseDomainRepoId(repoId string) (provider string, connectionId uint64, scopeId string, err errors.Error) {
	parts := strings.SplitN(repoId, ":", 4)
	if len(parts) < 4 {
		return "", 0, "", errors.BadInput.New(fmt.Sprintf("invalid domain repo ID format: %s", repoId))
	}
	provider = parts[0]
	connId, parseErr := strconv.ParseUint(parts[2], 10, 64)
	if parseErr != nil {
		return "", 0, "", errors.BadInput.Wrap(parseErr, fmt.Sprintf("invalid connectionId in repo ID: %s", repoId))
	}
	return provider, connId, parts[3], nil
}
