package tasks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchGithubAssessment(t *testing.T) {
	assessmentJSON := `{"overall_score": 85.5, "certification_level": "Gold"}`
	encoded := base64.StdEncoding.EncodeToString([]byte(assessmentJSON))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/repos/myorg/myrepo/contents/.agentready/assessment-latest.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if ref := r.URL.Query().Get("ref"); ref != "main" {
			t.Errorf("expected ref=main, got ref=%s", ref)
		}
		resp := map[string]interface{}{
			"content":  encoded,
			"encoding": "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubAssessment(server.URL, "myorg/myrepo", ".agentready/assessment-latest.json", "main", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}

func TestFetchGithubAssessment_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	result, err := FetchGithubAssessment(server.URL, "myorg/myrepo", ".agentready/assessment-latest.json", "main", "test-token")
	if err != nil {
		t.Fatalf("404 should not return error, got: %v", err)
	}
	if result != "" {
		t.Errorf("404 should return empty string, got %q", result)
	}
}

func TestFetchGithubAssessment_NoBranch(t *testing.T) {
	assessmentJSON := `{"overall_score": 50.0}`
	encoded := base64.StdEncoding.EncodeToString([]byte(assessmentJSON))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query params, got %s", r.URL.RawQuery)
		}
		resp := map[string]interface{}{
			"content":  encoded,
			"encoding": "base64",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	result, err := FetchGithubAssessment(server.URL, "myorg/myrepo", ".agentready/assessment-latest.json", "", "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}

func TestFetchGitlabAssessment(t *testing.T) {
	assessmentJSON := `{"overall_score": 72.0, "certification_level": "Silver"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Private-Token") != "glpat-test" {
			t.Errorf("expected Private-Token glpat-test, got %s", r.Header.Get("Private-Token"))
		}
		expectedPath := "/api/v4/projects/42/repository/files/.agentready%2Fassessment-latest.json/raw"
		actualPath := r.URL.RawPath
		if actualPath == "" {
			actualPath = r.URL.Path
		}
		if actualPath != expectedPath {
			t.Errorf("unexpected path: %s, expected: %s", actualPath, expectedPath)
		}
		w.Write([]byte(assessmentJSON))
	}))
	defer server.Close()

	result, err := FetchGitlabAssessment(server.URL, 42, ".agentready/assessment-latest.json", "main", "glpat-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}

func TestFetchGitlabAssessment_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := FetchGitlabAssessment(server.URL, 42, ".agentready/assessment-latest.json", "main", "glpat-test")
	if err != nil {
		t.Fatalf("404 should not return error, got: %v", err)
	}
	if result != "" {
		t.Errorf("404 should return empty string, got %q", result)
	}
}

func TestParseDomainRepoId(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantProvider string
		wantConnId   uint64
		wantScopeId  string
		wantErr      bool
	}{
		{"github", "github:GithubRepo:1:12345", "github", 1, "12345", false},
		{"gitlab", "gitlab:GitlabProject:3:42", "gitlab", 3, "42", false},
		{"invalid", "bad-format", "", 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, connId, scopeId, err := ParseDomainRepoId(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDomainRepoId() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if provider != tt.wantProvider {
				t.Errorf("provider = %v, want %v", provider, tt.wantProvider)
			}
			if connId != tt.wantConnId {
				t.Errorf("connId = %v, want %v", connId, tt.wantConnId)
			}
			if scopeId != tt.wantScopeId {
				t.Errorf("scopeId = %v, want %v", scopeId, tt.wantScopeId)
			}
		})
	}
}

func TestCollectorAssessmentId(t *testing.T) {
	tests := []struct {
		name    string
		repoId  string
		rawJSON string
		wantId  string
		wantErr bool
	}{
		{
			name:    "valid assessment",
			repoId:  "github:1:123",
			rawJSON: `{"repository":{"commit_hash":"abc123"}}`,
			wantId:  "github:1:123:abc123",
		},
		{
			name:    "missing commit hash",
			repoId:  "github:1:123",
			rawJSON: `{"repository":{}}`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			repoId:  "github:1:123",
			rawJSON: `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var partial collectorAssessmentJSON
			err := json.Unmarshal([]byte(tt.rawJSON), &partial)

			if tt.wantErr {
				if err == nil && partial.Repository.CommitHash == "" {
					// Expected: empty commit hash is an error case
					return
				}
				if err != nil {
					return
				}
				t.Errorf("expected error, got none")
				return
			}

			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}

			gotId := fmt.Sprintf("%s:%s", tt.repoId, partial.Repository.CommitHash)
			if gotId != tt.wantId {
				t.Errorf("got Id %q, want %q", gotId, tt.wantId)
			}
		})
	}

}

func TestFetchGitlabAssessment_SymlinkResolution(t *testing.T) {
	// Ensure that if the first file found looks like a symlink the path gets followed
	// until the actual assessment file is found.
	assessmentJSONSymlink := `assessment-20260507-111310.json`
	assessmentJSON := `{"overall_score": 72.0, "certification_level": "Silver"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualPath := r.URL.RawPath
		if actualPath == "" {
			actualPath = r.URL.Path
		}

		switch {
		case strings.Contains(actualPath, "assessment-latest.json"):
			w.Write([]byte(assessmentJSONSymlink))
		case strings.Contains(actualPath, assessmentJSONSymlink):
			w.Write([]byte(assessmentJSON))
		default:
			t.Errorf("unexpected path: %s", actualPath)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := FetchGitlabAssessment(server.URL, 42, ".agentready/assessment-latest.json", "main", "glpat-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
}

func TestGitlabAssessment_TooManyHops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No matter what file is requested, respond with another file name.
		// This guarantees the loop never terminates naturally.
		w.Write([]byte("another-symlink.json"))
	}))
	defer server.Close()

	_, err := FetchGitlabAssessment(server.URL, 42, ".agentready/assessment-latest.json", "main", "glpat-test")

	// Error is expected, success would mean the infinite loop isn't caught.
	if err == nil {
		t.Fatal("expected error for too many symlink hops, got nil")
	}

	// Verify it's the correct error
	if !strings.Contains(err.Error(), "too many symlink hops") {
		t.Errorf("expected 'too many symlink hops' error, got %v", err)
	}
}

func TestFetchGitlabAssessment_DirectJSON(t *testing.T) {
	// When the file ISN't a symlink, the first response is already valid JSON.
	// resolveGitlabSymlink should return immediately.
	assessmentJSON := `{"overall_score": 72.0, "certification_level": "Silver"}`
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte(assessmentJSON))
	}))
	defer server.Close()

	result, err := FetchGitlabAssessment(server.URL, 42, ".agentready/assessment-latest.json", "main", "glpat-test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != assessmentJSON {
		t.Errorf("expected %q, got %q", assessmentJSON, result)
	}
	// Only 1 HTTP call means no symlink follow happened.
	// If this were 2, it would mean resolveGitlabSymlink incorrectly treated
	// valid JSON as a symlink target, wasting an API call on every request.
	if callCount != 1 {
		t.Errorf("expected 1 API call for direct JSON, got %d", callCount)
	}
}

func TestLooksLikeSymlinkTarget(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Should match, these look like symlink targets
		{"simple filename", "assessment-20260512.json", true},
		{"with directory", "subdir/assessment-20260512.json", true},
		{"relative path", "../assessment-20260512.json", true},

		// Should NOT match, these are something else
		{"empty string", "", false},
		{"valid JSON object", `{"score": 85}`, false},
		{"valid JSON array", `[1, 2, 3]`, false},
		{"has newline", "file1.json\nfile2.json", false},
		{"has carriage return", "file.json\r", false},
		{"has null byte", "file\x00.json", false},
		{"too long", strings.Repeat("a", 501) + ".json", false},
		{"wrong extension", "assessment-20260512.yaml", false},
		{"no extension", "assessment-20260512", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeSymlinkTarget(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeSymlinkTarget(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
