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

	"github.com/apache/incubator-devlake/plugins/github/models"
	"github.com/stretchr/testify/assert"
)

// --- GraphqlInlineActorQuery tests ---

func TestActorQuery_GetLogin_UserOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Login: "cathay4t", Id: 934948},
	}
	assert.Equal(t, "cathay4t", q.GetLogin())
}

func TestActorQuery_GetLogin_BotOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{Login: "dependabot[bot]", Id: 49699333},
	}
	assert.Equal(t, "dependabot[bot]", q.GetLogin())
}

func TestActorQuery_GetLogin_UserTakesPrecedence(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Login: "realuser"},
		Bot:  GithubBotAccountEdge{Login: "botlogin"},
	}
	assert.Equal(t, "realuser", q.GetLogin())
}

func TestActorQuery_GetLogin_BothEmpty(t *testing.T) {
	q := &GraphqlInlineActorQuery{}
	assert.Equal(t, "", q.GetLogin())
}

func TestActorQuery_GetId_UserOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Id: 934948},
	}
	assert.Equal(t, 934948, q.GetId())
}

func TestActorQuery_GetId_BotOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{Id: 49699333},
	}
	assert.Equal(t, 49699333, q.GetId())
}

func TestActorQuery_GetId_UserTakesPrecedence(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Id: 100},
		Bot:  GithubBotAccountEdge{Id: 200},
	}
	assert.Equal(t, 100, q.GetId())
}

func TestActorQuery_GetId_BothZero(t *testing.T) {
	q := &GraphqlInlineActorQuery{}
	assert.Equal(t, 0, q.GetId())
}

func TestActorQuery_GetName_ReturnsUserName(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Name: "Edward Haas"},
	}
	assert.Equal(t, "Edward Haas", q.GetName())
}

func TestActorQuery_GetName_EmptyForBot(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{Login: "dependabot[bot]", Id: 49699333},
	}
	assert.Equal(t, "", q.GetName())
}

func TestActorQuery_GetCompany_ReturnsUserCompany(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Company: "Red Hat"},
	}
	assert.Equal(t, "Red Hat", q.GetCompany())
}

func TestActorQuery_GetEmail_ReturnsUserEmail(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Email: "user@example.com"},
	}
	assert.Equal(t, "user@example.com", q.GetEmail())
}

func TestActorQuery_GetAvatarUrl_UserOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{AvatarUrl: "https://user-avatar.png"},
	}
	assert.Equal(t, "https://user-avatar.png", q.GetAvatarUrl())
}

func TestActorQuery_GetAvatarUrl_FallsBackToBot(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{AvatarUrl: "https://bot-avatar.png"},
	}
	assert.Equal(t, "https://bot-avatar.png", q.GetAvatarUrl())
}

func TestActorQuery_GetHtmlUrl_UserOnly(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{HtmlUrl: "https://github.com/user"},
	}
	assert.Equal(t, "https://github.com/user", q.GetHtmlUrl())
}

func TestActorQuery_GetHtmlUrl_FallsBackToBot(t *testing.T) {
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{HtmlUrl: "https://github.com/apps/dependabot"},
	}
	assert.Equal(t, "https://github.com/apps/dependabot", q.GetHtmlUrl())
}

// --- GraphqlInlineAccountQuery tests ---

func TestAccountQuery_GetLogin(t *testing.T) {
	q := &GraphqlInlineAccountQuery{
		GithubAccountEdge: GithubAccountEdge{Login: "cathay4t"},
	}
	assert.Equal(t, "cathay4t", q.GetLogin())
}

func TestAccountQuery_GetId(t *testing.T) {
	q := &GraphqlInlineAccountQuery{
		GithubAccountEdge: GithubAccountEdge{Id: 934948},
	}
	assert.Equal(t, 934948, q.GetId())
}

func TestAccountQuery_GetLogin_Empty(t *testing.T) {
	q := &GraphqlInlineAccountQuery{}
	assert.Equal(t, "", q.GetLogin())
}

func TestAccountQuery_GetId_Zero(t *testing.T) {
	q := &GraphqlInlineAccountQuery{}
	assert.Equal(t, 0, q.GetId())
}

// --- GraphqlAccount interface compliance ---

func TestActorQueryImplementsGraphqlAccount(t *testing.T) {
	var _ GraphqlAccount = &GraphqlInlineActorQuery{}
}

func TestAccountQueryImplementsGraphqlAccount(t *testing.T) {
	var _ GraphqlAccount = &GraphqlInlineAccountQuery{}
}

// --- extractGraphqlPreActorAccount tests ---

func TestExtractActorAccount_NilInput(t *testing.T) {
	var results []interface{}
	extractGraphqlPreActorAccount(&results, nil, 132556280, 2)
	assert.Empty(t, results)
}

func TestExtractActorAccount_ZeroId(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineActorQuery{} // all zero values
	extractGraphqlPreActorAccount(&results, q, 132556280, 2)
	assert.Empty(t, results)
}

func TestExtractActorAccount_ValidBot(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{Login: "dependabot[bot]", Id: 49699333},
	}
	extractGraphqlPreActorAccount(&results, q, 132556280, 2)

	assert.Len(t, results, 1)
	account, ok := results[0].(*models.GithubRepoAccount)
	assert.True(t, ok)
	assert.Equal(t, "dependabot[bot]", account.Login)
	assert.Equal(t, 49699333, account.AccountId)
	assert.Equal(t, 132556280, account.RepoGithubId)
	assert.Equal(t, uint64(2), account.ConnectionId)
}

func TestExtractActorAccount_ValidUser(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Login: "cathay4t", Id: 934948},
	}
	extractGraphqlPreActorAccount(&results, q, 132556280, 2)

	assert.Len(t, results, 1)
	account, ok := results[0].(*models.GithubRepoAccount)
	assert.True(t, ok)
	assert.Equal(t, "cathay4t", account.Login)
	assert.Equal(t, 934948, account.AccountId)
}

func TestExtractActorAccount_UserTakesPrecedenceOverBot(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineActorQuery{
		User: GithubAccountEdge{Login: "realuser", Id: 100},
		Bot:  GithubBotAccountEdge{Login: "botuser", Id: 200},
	}
	extractGraphqlPreActorAccount(&results, q, 132556280, 2)

	assert.Len(t, results, 1)
	account := results[0].(*models.GithubRepoAccount)
	assert.Equal(t, "realuser", account.Login)
	assert.Equal(t, 100, account.AccountId)
}

// --- extractGraphqlPreAccount tests ---

func TestExtractAccount_NilInput(t *testing.T) {
	var results []interface{}
	extractGraphqlPreAccount(&results, nil, 132556280, 2)
	assert.Empty(t, results)
}

func TestExtractAccount_ZeroId(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineAccountQuery{}
	extractGraphqlPreAccount(&results, q, 132556280, 2)
	assert.Empty(t, results)
}

func TestExtractAccount_ValidUser(t *testing.T) {
	var results []interface{}
	q := &GraphqlInlineAccountQuery{
		GithubAccountEdge: GithubAccountEdge{Login: "cathay4t", Id: 934948},
	}
	extractGraphqlPreAccount(&results, q, 132556280, 2)

	assert.Len(t, results, 1)
	account, ok := results[0].(*models.GithubRepoAccount)
	assert.True(t, ok)
	assert.Equal(t, "cathay4t", account.Login)
	assert.Equal(t, 934948, account.AccountId)
	assert.Equal(t, 132556280, account.RepoGithubId)
	assert.Equal(t, uint64(2), account.ConnectionId)
}

// --- extractGraphqlPreActorAccount appends to existing results ---

func TestExtractActorAccount_AppendsToExisting(t *testing.T) {
	results := []interface{}{"existing-item"}
	q := &GraphqlInlineActorQuery{
		Bot: GithubBotAccountEdge{Login: "dependabot[bot]", Id: 49699333},
	}
	extractGraphqlPreActorAccount(&results, q, 132556280, 2)

	assert.Len(t, results, 2)
	assert.Equal(t, "existing-item", results[0])
	account := results[1].(*models.GithubRepoAccount)
	assert.Equal(t, "dependabot[bot]", account.Login)
}
