package tasks

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseSubmissionEntries(t *testing.T) {
	tree := []githubTreeEntry{
		{Path: "submissions/org1/repo1/2026-02-07T14-42-31-assessment.json", Type: "blob"},
		{Path: "submissions/org1/repo1/2026-03-01T10-00-00-assessment.json", Type: "blob"},
		{Path: "submissions/org2/repoA/2026-01-15T09-00-00-assessment.json", Type: "blob"},
		{Path: "submissions/.trigger", Type: "blob"},
		{Path: "submissions/org1", Type: "tree"},
		{Path: "submissions/org1/repo1", Type: "tree"},
		{Path: "README.md", Type: "blob"},
		{Path: "submissions/org1/repo1/notes.txt", Type: "blob"},
		{Path: "submissions/deep/nested/too/far/file.json", Type: "blob"},
	}

	entries := ParseSubmissionEntries(tree, "submissions")

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First match
	if entries[0].Org != "org1" {
		t.Errorf("entries[0].Org = %q, want %q", entries[0].Org, "org1")
	}
	if entries[0].Repo != "repo1" {
		t.Errorf("entries[0].Repo = %q, want %q", entries[0].Repo, "repo1")
	}
	if entries[0].Filename != "2026-02-07T14-42-31-assessment.json" {
		t.Errorf("entries[0].Filename = %q, want %q", entries[0].Filename, "2026-02-07T14-42-31-assessment.json")
	}
	if entries[0].TreePath != "submissions/org1/repo1/2026-02-07T14-42-31-assessment.json" {
		t.Errorf("entries[0].TreePath = %q, want %q", entries[0].TreePath, "submissions/org1/repo1/2026-02-07T14-42-31-assessment.json")
	}

	// Second match
	if entries[1].Org != "org1" {
		t.Errorf("entries[1].Org = %q, want %q", entries[1].Org, "org1")
	}
	if entries[1].Repo != "repo1" {
		t.Errorf("entries[1].Repo = %q, want %q", entries[1].Repo, "repo1")
	}
	if entries[1].Filename != "2026-03-01T10-00-00-assessment.json" {
		t.Errorf("entries[1].Filename = %q, want %q", entries[1].Filename, "2026-03-01T10-00-00-assessment.json")
	}
	if entries[1].TreePath != "submissions/org1/repo1/2026-03-01T10-00-00-assessment.json" {
		t.Errorf("entries[1].TreePath = %q, want %q", entries[1].TreePath, "submissions/org1/repo1/2026-03-01T10-00-00-assessment.json")
	}

	// Third match
	if entries[2].Org != "org2" {
		t.Errorf("entries[2].Org = %q, want %q", entries[2].Org, "org2")
	}
	if entries[2].Repo != "repoA" {
		t.Errorf("entries[2].Repo = %q, want %q", entries[2].Repo, "repoA")
	}
	if entries[2].Filename != "2026-01-15T09-00-00-assessment.json" {
		t.Errorf("entries[2].Filename = %q, want %q", entries[2].Filename, "2026-01-15T09-00-00-assessment.json")
	}
	if entries[2].TreePath != "submissions/org2/repoA/2026-01-15T09-00-00-assessment.json" {
		t.Errorf("entries[2].TreePath = %q, want %q", entries[2].TreePath, "submissions/org2/repoA/2026-01-15T09-00-00-assessment.json")
	}
}

func TestParseSubmissionEntries_CustomPath(t *testing.T) {
	tree := []githubTreeEntry{
		{Path: "data/assessments/org1/repo1/test.json", Type: "blob"},
		{Path: "submissions/org1/repo1/test.json", Type: "blob"},
	}

	entries := ParseSubmissionEntries(tree, "data/assessments")

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Org != "org1" {
		t.Errorf("Org = %q, want %q", entries[0].Org, "org1")
	}
	if entries[0].Repo != "repo1" {
		t.Errorf("Repo = %q, want %q", entries[0].Repo, "repo1")
	}
	if entries[0].TreePath != "data/assessments/org1/repo1/test.json" {
		t.Errorf("TreePath = %q, want %q", entries[0].TreePath, "data/assessments/org1/repo1/test.json")
	}
}

func TestParseSubmissionEntries_Empty(t *testing.T) {
	entries := ParseSubmissionEntries(nil, "submissions")

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestMakeSubmissionsRepoId(t *testing.T) {
	got := MakeSubmissionsRepoId("Red-Hat-AI-Innovation-Team", "sdg_hub")
	want := "submissions:Red-Hat-AI-Innovation-Team/sdg_hub"
	if got != want {
		t.Errorf("MakeSubmissionsRepoId() = %q, want %q", got, want)
	}
}

func TestFetchGithubTree(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/trees/main" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("recursive") != "1" {
			t.Errorf("expected recursive=1, got %s", r.URL.Query().Get("recursive"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}

		resp := githubTreeResponse{
			SHA:       "abc123",
			Truncated: false,
			Tree: []githubTreeEntry{
				{Path: "submissions/org1/repo1/test.json", Mode: "100644", Type: "blob", SHA: "def456", Size: 1024},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "main", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SHA != "abc123" {
		t.Errorf("SHA = %q, want %q", result.SHA, "abc123")
	}
	if len(result.Tree) != 1 {
		t.Fatalf("expected 1 tree entry, got %d", len(result.Tree))
	}
	if result.Tree[0].Path != "submissions/org1/repo1/test.json" {
		t.Errorf("Tree[0].Path = %q, want %q", result.Tree[0].Path, "submissions/org1/repo1/test.json")
	}
}

func TestFetchGithubTree_NoToken(t *testing.T) {
	_, err := FetchGithubTree(context.Background(), "https://api.github.com", "owner/repo", "main", "")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestFetchGithubTree_DefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/trees/main" {
			t.Errorf("expected path /repos/owner/repo/git/trees/main, got %s", r.URL.Path)
		}

		resp := githubTreeResponse{SHA: "abc", Tree: []githubTreeEntry{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	_, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchGithubTree_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	_, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "main", "test-token")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected error containing '403', got %v", err)
	}
}

func TestFetchGithubTree_BranchWithSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/repos/owner/repo/git/trees/feature%2Fsubmissions"
		if r.URL.RawPath != expectedPath {
			t.Errorf("expected RawPath %q, got %q", expectedPath, r.URL.RawPath)
		}
		resp := githubTreeResponse{SHA: "abc", Tree: []githubTreeEntry{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	_, err := FetchGithubTree(context.Background(), server.URL, "owner/repo", "feature/submissions", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchGithubAssessment_CustomBranch(t *testing.T) {
	assessmentJSON := `{"overall_score": 70.0}`
	encoded := base64.StdEncoding.EncodeToString([]byte(assessmentJSON))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ref := r.URL.Query().Get("ref"); ref != "test" {
			t.Errorf("expected ref=test, got ref=%s", ref)
		}
		resp := map[string]interface{}{
			"content":  encoded,
			"encoding": "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubAssessment(context.Background(), server.URL, "org/submissions-repo", "submissions/myorg/myrepo/assessment.json", "test", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}
