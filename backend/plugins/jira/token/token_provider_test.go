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

package token

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/apache/incubator-devlake/plugins/jira/models"
)

func oauthConn(tokenURL string) *models.JiraConn {
	jc := &models.JiraConn{
		ClientId:      "cid",
		ClientSecret:  "csecret",
		CloudId:       "cloud-1",
		OAuthTokenURL: tokenURL,
	}
	jc.AuthMethod = models.AUTH_METHOD_OAUTH2
	return jc
}

func TestNeedsRefresh(t *testing.T) {
	tp := &TokenProvider{conn: oauthConn("")}

	assert.True(t, tp.needsRefresh(), "empty token should refresh")

	tp.conn.SetOAuthAccessToken("tok", time.Now().Add(10*time.Minute))
	assert.False(t, tp.needsRefresh())

	tp.conn.SetOAuthAccessToken("tok", time.Now().Add(1*time.Minute))
	assert.True(t, tp.needsRefresh())

	tp.conn.SetOAuthAccessToken("tok", time.Now().Add(-1*time.Minute))
	assert.True(t, tp.needsRefresh())
}

func TestGetTokenMintsWhenExpired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "fresh-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	conn.SetOAuthAccessToken("stale", time.Now().Add(-time.Minute))

	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)

	token, err := tp.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "fresh-token", token)
}

func TestForceRefreshSkipsIfTokenChanged(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	conn.SetOAuthAccessToken("current", time.Now().Add(time.Hour))
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)

	require.NoError(t, tp.ForceRefresh("stale-old-token"))
	assert.Equal(t, 0, calls)
	assert.Equal(t, "current", conn.OAuthAccessToken())

	require.NoError(t, tp.ForceRefresh("current"))
	assert.Equal(t, 1, calls)
	assert.Equal(t, "new-token", conn.OAuthAccessToken())
}

func TestGetTokenConcurrency(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "shared-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, gerr := tp.GetToken()
			assert.NoError(t, gerr)
			assert.Equal(t, "shared-token", token)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls)
}

func TestRoundTripper401Refresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "new-token",
			"expires_in":   3600,
		}))
	}))
	defer server.Close()

	conn := oauthConn(server.URL)
	conn.SetOAuthAccessToken("old-token", time.Now().Add(10*time.Minute))

	tp, err := NewTokenProvider(conn, nil)
	require.NoError(t, err)

	apiCalls := 0
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		apiCalls++
		auth := req.Header.Get("Authorization")
		if auth == "Bearer old-token" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		if auth == "Bearer new-token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(http.NoBody),
				Request:    req,
			}, nil
		}
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(http.NoBody), Request: req}, nil
	})

	rt := NewRefreshRoundTripper(base, tp)
	req, reqErr := http.NewRequest(http.MethodGet, "https://api.atlassian.com/ex/jira/c/rest/api/2/serverInfo", nil)
	require.NoError(t, reqErr)

	resp, tripErr := rt.RoundTrip(req)
	require.NoError(t, tripErr)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, apiCalls)
	assert.Equal(t, "new-token", conn.OAuthAccessToken())
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
