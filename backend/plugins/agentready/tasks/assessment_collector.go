package tasks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/agentready/models"
)

var CollectAssessmentsMeta = plugin.SubTaskMeta{
	Name:             "collectAssessments",
	EntryPoint:       CollectAssessments,
	EnabledByDefault: true,
	Description:      "Fetch assessment JSON files from connected GitHub/GitLab repositories",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

type githubConn struct {
	ID       uint64 `gorm:"primaryKey;column:id"`
	Endpoint string `gorm:"column:endpoint"`
	Token    string `gorm:"column:token;serializer:encdec"`
}

func (githubConn) TableName() string { return "_tool_github_connections" }

type gitlabConn struct {
	ID       uint64 `gorm:"primaryKey;column:id"`
	Endpoint string `gorm:"column:endpoint"`
	Token    string `gorm:"column:token;serializer:encdec"`
}

func (gitlabConn) TableName() string { return "_tool_gitlab_connections" }

type githubRepoRow struct {
	ConnectionId uint64 `gorm:"column:connection_id"`
	GithubId     int    `gorm:"column:github_id"`
	FullName     string `gorm:"column:full_name"`
}

func (githubRepoRow) TableName() string { return "_tool_github_repos" }

type gitlabProjectRow struct {
	ConnectionId      uint64 `gorm:"column:connection_id"`
	GitlabId          int    `gorm:"column:gitlab_id"`
	PathWithNamespace string `gorm:"column:path_with_namespace"`
	DefaultBranch     string `gorm:"column:default_branch"`
}

func (gitlabProjectRow) TableName() string { return "_tool_gitlab_projects" }

type projectMappingRow struct {
	ProjectName string `gorm:"column:project_name"`
	Table       string `gorm:"column:table"`
	RowId       string `gorm:"column:row_id"`
}

func (projectMappingRow) TableName() string { return "project_mapping" }

type collectorAssessmentJSON struct {
	Repository struct {
		CommitHash string `json:"commit_hash"`
	} `json:"repository"`
}

func CollectAssessments(taskCtx plugin.SubTaskContext) errors.Error {
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()
	data := taskCtx.GetData().(*AgentReadyTaskData)
	config := data.Options.ScopeConfig
	if config == nil {
		config = models.GetDefaultScopeConfig()
	}

	filePath := config.AssessmentFilePath
	if filePath == "" {
		filePath = models.DefaultAssessmentFilePath
	}

	repos, err := discoverRepos(db, data.Options, logger)
	if err != nil {
		return err
	}

	logger.Info("Discovered %d repos for agentready collection", len(repos))
	taskCtx.SetProgress(0, len(repos))

	now := time.Now()
	for _, repo := range repos {
		var rawJSON string
		var fetchErr error

		branch := data.Options.Branch
		if branch == "" && config.Branch != "" {
			branch = config.Branch
		}
		if branch == "" {
			branch = repo.DefaultBranch
		}

		switch repo.Provider {
		case "github":
			endpoint := repo.Endpoint
			if endpoint == "" {
				endpoint = "https://api.github.com"
			}
			rawJSON, fetchErr = FetchGithubAssessment(endpoint, repo.FullName, filePath, branch, repo.Token)
		case "gitlab":
			endpoint := repo.Endpoint
			if endpoint == "" {
				endpoint = "https://gitlab.com"
			}
			rawJSON, fetchErr = FetchGitlabAssessment(endpoint, repo.GitlabId, filePath, branch, repo.Token)
		default:
			logger.Warn(nil, "Unsupported provider %s for repo %s, skipping", repo.Provider, repo.DomainRepoId)
			taskCtx.IncProgress(1)
			continue
		}

		if fetchErr != nil {
			logger.Warn(nil, "Failed to fetch assessment for repo %s: %v", repo.DomainRepoId, fetchErr)
			taskCtx.IncProgress(1)
			continue
		}
		if rawJSON == "" {
			logger.Info("No assessment file found for repo %s, skipping", repo.DomainRepoId)
			taskCtx.IncProgress(1)
			continue
		}
		var partial collectorAssessmentJSON
		if jsonErr := json.Unmarshal([]byte(rawJSON), &partial); jsonErr != nil {
			logger.Warn(nil, "Failed to parse assessment JSON for repo %s: %v", repo.DomainRepoId, jsonErr)
			taskCtx.IncProgress(1)
			continue
		}

		commitHash := partial.Repository.CommitHash
		if commitHash == "" {
			logger.Warn(nil, "Assessment for repo %s has no commit_hash, skipping", repo.DomainRepoId)
			taskCtx.IncProgress(1)
			continue
		}

		assessment := &models.AgentReadyAssessment{
			Id:           fmt.Sprintf("%s:%s", repo.DomainRepoId, commitHash),
			RepoId:       repo.DomainRepoId,
			ConnectionId: repo.ConnectionId,
			Provider:     repo.Provider,
			CollectedAt:  now,
			RawJSON:      rawJSON,
		}

		if repo.FullName != "" {
			assessment.RepoName = repo.FullName
		} else {
			assessment.RepoName = repo.PathWithNamespace
		}

		dbErr := db.CreateOrUpdate(assessment)
		if dbErr != nil {
			logger.Warn(dbErr, "Failed to save raw assessment for repo %s", repo.DomainRepoId)
		}
		taskCtx.IncProgress(1)
	}

	return nil
}

func discoverRepos(db dal.Dal, options *AgentReadyOptions, logger log.Logger) ([]*RepoInfo, errors.Error) {
	var repoIds []string

	if options.ProjectName != "" {
		var mappings []projectMappingRow
		err := db.All(&mappings,
			dal.From(&projectMappingRow{}),
			dal.Where("project_name = ? AND `table` = ?", options.ProjectName, "repos"),
		)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to query project_mapping")
		}
		for _, m := range mappings {
			repoIds = append(repoIds, m.RowId)
		}
	} else {
		repoIds = []string{options.RepoId}
	}

	var repos []*RepoInfo
	for _, repoId := range repoIds {
		provider, connId, scopeId, err := ParseDomainRepoId(repoId)
		if err != nil {
			logger.Warn(err, "Skipping unparseable repo ID: %s", repoId)
			continue
		}

		info := &RepoInfo{
			DomainRepoId: repoId,
			Provider:     provider,
			ConnectionId: connId,
		}

		switch provider {
		case "github":
			scopeIdInt, parseErr := strconv.Atoi(scopeId)
			if parseErr != nil {
				logger.Warn(nil, "Invalid GitHub scope ID %s in repo %s", scopeId, repoId)
				continue
			}
			var repo githubRepoRow
			dbErr := db.First(&repo, dal.Where("connection_id = ? AND github_id = ?", connId, scopeIdInt))
			if dbErr != nil {
				logger.Warn(dbErr, "GitHub repo not found for connection=%d github_id=%d", connId, scopeIdInt)
				continue
			}
			info.FullName = repo.FullName

			var conn githubConn
			dbErr = db.First(&conn, dal.Where("id = ?", connId))
			if dbErr != nil {
				logger.Warn(dbErr, "GitHub connection %d not found", connId)
				continue
			}
			info.Endpoint = conn.Endpoint
			info.Token = conn.Token

		case "gitlab":
			scopeIdInt, parseErr := strconv.Atoi(scopeId)
			if parseErr != nil {
				logger.Warn(nil, "Invalid GitLab scope ID %s in repo %s", scopeId, repoId)
				continue
			}
			var project gitlabProjectRow
			dbErr := db.First(&project, dal.Where("connection_id = ? AND gitlab_id = ?", connId, scopeIdInt))
			if dbErr != nil {
				logger.Warn(dbErr, "GitLab project not found for connection=%d gitlab_id=%d", connId, scopeIdInt)
				continue
			}
			info.GitlabId = project.GitlabId
			info.PathWithNamespace = project.PathWithNamespace
			info.DefaultBranch = project.DefaultBranch

			var conn gitlabConn
			dbErr = db.First(&conn, dal.Where("id = ?", connId))
			if dbErr != nil {
				logger.Warn(dbErr, "GitLab connection %d not found", connId)
				continue
			}
			info.Endpoint = conn.Endpoint
			info.Token = conn.Token

		default:
			logger.Warn(nil, "Unsupported provider %s for repo %s", provider, repoId)
			continue
		}

		repos = append(repos, info)
	}

	return repos, nil
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// FetchGithubAssessment fetches an assessment file via the GitHub Contents API.
// The filePath is typically a symlink (e.g. assessment-latest.json -> assessment-<timestamp>.json);
// the Contents API resolves symlinks automatically and returns the target file's content.
func FetchGithubAssessment(endpoint, fullName, filePath, branch, token string) (string, error) {
	endpoint = strings.TrimSuffix(endpoint, "/")
	apiURL := fmt.Sprintf("%s/repos/%s/contents/%s", endpoint, fullName, filePath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	if branch != "" {
		q := req.URL.Query()
		q.Set("ref", branch)
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding GitHub response: %w", err)
	}

	if result.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding: %s", result.Encoding)
	}

	cleaned := strings.ReplaceAll(result.Content, "\n", "")
	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return "", fmt.Errorf("decoding base64 content: %w", err)
	}

	return string(decoded), nil
}

func fetchGitlabRawFile(endpoint string, projectId int, filePath, branch, token string) (string, error) {
	endpoint = strings.TrimSuffix(endpoint, "/")
	encodedPath := url.PathEscape(filePath)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/files/%s/raw", endpoint, projectId, encodedPath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	if branch != "" {
		q := req.URL.Query()
		q.Set("ref", branch)
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("Private-Token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching from GitLab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("GitLab API returned %d: %s", resp.StatusCode, string(body))
	}

	const maxAssessmentSize = 10 << 20 // 10 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAssessmentSize))
	if err != nil {
		return "", fmt.Errorf("reading GitLab response: %w", err)
	}

	return string(body), nil
}

// FetchGitlabAssessment fetches an assessment file via the Gitlab Repository Files API.
// The filePath is typically a symlink (e.g. assessment-latest.json -> assessment-<timestamp>.json);
// the Gitlab Repository Files API does not resolve symlinks automatically.
// To resolve this, multiple hops will be made (max 5) to find the actual assessment file.
func FetchGitlabAssessment(endpoint string, projectId int, filePath, branch, token string) (string, error) {
	// In theory there should only be one hop from assessment-latest.json to the actual file.
	const maxHops = 2
	content, err := fetchGitlabRawFile(endpoint, projectId, filePath, branch, token)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Gitlab assessment: %w", err)
	}
	// No assessment file found.
	if content == "" {
		return "", nil
	}
	return resolveGitlabSymlink(endpoint, projectId, filePath, branch, token, content, maxHops)
}

// looksLikeSymlinkTarget validates if a string looks like a symlink
func looksLikeSymlinkTarget(s string) bool {
	// Empty
	if s == "" {
		return false
	}
	// Too many characters
	if len(s) > 500 {
		return false
	}
	// Null bytes
	if strings.Contains(s, "\x00") {
		return false
	}
	// Newlines
	if strings.Contains(s, "\n") {
		return false
	}
	// Carriage return
	if strings.Contains(s, "\r") {
		return false
	}
	// JSON characters
	if strings.ContainsAny(s, "{}:,[]\"'\\") {
		return false
	}
	// Doesn't end with '.json'
	if !strings.HasSuffix(s, ".json") {
		return false
	}

	// Should be symlink to another json file containing latest
	// assessment json.
	return true
}

// resolveGitlabSymlink detects if rawContent is a symlink target path
// and follows it to fetch the actual file. Returns the content as-is
// if it's already valid JSON.
func resolveGitlabSymlink(endpoint string, projectId int, originalFilePath, branch, token, rawContent string, maxHops int) (string, error) {
	content := rawContent
	// ".agentready/assessment-latest.json" -> ".agentready"
	currentDir := path.Dir(originalFilePath)
	for i := 0; i < maxHops; i++ {
		trimmed := strings.TrimSpace(content)

		// Already valid JSON, done
		// This is the actual assessment file
		if json.Valid([]byte(trimmed)) {
			return trimmed, nil
		}

		// Not JSON, but doesn't look like a symlink target either
		// Return as-is and let the caller's json.Unmarshal produce the error
		if !looksLikeSymlinkTarget(trimmed) {
			return content, nil
		}

		// Resolve relative to current directory:
		// path.Join(".agentready", "assessment-20260512.json")
		// new path -> ".agentready/assessment-20260512.json"
		// path.Clean handles "../" normalization
		resolvedPath := path.Clean(path.Join(currentDir, trimmed))

		// Update currentDir for potential next hop
		currentDir = path.Dir(resolvedPath)

		// Fetch the resolved target
		// Should only be one more hop
		fetched, err := fetchGitlabRawFile(endpoint, projectId, resolvedPath, branch, token)
		if err != nil {
			return "", fmt.Errorf("following symlink %q -> %q: %w", originalFilePath, resolvedPath, err)
		}
		if fetched == "" {
			return "", fmt.Errorf("symlink target %q not found (404)", resolvedPath)
		}

		content = fetched
	}
	return "", fmt.Errorf("too many symlink hops (max %d) starting from %q", maxHops, originalFilePath)
}
