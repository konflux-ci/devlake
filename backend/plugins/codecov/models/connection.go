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
	"fmt"
	"net/http"

	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// CodecovAccessToken supports token-based authentication
type CodecovAccessToken struct {
	helper.AccessToken `mapstructure:",squash"`
}

// SetupAuthentication sets up the HTTP Request Authentication
func (conn *CodecovAccessToken) SetupAuthentication(req *http.Request) errors.Error {
	if conn.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", conn.Token))
	}
	return nil
}

// CodecovConn holds the essential information to connect to the Codecov API
type CodecovConn struct {
	helper.RestConnection `mapstructure:",squash"`
	CodecovAccessToken    `mapstructure:",squash"`
	Organization          string `mapstructure:"organization" json:"organization" gorm:"type:varchar(255)" validate:"required"`
}

// CodecovConnection holds CodecovConn plus ID/Name for database storage
type CodecovConnection struct {
	helper.BaseConnection `mapstructure:",squash"`
	CodecovConn           `mapstructure:",squash"`
}

func (connection CodecovConnection) TableName() string {
	return "_tool_codecov_connections"
}

func (connection *CodecovConnection) MergeFromRequest(target *CodecovConnection, body map[string]interface{}) error {
	modifiedConnection := CodecovConnection{}
	if err := helper.DecodeMapStruct(body, &modifiedConnection, true); err != nil {
		return err
	}
	return connection.Merge(target, &modifiedConnection, body)
}

func (connection *CodecovConnection) Merge(existed, modified *CodecovConnection, body map[string]interface{}) error {
	existedTokenStr := existed.Token
	existed.Name = modified.Name
	existed.Organization = modified.Organization
	existed.Proxy = modified.Proxy
	existed.Endpoint = modified.Endpoint
	existed.RateLimitPerHour = modified.RateLimitPerHour

	// handle token
	if existedTokenStr == "" {
		if modified.Token != "" {
			existed.Token = modified.Token
		}
	} else {
		if modified.Token == "" {
			// delete token
			existed.Token = modified.Token
		} else {
			// update token
			sanitizeToken := existed.Sanitize().Token
			if sanitizeToken == modified.Token {
				// change nothing, restore it
				existed.Token = existedTokenStr
			} else {
				// has changed, replace it with the new token
				existed.Token = modified.Token
			}
		}
	}

	return nil
}

func (connection CodecovConnection) Sanitize() CodecovConnection {
	connection.CodecovConn = connection.CodecovConn.Sanitize()
	return connection
}

func (conn *CodecovConn) Sanitize() CodecovConn {
	conn.SanitizeToken()
	return *conn
}

func (conn *CodecovConn) SanitizeToken() CodecovConn {
	if conn.Token == "" {
		return *conn
	}
	// Codecov tokens are typically UUIDs or similar, mask most of it
	if len(conn.Token) > 8 {
		showPrefixLen := 4
		hiddenLen := len(conn.Token) - showPrefixLen - 4
		secret := ""
		for i := 0; i < hiddenLen; i++ {
			secret += "*"
		}
		conn.Token = conn.Token[:showPrefixLen] + secret + conn.Token[len(conn.Token)-4:]
	} else {
		conn.Token = "****"
	}
	return *conn
}
