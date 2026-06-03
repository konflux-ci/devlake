package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

const maxTreeResponseSize = 5 << 20 // 5 MB

// SubmissionEntry represents a single assessment file discovered in the
// submissions directory tree.
type SubmissionEntry struct {
	Org      string
	Repo     string
	Filename string
	TreePath string // full path like "submissions/org/repo/file.json"
}

type githubTreeResponse struct {
	SHA       string            `json:"sha"`
	Tree      []githubTreeEntry `json:"tree"`
	Truncated bool              `json:"truncated"`
}

type githubTreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int    `json:"size"`
}

// ParseSubmissionEntries filters tree entries to find assessment JSON files
// matching the expected {submissionsPath}/{org}/{repo}/{filename}.json structure.
func ParseSubmissionEntries(tree []githubTreeEntry, submissionsPath string) []SubmissionEntry {
	prefix := submissionsPath + "/"
	var entries []SubmissionEntry

	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		if !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		if !strings.HasSuffix(entry.Path, ".json") {
			continue
		}

		// Strip prefix to get "org/repo/filename.json"; reject deeper nesting
		relPath := strings.TrimPrefix(entry.Path, prefix)
		parts := strings.SplitN(relPath, "/", 4)
		if len(parts) != 3 {
			continue
		}

		entries = append(entries, SubmissionEntry{
			Org:      parts[0],
			Repo:     parts[1],
			Filename: parts[2],
			TreePath: entry.Path,
		})
	}

	return entries
}

// FetchGithubTree fetches the full recursive tree for a branch via the
// GitHub Git Trees API.
func FetchGithubTree(ctx context.Context, endpoint, fullName, branch, token string) (*githubTreeResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("a GitHub token is required to fetch the tree")
	}
	if branch == "" {
		branch = "main"
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", endpoint, fullName, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating tree request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching tree from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return nil, fmt.Errorf("GitHub Trees API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTreeResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading tree response: %w", err)
	}
	if len(body) > maxTreeResponseSize {
		return nil, fmt.Errorf("tree response exceeds %d bytes limit", maxTreeResponseSize)
	}

	var treeResp githubTreeResponse
	if err := json.Unmarshal(body, &treeResp); err != nil {
		return nil, fmt.Errorf("decoding tree response: %w", err)
	}

	return &treeResp, nil
}

// MakeSubmissionsRepoId creates a synthetic repo ID for submissions-sourced
// assessments in the format "submissions:{org}/{repo}".
func MakeSubmissionsRepoId(org, repo string) string {
	return fmt.Sprintf("submissions:%s/%s", org, repo)
}

// collectFromSubmissionsRepo fetches all assessment JSON files from a
// centralized submissions repository and stores them as assessments.
func collectFromSubmissionsRepo(ctx context.Context, db dal.Dal, logger log.Logger, taskCtx plugin.SubTaskContext, config *models.AgentReadyScopeConfig, projectName string) {
	submissionsPath := config.SubmissionsPath
	if submissionsPath == "" {
		submissionsPath = models.DefaultSubmissionsPath
	}

	// Look up the GitHub connection for authentication
	var conn githubConn
	dbErr := db.First(&conn, dal.Where("id = ?", config.SubmissionsConnectionId))
	if dbErr != nil {
		logger.Warn(dbErr, "GitHub connection %d not found for submissions repo", config.SubmissionsConnectionId)
		return
	}

	endpoint := conn.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com"
	}

	branch := config.SubmissionsBranch

	// Fetch the full recursive tree
	treeResp, err := FetchGithubTree(ctx, endpoint, config.SubmissionsRepo, branch, conn.Token)
	if err != nil {
		logger.Warn(nil, "Failed to fetch tree for submissions repo %s: %v", config.SubmissionsRepo, err)
		return
	}

	if treeResp.Truncated {
		logger.Warn(nil, "Submissions repo tree is truncated; some entries may be missing")
	}

	// Parse entries from the tree
	entries := ParseSubmissionEntries(treeResp.Tree, submissionsPath)
	logger.Info("Found %d submission entries in %s", len(entries), config.SubmissionsRepo)

	// Register submission repos in project_mapping so Grafana dashboards
	// (which JOIN on project_mapping) can find them.
	if projectName != "" {
		seen := map[string]bool{}
		for _, entry := range entries {
			repoId := MakeSubmissionsRepoId(entry.Org, entry.Repo)
			if seen[repoId] {
				continue
			}
			seen[repoId] = true
			mapping := &projectMappingRow{
				ProjectName: projectName,
				Table:       "repos",
				RowId:       repoId,
			}
			if mapErr := db.CreateOrUpdate(mapping); mapErr != nil {
				logger.Warn(mapErr, "Failed to insert project_mapping for %s", repoId)
			}
		}
	}

	now := time.Now()
	for _, entry := range entries {
		// Fetch the assessment file content via the Contents API
		rawJSON, fetchErr := FetchGithubAssessment(ctx, endpoint, config.SubmissionsRepo, entry.TreePath, "", conn.Token)
		if fetchErr != nil {
			logger.Warn(nil, "Failed to fetch submission %s: %v", entry.TreePath, fetchErr)
			taskCtx.IncProgress(1)
			continue
		}
		if rawJSON == "" {
			logger.Info("Empty submission file %s, skipping", entry.TreePath)
			taskCtx.IncProgress(1)
			continue
		}

		// Parse partial JSON to extract commit hash
		var partial collectorAssessmentJSON
		if jsonErr := json.Unmarshal([]byte(rawJSON), &partial); jsonErr != nil {
			logger.Warn(nil, "Failed to parse submission JSON %s: %v", entry.TreePath, jsonErr)
			taskCtx.IncProgress(1)
			continue
		}

		commitHash := partial.Repository.CommitHash
		if commitHash == "" {
			logger.Warn(nil, "Submission %s has no commit_hash, skipping", entry.TreePath)
			taskCtx.IncProgress(1)
			continue
		}

		repoId := MakeSubmissionsRepoId(entry.Org, entry.Repo)
		assessment := &models.AgentReadyAssessment{
			Id:           fmt.Sprintf("%s:%s", repoId, commitHash),
			RepoId:       repoId,
			RepoName:     fmt.Sprintf("%s/%s", entry.Org, entry.Repo),
			ConnectionId: config.SubmissionsConnectionId,
			Provider:     "submissions",
			CollectedAt:  now,
			RawJSON:      rawJSON,
		}

		if saveErr := db.CreateOrUpdate(assessment); saveErr != nil {
			logger.Warn(saveErr, "Failed to save submission assessment for %s", entry.TreePath)
		}
		taskCtx.IncProgress(1)
	}
}
