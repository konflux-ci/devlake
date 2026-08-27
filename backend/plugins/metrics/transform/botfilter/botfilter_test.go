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

package botfilter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBot(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		wantBot bool
	}{
		{"empty name", "", true},
		{"real human", "alice", false},
		{"dependabot", "dependabot[bot]", true},
		{"renovate", "renovate[bot]", true},
		{"copilot", "Copilot", true},
		{"cursor bot", "cursor[bot]", true},
		{"snyk", "snyk-bot", true},
		{"-bot suffix", "my-bot", true},
		{"-robot suffix", "my-robot", true},
		{"ci-operator", "ci-operator", true},
		{"rh-tap-build-team", "rh-tap-build-team", true},
		{"red-hat-konflux", "red-hat-konflux", true},
		{"project_123_bot", "project_123_bot", true},
		{"group_99_bot", "group_99_bot", true},
		{"gh-actions always bot", "github-actions[bot]", true},
		{"gh-actions bare", "github-actions", true},
		{"fullsend-ai", "fullsend-ai", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBot(tt.author, "", "")
			assert.Equal(t, tt.wantBot, got)
		})
	}
}
