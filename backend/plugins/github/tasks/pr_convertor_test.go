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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasValidAccountId(t *testing.T) {
	tests := []struct {
		name          string
		toolAccountId int
		want          bool
	}{
		// GitHub's REST API returns author/merged_by = null for many bot
		// PRs; the tool layer stores that as AuthorId/MergedById = 0. That
		// zero must not be treated as a real account, or it produces a
		// phantom domain ID (e.g. "github:GithubAccount:1:0") that matches
		// no row in accounts.
		{"zero id is a phantom null-author marker", 0, false},
		{"negative id is invalid", -1, false},
		{"positive id is a valid account", 123456, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasValidAccountId(tt.toolAccountId))
		})
	}
}
