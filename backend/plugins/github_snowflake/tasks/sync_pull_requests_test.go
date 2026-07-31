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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildPullRequestsQuery_NoTimeFilter(t *testing.T) {
	q, args := buildPullRequestsQuery(12345, nil)
	assert.Contains(t, q, "FROM PULL_REQUEST pr")
	assert.Contains(t, q, "JOIN ISSUE i")
	assert.Contains(t, q, "LEFT JOIN ISSUE_MERGED im")
	assert.Contains(t, q, `LEFT JOIN "USER" u`)
	assert.Contains(t, q, "WHERE i.REPOSITORY_ID = ?")
	assert.NotContains(t, q, "pr.UPDATED_AT >")
	assert.Equal(t, []interface{}{12345}, args)
}

func TestBuildPullRequestsQuery_WithTimeFilter(t *testing.T) {
	ts := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	q, args := buildPullRequestsQuery(12345, &ts)
	assert.Contains(t, q, "AND pr.UPDATED_AT > ?")
	assert.Equal(t, 2, len(args))
	assert.Equal(t, 12345, args[0])
	assert.Equal(t, ts, args[1])
}

func TestBuildPullRequestsQuery_RequiredColumns(t *testing.T) {
	q, _ := buildPullRequestsQuery(1, nil)
	for _, col := range []string{
		"pr.ID", "i.REPOSITORY_ID", "i.NUMBER", "i.STATE", "i.TITLE",
		"pr.UPDATED_AT", "pr.DRAFT", "pr.MERGE_COMMIT_SHA",
		"pr.HEAD_REF", "pr.BASE_REF", "i.USER_ID", "im.MERGED_AT",
	} {
		assert.Contains(t, q, col)
	}
}
