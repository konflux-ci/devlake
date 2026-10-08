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
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/merico-dev/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQuerier implements graphqlQuerier for testing.
type mockQuerier struct {
	pages [][]GraphqlQueryReview
	call  int
	err   error
}

func (m *mockQuerier) Query(q interface{}, _ map[string]interface{}) ([]graphql.DataError, error) {
	if m.err != nil {
		return nil, m.err
	}
	wrapper := q.(*GraphqlQueryPrReviewsWrapper)
	page := m.pages[m.call]
	m.call++
	wrapper.Repository.PullRequest.Reviews.Nodes = page
	hasNext := m.call < len(m.pages)
	wrapper.Repository.PullRequest.Reviews.PageInfo = &api.GraphqlQueryPageInfo{
		HasNextPage: hasNext,
		EndCursor:   fmt.Sprintf("cursor%d", m.call),
	}
	return nil, nil
}

// nilPageInfoQuerier returns responses with a nil PageInfo.
type nilPageInfoQuerier struct{}

func (n *nilPageInfoQuerier) Query(q interface{}, _ map[string]interface{}) ([]graphql.DataError, error) {
	wrapper := q.(*GraphqlQueryPrReviewsWrapper)
	wrapper.Repository.PullRequest.Reviews.Nodes = nil
	wrapper.Repository.PullRequest.Reviews.PageInfo = nil
	return nil, nil
}

func makeReview(id int) GraphqlQueryReview {
	return GraphqlQueryReview{DatabaseId: id, State: "APPROVED"}
}

func makeReviews(count int) []GraphqlQueryReview {
	reviews := make([]GraphqlQueryReview, count)
	for i := range reviews {
		reviews[i] = makeReview(i + 1)
	}
	return reviews
}

// ── Group A: fetchRemainingReviews pagination ─────────────────────────────────

func TestFetchRemainingReviews_SinglePage(t *testing.T) {
	mock := &mockQuerier{pages: [][]GraphqlQueryReview{makeReviews(2)}}
	got, err := fetchRemainingReviews(mock, "owner", "repo", 1, "cursor0")
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.Equal(t, 1, mock.call)
}

func TestFetchRemainingReviews_TwoPages(t *testing.T) {
	mock := &mockQuerier{pages: [][]GraphqlQueryReview{makeReviews(100), makeReviews(2)}}
	got, err := fetchRemainingReviews(mock, "owner", "repo", 1, "cursor0")
	require.NoError(t, err)
	assert.Len(t, got, 102)
	assert.Equal(t, 2, mock.call)
}

func TestFetchRemainingReviews_NilPageInfo(t *testing.T) {
	// When the server returns nil PageInfo the loop must terminate immediately.
	mock := &nilPageInfoQuerier{}
	got, err := fetchRemainingReviews(mock, "owner", "repo", 1, "cursor0")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestFetchRemainingReviews_QueryError(t *testing.T) {
	mock := &mockQuerier{
		pages: [][]GraphqlQueryReview{makeReviews(1)},
		err:   fmt.Errorf("network error"),
	}
	_, err := fetchRemainingReviews(mock, "owner", "repo", 1, "cursor0")
	assert.Error(t, err)
}

// ── Group B: ResponseParser since/updatedAt stop condition ────────────────────

// stopWhen replicates the stop-condition logic from CollectPrs ResponseParser.
func stopWhen(since *time.Time, prs []GraphqlQueryPr) ([]json.RawMessage, errors.Error) {
	var msgs []json.RawMessage
	for _, pr := range prs {
		if since != nil && since.After(pr.UpdatedAt) {
			return msgs, api.ErrFinishCollect
		}
		msgs = append(msgs, errors.Must1(json.Marshal(pr)))
	}
	return msgs, nil
}

func makePR(updatedAt time.Time) GraphqlQueryPr {
	return GraphqlQueryPr{UpdatedAt: updatedAt}
}

func TestResponseParser_SinceNil_AllIncluded(t *testing.T) {
	t0 := time.Now().Add(-2 * time.Hour)
	t1 := time.Now().Add(-1 * time.Hour)
	msgs, err := stopWhen(nil, []GraphqlQueryPr{makePR(t1), makePR(t0)})
	require.NoError(t, err)
	assert.Len(t, msgs, 2)
}

func TestResponseParser_SinceAfterAll_StopsImmediately(t *testing.T) {
	t0 := time.Now().Add(-2 * time.Hour)
	t1 := time.Now().Add(-1 * time.Hour)
	since := time.Now() // newer than all PRs
	msgs, err := stopWhen(&since, []GraphqlQueryPr{makePR(t1), makePR(t0)})
	assert.ErrorIs(t, err, api.ErrFinishCollect)
	assert.Empty(t, msgs)
}

func TestResponseParser_SinceBetween_Partial(t *testing.T) {
	t0 := time.Now().Add(-2 * time.Hour)
	t1 := time.Now().Add(-1 * time.Hour)
	since := t0.Add(30 * time.Minute) // between t0 and t1; t0 is older so since.After(t0)=true
	msgs, err := stopWhen(&since, []GraphqlQueryPr{makePR(t1), makePR(t0)})
	assert.ErrorIs(t, err, api.ErrFinishCollect)
	assert.Len(t, msgs, 1) // only t1 (newer) emitted
}

func TestResponseParser_SinceNil_EmptyList(t *testing.T) {
	msgs, err := stopWhen(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, msgs)
}

// ── Group C: review merging via PageInfo ──────────────────────────────────────

// mergeReviews replicates the review-pagination guard used in both ResponseParsers.
func mergeReviews(pr *GraphqlQueryPr, client graphqlQuerier, owner, repo string) errors.Error {
	if pr.Reviews.PageInfo != nil && pr.Reviews.PageInfo.HasNextPage {
		more, err := fetchRemainingReviews(client, owner, repo, pr.Number, pr.Reviews.PageInfo.EndCursor)
		if err != nil {
			return err
		}
		pr.Reviews.Nodes = append(pr.Reviews.Nodes, more...)
	}
	return nil
}

func TestMergeReviews_NoPagination(t *testing.T) {
	pr := &GraphqlQueryPr{
		Number: 1,
		Reviews: struct {
			TotalCount graphql.Int
			PageInfo   *api.GraphqlQueryPageInfo
			Nodes      []GraphqlQueryReview `graphql:"nodes"`
		}{
			PageInfo: &api.GraphqlQueryPageInfo{HasNextPage: false},
			Nodes:    makeReviews(5),
		},
	}
	mock := &mockQuerier{pages: [][]GraphqlQueryReview{makeReviews(10)}}
	err := mergeReviews(pr, mock, "owner", "repo")
	require.NoError(t, err)
	assert.Len(t, pr.Reviews.Nodes, 5) // not extended
	assert.Equal(t, 0, mock.call)       // no query made
}

func TestMergeReviews_OneExtraPage(t *testing.T) {
	pr := &GraphqlQueryPr{
		Number: 42,
		Reviews: struct {
			TotalCount graphql.Int
			PageInfo   *api.GraphqlQueryPageInfo
			Nodes      []GraphqlQueryReview `graphql:"nodes"`
		}{
			PageInfo: &api.GraphqlQueryPageInfo{HasNextPage: true, EndCursor: "c0"},
			Nodes:    makeReviews(100),
		},
	}
	mock := &mockQuerier{pages: [][]GraphqlQueryReview{makeReviews(7)}}
	err := mergeReviews(pr, mock, "owner", "repo")
	require.NoError(t, err)
	assert.Len(t, pr.Reviews.Nodes, 107)
	assert.Equal(t, 1, mock.call)
}

func TestMergeReviews_ErrorOnExtraPage(t *testing.T) {
	pr := &GraphqlQueryPr{
		Number: 99,
		Reviews: struct {
			TotalCount graphql.Int
			PageInfo   *api.GraphqlQueryPageInfo
			Nodes      []GraphqlQueryReview `graphql:"nodes"`
		}{
			PageInfo: &api.GraphqlQueryPageInfo{HasNextPage: true, EndCursor: "c0"},
			Nodes:    makeReviews(10),
		},
	}
	mock := &mockQuerier{
		pages: [][]GraphqlQueryReview{makeReviews(1)},
		err:   fmt.Errorf("timeout"),
	}
	err := mergeReviews(pr, mock, "owner", "repo")
	assert.Error(t, err)
}
