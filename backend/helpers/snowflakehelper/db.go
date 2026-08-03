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

// Package snowflakehelper provides shared Snowflake connectivity helpers for
// Konflux-owned snowflake-backed DevLake plugins (github_snowflake, jira_snowflake).
package snowflakehelper

import (
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"

	"github.com/apache/incubator-devlake/core/errors"
	sf "github.com/snowflakedb/gosnowflake"
)

// Open opens a database/sql connection to Snowflake.
//
// authType controls authentication:
//   - "keypair" (default): JWT key-pair auth using privateKeyPEM. Works in containers and CI.
//   - "externalbrowser": SSO via browser pop-up. Only works when DevLake runs on a desktop host
//     (i.e. via `make run`, not inside a Docker container).
func Open(account, user, authType, privateKeyPEM, database, schema, warehouse, role string) (*sql.DB, errors.Error) {
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

	switch authType {
	case "", "keypair":
		privKey, err := ParseRSAPrivateKey(privateKeyPEM)
		if err != nil {
			return nil, errors.Default.Wrap(err, "failed to parse Snowflake private key")
		}
		cfg.Authenticator = sf.AuthTypeJwt
		cfg.PrivateKey = privKey
	case "externalbrowser":
		cfg.Authenticator = sf.AuthTypeExternalBrowser
	default:
		return nil, errors.BadInput.New(fmt.Sprintf(
			`unsupported authType %q; must be "keypair" or "externalbrowser"`, authType,
		))
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

// ParseRSAPrivateKey parses a PKCS#8 PEM-encoded RSA private key.
func ParseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
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
