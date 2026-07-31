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
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	githubmodels "github.com/apache/incubator-devlake/plugins/github/models"
	sf "github.com/snowflakedb/gosnowflake"
)

// Raw table name constants used for StatefulDataConverter state params
// (same strings as plugins/github/tasks so state keys stay compatible).
const (
	RAW_PULL_REQUEST_TABLE = "github_api_pull_requests"
	RAW_PR_COMMIT_TABLE    = "github_api_pull_request_commits"
	RAW_PR_REVIEW_TABLE    = "github_api_pull_request_reviews"
	RAW_ACCOUNT_TABLE      = "github_api_accounts"
)

// GithubSnowflakeOptions contains all per-pipeline task options.
type GithubSnowflakeOptions struct {
	ConnectionId  uint64                          `json:"connectionId"  mapstructure:"connectionId"`
	GithubId      int                             `json:"githubId"      mapstructure:"githubId"`
	Name          string                          `json:"name"          mapstructure:"name"`
	FullName      string                          `json:"fullName"      mapstructure:"fullName"`
	ScopeConfigId uint64                          `json:"scopeConfigId" mapstructure:"scopeConfigId"`
	ScopeConfig   *githubmodels.GithubScopeConfig `json:"scopeConfig"   mapstructure:"scopeConfig"`
}

// GithubSnowflakeTaskData is passed to every subtask via taskCtx.GetData().
type GithubSnowflakeTaskData struct {
	Options     *GithubSnowflakeOptions
	SnowflakeDB *sql.DB
}

// GithubApiParams mirrors github/models.GithubApiParams so that RawDataSubTaskArgs
// produces the same _raw_data_params format for state management.
type GithubApiParams struct {
	ConnectionId uint64
	Name         string
}

// DecodeAndValidateTaskOptions decodes and validates options for the task.
func DecodeAndValidateTaskOptions(options map[string]interface{}) (*GithubSnowflakeOptions, errors.Error) {
	var op GithubSnowflakeOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, err
	}
	if op.ConnectionId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid connectionId: %d", op.ConnectionId))
	}
	if op.GithubId == 0 {
		return nil, errors.BadInput.New(fmt.Sprintf("invalid githubId: %d", op.GithubId))
	}
	if op.Name == "" {
		op.Name = op.FullName
	}
	if op.FullName == "" {
		op.FullName = op.Name
	}
	if op.Name == "" {
		return nil, errors.BadInput.New("name (owner/repo full name) must not be empty")
	}
	return &op, nil
}

// OpenSnowflakeDB opens a database/sql connection to Snowflake.
//
// authType controls authentication:
//   - "keypair" (default): JWT key-pair auth using privateKeyPEM. Works in containers and CI.
//   - "externalbrowser": SSO via browser pop-up. Only works when DevLake runs on a desktop host
//     (i.e. via `make run`, not inside a Docker container).
func OpenSnowflakeDB(account, user, authType, privateKeyPEM, database, schema, warehouse, role string) (*sql.DB, errors.Error) {
	cfg := &sf.Config{
		Account:  account,
		User:     user,
		Database: database,
		Schema:   schema,
	}
	if warehouse != "" {
		cfg.Warehouse = warehouse
	}
	if role != "" {
		cfg.Role = role
	}

	if authType == "externalbrowser" {
		cfg.Authenticator = sf.AuthTypeExternalBrowser
	} else {
		privKey, err := parseRSAPrivateKey(privateKeyPEM)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to parse Snowflake private key")
		}
		cfg.Authenticator = sf.AuthTypeJwt
		cfg.PrivateKey = privKey
	}

	dsn, goErr := sf.DSN(cfg)
	if goErr != nil {
		return nil, errors.Default.Wrap(goErr, "failed to build Snowflake DSN")
	}
	db, goErr := sql.Open("snowflake", dsn)
	if goErr != nil {
		return nil, errors.Default.Wrap(goErr, "failed to open Snowflake connection")
	}
	return db, nil
}

// parseRSAPrivateKey parses a PKCS#8 PEM-encoded RSA private key.
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not an RSA key")
	}
	return rsaKey, nil
}

// repoShortName returns the short name from owner/repo full name.
func repoShortName(fullName string) string {
	parts := strings.Split(fullName, "/")
	if len(parts) == 0 {
		return fullName
	}
	return parts[len(parts)-1]
}
