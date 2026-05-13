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
	"github.com/apache/incubator-devlake/plugins/github/models"
)

// GraphqlAccount is an interface for GraphQL account types that can provide
// basic identity fields. Both GraphqlInlineAccountQuery (User-only contexts)
// and GraphqlInlineActorQuery (Actor contexts including Bot) implement this.
type GraphqlAccount interface {
	GetLogin() string
	GetId() int
}

// GithubAccountEdge represents the fields available on GitHub User accounts.
type GithubAccountEdge struct {
	Login     string
	Id        int `graphql:"databaseId"`
	Name      string
	Company   string
	Email     string
	AvatarUrl string
	HtmlUrl   string `graphql:"url"`
}

// GithubBotAccountEdge represents a GitHub Bot actor.
// Bot accounts support a subset of the fields that User accounts do
// (no Name, Company, or Email).
type GithubBotAccountEdge struct {
	Login     string
	Id        int    `graphql:"databaseId"`
	AvatarUrl string
	HtmlUrl   string `graphql:"url"`
}

// GraphqlInlineAccountQuery is used in GraphQL contexts where only User type
// is valid (e.g. assignees, commit author). It uses a single "... on User"
// fragment.
type GraphqlInlineAccountQuery struct {
	GithubAccountEdge `graphql:"... on User"`
}

// GetLogin returns the login.
func (q *GraphqlInlineAccountQuery) GetLogin() string {
	return q.Login
}

// GetId returns the database ID.
func (q *GraphqlInlineAccountQuery) GetId() int {
	return q.Id
}

// GraphqlInlineActorQuery resolves GitHub's Actor interface which can be a
// User or a Bot. Both fragments are included so that bot-authored PRs,
// reviews, and issues are correctly attributed. Use this type for fields
// typed as Actor in the GitHub GraphQL schema (e.g. PR author, review
// author, mergedBy).
type GraphqlInlineActorQuery struct {
	User GithubAccountEdge    `graphql:"... on User"`
	Bot  GithubBotAccountEdge `graphql:"... on Bot"`
}

// GetLogin returns the login for whichever actor type matched.
func (q *GraphqlInlineActorQuery) GetLogin() string {
	if q.User.Login != "" {
		return q.User.Login
	}
	return q.Bot.Login
}

// GetId returns the database ID for whichever actor type matched.
func (q *GraphqlInlineActorQuery) GetId() int {
	if q.User.Id != 0 {
		return q.User.Id
	}
	return q.Bot.Id
}

// GetName returns the display name (only available for User actors).
func (q *GraphqlInlineActorQuery) GetName() string {
	return q.User.Name
}

// GetCompany returns the company (only available for User actors).
func (q *GraphqlInlineActorQuery) GetCompany() string {
	return q.User.Company
}

// GetEmail returns the email (only available for User actors).
func (q *GraphqlInlineActorQuery) GetEmail() string {
	return q.User.Email
}

// GetAvatarUrl returns the avatar URL for whichever actor type matched.
func (q *GraphqlInlineActorQuery) GetAvatarUrl() string {
	if q.User.AvatarUrl != "" {
		return q.User.AvatarUrl
	}
	return q.Bot.AvatarUrl
}

// GetHtmlUrl returns the HTML URL for whichever actor type matched.
func (q *GraphqlInlineActorQuery) GetHtmlUrl() string {
	if q.User.HtmlUrl != "" {
		return q.User.HtmlUrl
	}
	return q.Bot.HtmlUrl
}

// extractGraphqlPreActorAccount extracts a GithubRepoAccount from a
// GraphqlInlineActorQuery (Actor context: PR author, review author, etc.).
func extractGraphqlPreActorAccount(result *[]interface{}, res *GraphqlInlineActorQuery, repoId int, connId uint64) {
	if res == nil || res.GetId() == 0 {
		return
	}
	*result = append(*result, &models.GithubRepoAccount{
		ConnectionId: connId,
		RepoGithubId: repoId,
		Login:        res.GetLogin(),
		AccountId:    res.GetId(),
	})
}

// extractGraphqlPreAccount extracts a GithubRepoAccount from a
// GraphqlInlineAccountQuery (User-only context: assignees, commit author).
func extractGraphqlPreAccount(result *[]interface{}, res *GraphqlInlineAccountQuery, repoId int, connId uint64) {
	if res == nil || res.GetId() == 0 {
		return
	}
	*result = append(*result, &models.GithubRepoAccount{
		ConnectionId: connId,
		RepoGithubId: repoId,
		Login:        res.GetLogin(),
		AccountId:    res.GetId(),
	})
}
