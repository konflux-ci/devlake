/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	helperapi "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/stretchr/testify/assert"
)

// buildTestTaskData creates a minimal JiraTaskData pointing ApiClient at the given server URL.
func buildTestTaskData(serverURL string, connectionId, boardId uint64) *JiraTaskData {
	apiClient := &helperapi.ApiClient{}
	apiClient.Setup(serverURL, nil, 10*time.Second)
	return &JiraTaskData{
		Options: &JiraOptions{
			ConnectionId: connectionId,
			BoardId:      boardId,
		},
		ApiClient: &helperapi.ApiAsyncClient{ApiClient: apiClient},
	}
}

// boardResponse is the JSON shape returned by agile/1.0/board/:id/issue.
type boardResponse struct {
	Total  int              `json:"total"`
	Issues []boardRespIssue `json:"issues"`
}

type boardRespIssue struct {
	Key string `json:"key"`
}

func TestFetchBoardMembership_EmptyIssues(t *testing.T) {
	// No issues to check → no HTTP calls needed, should return an empty map.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected HTTP call with empty issue list")
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, nil)
	assert.NoError(t, err)
	assert.NotNil(t, onBoard)
	assert.Empty(t, onBoard)
}

func TestFetchBoardMembership_AllOnBoard(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
		{"PROJ-3", 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := boardResponse{
			Total: 3,
			Issues: []boardRespIssue{
				{Key: "PROJ-1"},
				{Key: "PROJ-2"},
				{Key: "PROJ-3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-2": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_SomeNotOnBoard(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2}, // will be missing from API response
		{"PROJ-3", 3},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only PROJ-1 and PROJ-3 are on the board.
		resp := boardResponse{
			Total:  2,
			Issues: []boardRespIssue{{Key: "PROJ-1"}, {Key: "PROJ-3"}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_404ReturnsNil(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Nil(t, onBoard)
}

func TestFetchBoardMembership_NonOKStatusReturnsError(t *testing.T) {
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	_, err := fetchBoardMembership(data, 42, issues)
	assert.Error(t, err)
}

func TestFetchBoardMembership_PaginationFetchesAllPages(t *testing.T) {
	// API returns 3 issues total but only 2 per page → 2 requests.
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
		{"PROJ-3", 3},
	}

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		requestCount++
		var resp boardResponse
		if startAt == "0" || startAt == "" {
			resp = boardResponse{
				Total:  3,
				Issues: []boardRespIssue{{Key: "PROJ-1"}, {Key: "PROJ-2"}},
			}
		} else if startAt == "100" {
			resp = boardResponse{
				Total:  3,
				Issues: []boardRespIssue{{Key: "PROJ-3"}},
			}
		} else {
			http.Error(w, "unexpected startAt: "+startAt, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, 2, requestCount)
	assert.Equal(t, map[string]bool{
		"PROJ-1": true,
		"PROJ-2": true,
		"PROJ-3": true,
	}, onBoard)
}

func TestFetchBoardMembership_EmptyPageBreaksLoop(t *testing.T) {
	// Simulates permission filtering: API reports Total > 0 but returns 0 issues.
	// The loop must break immediately instead of spinning indefinitely.
	issues := []struct {
		IssueKey string
		IssueId  uint64
	}{
		{"PROJ-1", 1},
		{"PROJ-2", 2},
	}

	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		resp := boardResponse{
			Total:  5, // non-zero total but no issues returned (permission filtering)
			Issues: []boardRespIssue{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Equal(t, 1, requestCount)
	assert.Empty(t, onBoard)
}

func TestFetchBoardMembership_BatchesBigIssueListIntoMultipleAPIRequests(t *testing.T) {
	// Build 150 issues — more than staleBoardIssueCheckBatchSize (100).
	// Should result in 2 separate API batch requests with different JQL sets.
	issues := make([]struct {
		IssueKey string
		IssueId  uint64
	}, 150)
	for i := range issues {
		issues[i] = struct {
			IssueKey string
			IssueId  uint64
		}{fmt.Sprintf("PROJ-%d", i+1), uint64(i + 1)}
	}

	jqlBatches := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql := r.URL.Query().Get("jql")
		if r.URL.Query().Get("startAt") == "0" || r.URL.Query().Get("startAt") == "" {
			jqlBatches = append(jqlBatches, jql)
		}
		// Return all requested keys as "on board" so pagination terminates immediately.
		parts := strings.TrimPrefix(jql, "issue IN (")
		parts = strings.TrimSuffix(parts, ")")
		keys := strings.Split(parts, ",")
		respIssues := make([]boardRespIssue, 0, len(keys))
		for _, k := range keys {
			respIssues = append(respIssues, boardRespIssue{Key: k})
		}
		resp := boardResponse{Total: len(respIssues), Issues: respIssues}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	data := buildTestTaskData(srv.URL, 1, 42)
	onBoard, err := fetchBoardMembership(data, 42, issues)
	assert.NoError(t, err)
	assert.Len(t, jqlBatches, 2)
	assert.Contains(t, jqlBatches[0], "PROJ-1")
	assert.Contains(t, jqlBatches[0], "PROJ-100")
	assert.NotContains(t, jqlBatches[0], "PROJ-101")
	assert.Contains(t, jqlBatches[1], "PROJ-101")
	assert.Contains(t, jqlBatches[1], "PROJ-150")
	assert.Len(t, onBoard, 150)
}
