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
	"net/http"
	"sync"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/plugins/jira/models"
)

const (
	DefaultRefreshBuffer = 5 * time.Minute
)

// TokenProvider mints Atlassian OAuth 2.0 client-credentials tokens and caches them in memory.
type TokenProvider struct {
	conn       *models.JiraConn
	httpClient *http.Client
	logger     log.Logger
	mu         sync.Mutex
}

// NewTokenProvider creates a TokenProvider that uses a dedicated HTTP client for token requests.
// Call this BEFORE wrapping the Jira API client's transport with RefreshRoundTripper so token
// minting never goes through the round tripper.
func NewTokenProvider(conn *models.JiraConn, logger log.Logger) (*TokenProvider, errors.Error) {
	httpClient, err := models.NewOAuthHTTPClient(conn.GetProxy())
	if err != nil {
		return nil, err
	}
	return &TokenProvider{
		conn:       conn,
		httpClient: httpClient,
		logger:     logger,
	}, nil
}

func (tp *TokenProvider) GetToken() (string, errors.Error) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.needsRefresh() {
		if tp.logger != nil {
			expiresStr := "unknown"
			if exp := tp.conn.OAuthAccessTokenExpiresAt(); exp != nil {
				expiresStr = exp.Format(time.RFC3339)
			}
			tp.logger.Info("Proactive oauth2 token refresh triggered (token expires at %s)", expiresStr)
		}
		if err := tp.refreshToken(); err != nil {
			return "", err
		}
	}
	return tp.conn.OAuthAccessToken(), nil
}

func (tp *TokenProvider) needsRefresh() bool {
	if tp.conn.OAuthAccessToken() == "" {
		return true
	}
	expiresAt := tp.conn.OAuthAccessTokenExpiresAt()
	if expiresAt == nil {
		return false
	}
	return time.Now().Add(DefaultRefreshBuffer).After(*expiresAt)
}

func (tp *TokenProvider) refreshToken() errors.Error {
	if tp.logger != nil {
		tp.logger.Info("Minting Jira oauth2 access token via client_credentials")
	}
	return tp.conn.MintOAuthAccessToken(tp.httpClient)
}

// ForceRefresh remints the access token if the current token is still equal to oldToken.
func (tp *TokenProvider) ForceRefresh(oldToken string) errors.Error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.conn.OAuthAccessToken() != oldToken {
		if tp.logger != nil {
			tp.logger.Info("Skipping reactive oauth2 token refresh — token already changed by another goroutine")
		}
		return nil
	}
	if tp.logger != nil {
		tp.logger.Info("Reactive oauth2 token refresh triggered (received 401)")
	}
	return tp.refreshToken()
}
