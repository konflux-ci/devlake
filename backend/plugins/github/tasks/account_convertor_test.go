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

func TestIsBotAccount(t *testing.T) {
	tests := []struct {
		name        string
		login       string
		accountType string
		want        bool
	}{
		// API-reported type takes precedence over any login pattern.
		{"api type marks bot even without suffix", "some-github-app", "Bot", true},

		// [bot] suffix - GitHub Apps.
		{"github app bot suffix", "renovate[bot]", "", true},
		{"github-actions bot suffix", "github-actions[bot]", "", true},
		{"coderabbit bot suffix", "coderabbit[bot]", "", true},
		{"cursor bot suffix", "cursor[bot]", "", true},
		{"claude bot suffix", "claude[bot]", "", true},
		{"copilot reviewer bot suffix", "copilot-pull-request-reviewer[bot]", "", true},

		// Known bot logins referenced by DPROD-1262.
		{"triage bot login", "konflux-ci-triage[bot]", "", true},
		{"review bot login", "konflux-ci-review[bot]", "", true},
		{"coder bot login", "konflux-ci-coder[bot]", "", true},
		{"fullsend bot login", "konflux-ci-fullsend[bot]", "", true},

		// Hyphenated -bot/-robot suffixes (no brackets).
		{"hyphenated bot suffix", "openshift-bot", "", true},
		{"hyphenated robot suffix", "openshift-cherrypick-robot", "", true},
		{"redhat renovate bot suffix", "redhat-renovate-bot", "", true},
		{"snyk bot suffix", "snyk-bot", "", true},

		// Standalone names without a suffix marker.
		{"standalone copilot login", "copilot", "", true},
		{"dependabot without brackets", "dependabot", "", true},
		{"github-actions without brackets", "github-actions", "", true},
		{"codecov commenter bot, real GitHub API type is User", "codecov-commenter", "User", true},

		// Negative cases: must not false-positive on substrings, only
		// on the known suffixes/markers above.
		{"normal human user", "octocat", "User", false},
		{"organization account is not a bot", "my-org", "Organization", false},
		{"surname containing bot as a substring", "abbott", "User", false},
		{"login containing bot mid-string", "robotics-fan", "User", false},
		{"copilot substring does not mark bot", "copilot-trainer", "User", false},
		{"dependabot substring does not mark bot", "dependabot-helper", "User", false},
		{"github-actions substring does not mark bot", "github-actions-user", "User", false},
		{"codecov-commenter substring does not mark bot", "codecov-commenter-fan", "User", false},
		{"empty type and no bot marker", "some-human-login", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBotAccount(tt.login, tt.accountType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildDomainAccount_SetsIsBot(t *testing.T) {
	tests := []struct {
		name      string
		row       *repoAccountForConvert
		wantIsBot bool
	}{
		{"human user with full profile", &repoAccountForConvert{Login: "octocat", Type: "User", AvatarUrl: "https://avatars.githubusercontent.com/u/1"}, false},
		{"api type marks bot", &repoAccountForConvert{Login: "some-app", Type: "Bot", AvatarUrl: "https://avatars.githubusercontent.com/u/2"}, true},
		{"bot suffix login", &repoAccountForConvert{Login: "renovate[bot]", Type: "User", AvatarUrl: "https://avatars.githubusercontent.com/u/3"}, true},
		{"unrecognized login with no profile data is not marked as bot", &repoAccountForConvert{Login: "totally-unknown-name", Type: "", AvatarUrl: ""}, false},
		{"unrecognized login with a real profile is not a bot", &repoAccountForConvert{Login: "totally-unknown-name", Type: "", AvatarUrl: "https://avatars.githubusercontent.com/u/4"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDomainAccount("github:GithubAccount:1:1", tt.row, "")
			assert.Equal(t, tt.wantIsBot, got.IsBot)
			assert.Equal(t, tt.row.Login, got.UserName)
		})
	}
}

func TestBuildDomainAccount_FullNameFallsBackToLogin(t *testing.T) {
	tests := []struct {
		name         string
		row          *repoAccountForConvert
		wantFullName string
	}{
		{"no profile falls back to login", &repoAccountForConvert{Login: "konflux-ci-triage[bot]", Name: ""}, "konflux-ci-triage[bot]"},
		{"real profile name is preserved", &repoAccountForConvert{Login: "octocat", Name: "The Octocat"}, "The Octocat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDomainAccount("github:GithubAccount:1:1", tt.row, "")
			assert.Equal(t, tt.wantFullName, got.FullName)
		})
	}
}
