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

package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	dsmodels "github.com/apache/incubator-devlake/helpers/pluginhelper/api/models"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

type CodecovRemotePagination struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type codecovRepo struct {
	Name        string `json:"name"`
	Service     string `json:"service"`
	Language    string `json:"language"`
	Active      bool   `json:"active"`
	ActivatedAt string `json:"activated_at"`
	Updatestamp string `json:"updatestamp"`
	Private     bool   `json:"private"`
	Branch      string `json:"branch"`
	Repository  struct {
		Name string `json:"name"`
	} `json:"repository"`
}

type codecovReposResponse struct {
	Results []codecovRepo `json:"results"`
	Next    *string       `json:"next,omitempty"`
}

func listCodecovRemoteScopes(
	connection *models.CodecovConnection,
	apiClient plugin.ApiClient,
	groupId string,
	page CodecovRemotePagination,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo],
	nextPage *CodecovRemotePagination,
	err errors.Error,
) {
	if page.Page == 0 {
		page.Page = 1
	}
	if page.PerPage == 0 {
		page.PerPage = 100
	}

	// Codecov API endpoint: GET /api/v2/github/{owner}/repos/
	// According to Codecov API docs: https://docs.codecov.com/reference/overview
	// Service is "github" for GitHub repositories
	// If groupId is empty, we're listing repos for the organization
	owner := connection.Organization
	if groupId != "" {
		owner = groupId
	}

	query := url.Values{
		"page":      []string{fmt.Sprintf("%v", page.Page)},
		"page_size": []string{fmt.Sprintf("%v", page.PerPage)},
	}

	// Codecov API format: /api/v2/github/{owner}/repos/
	reposUrl := fmt.Sprintf("/api/v2/github/%s/repos/", owner)
	reposBody, err := apiClient.Get(reposUrl, query, nil)
	if err != nil {
		return nil, nil, err
	}

	if reposBody.StatusCode == http.StatusNotFound {
		return nil, nil, errors.HttpStatus(http.StatusNotFound).New(fmt.Sprintf("Organization or owner '%s' not found", owner))
	}

	if reposBody.StatusCode != http.StatusOK {
		return nil, nil, errors.HttpStatus(reposBody.StatusCode).New("unexpected status code while fetching repositories")
	}

	var reposResponse codecovReposResponse
	err = api.UnmarshalResponse(reposBody, &reposResponse)
	if err != nil {
		return nil, nil, err
	}

	for _, repo := range reposResponse.Results {
		// Get repo name - it might be in repo.Name or repo.Repository.Name
		repoName := repo.Name
		if repoName == "" && repo.Repository.Name != "" {
			repoName = repo.Repository.Name
		}
		if repoName == "" {
			continue
		}

		fullName := fmt.Sprintf("%s/%s", owner, repoName)
		codecovId := fmt.Sprintf("%s/%s", owner, repoName)

		// Determine branch - only include repos with main or master branch
		branch := repo.Branch
		if branch != "main" && branch != "master" {
			// Skip repos that don't have main or master branch
			continue
		}

		children = append(children, dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo]{
			Type:     api.RAS_ENTRY_TYPE_SCOPE,
			ParentId: nil,
			Id:       codecovId,
			Name:     repoName,
			FullName: fullName,
			Data: &models.CodecovRepo{
				CodecovId:   codecovId,
				Name:        repoName,
				FullName:    fullName,
				Service:     repo.Service,
				Language:    repo.Language,
				Active:      repo.Active,
				ActivatedAt: repo.ActivatedAt,
				Updatestamp: repo.Updatestamp,
				Private:     repo.Private,
				Branch:      branch,
			},
		})
	}

	// Check if there's a next page
	if reposResponse.Next != nil && *reposResponse.Next != "" {
		nextPage = &CodecovRemotePagination{
			Page:    page.Page + 1,
			PerPage: page.PerPage,
		}
	}

	return children, nextPage, nil
}

func searchCodecovRepos(
	apiClient plugin.ApiClient,
	params *dsmodels.DsRemoteApiScopeSearchParams,
) (
	children []dsmodels.DsRemoteApiScopeListEntry[models.CodecovRepo],
	err errors.Error,
) {
	// Codecov API may not have a search endpoint, so we'll return empty for now
	// This can be implemented later if Codecov provides search functionality
	return children, nil
}

// RemoteScopes list all available scopes on the remote server
// @Summary list all available scopes on the remote server
// @Description list all available scopes on the remote server
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param groupId query string false "group ID"
// @Param pageToken query string false "page Token"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.CodecovRepo]
// @Tags plugins/codecov
// @Router /plugins/codecov/connections/{connectionId}/remote-scopes [GET]
func RemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeList.Get(input)
}

// SearchRemoteScopes searches scopes on the remote server
// @Summary searches scopes on the remote server
// @Description searches scopes on the remote server
// @Accept application/json
// @Param connectionId path int false "connection ID"
// @Param search query string false "search"
// @Param page query int false "page number"
// @Param pageSize query int false "page size per page"
// @Failure 400  {object} shared.ApiBody "Bad Request"
// @Failure 500  {object} shared.ApiBody "Internal Error"
// @Success 200  {object} dsmodels.DsRemoteApiScopeList[models.CodecovRepo] "the parentIds are always null"
// @Tags plugins/codecov
// @Router /plugins/codecov/connections/{connectionId}/search-remote-scopes [GET]
func SearchRemoteScopes(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	return raScopeSearch.Get(input)
}
