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

package models

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMintOAuthAccessToken(t *testing.T) {
	var gotGrant, gotID, gotSecret, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		require.NoError(t, r.ParseForm())
		gotGrant = r.Form.Get("grant_type")
		gotID = r.Form.Get("client_id")
		gotSecret = r.Form.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "minted-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
			"scope":        "read:jira-work",
		}))
	}))
	defer server.Close()

	jc := &JiraConn{
		ClientId:      "cid",
		ClientSecret:  "csecret",
		CloudId:       "cloud-1",
		OAuthTokenURL: server.URL,
	}
	jc.AuthMethod = AUTH_METHOD_OAUTH2

	err := jc.MintOAuthAccessToken(server.Client())
	require.NoError(t, err)
	assert.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	assert.Equal(t, "client_credentials", gotGrant)
	assert.Equal(t, "cid", gotID)
	assert.Equal(t, "csecret", gotSecret)
	assert.Equal(t, "minted-token", jc.OAuthAccessToken())
	require.NotNil(t, jc.OAuthAccessTokenExpiresAt())
	assert.True(t, jc.OAuthAccessTokenExpiresAt().After(time.Now().Add(50*time.Minute)))
}

func TestMintOAuthAccessTokenRejectsEmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"","expires_in":3600}`)
	}))
	defer server.Close()

	jc := &JiraConn{OAuthTokenURL: server.URL}
	err := jc.MintOAuthAccessToken(server.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty oauth2 access token")
}

func TestMintOAuthAccessTokenHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"invalid_client"}`)
	}))
	defer server.Close()

	jc := &JiraConn{OAuthTokenURL: server.URL}
	err := jc.MintOAuthAccessToken(server.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}
