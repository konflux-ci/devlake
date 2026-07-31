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

func TestBuildPrCommitsQuery_InnerJoinsPullRequest(t *testing.T) {
	q, args := buildPrCommitsQuery(99, nil)
	assert.Contains(t, q, "JOIN PULL_REQUEST pr")
	assert.Contains(t, q, "JOIN COMMIT c")
	assert.Contains(t, q, "FROM COMMIT_PULL_REQUEST cpr")
	assert.Contains(t, q, "WHERE i.REPOSITORY_ID = ?")
	assert.NotContains(t, q, "AUTHOR_DATE >")
	assert.Equal(t, []interface{}{99}, args)
}

func TestBuildPrCommitsQuery_WithTimeFilter(t *testing.T) {
	ts := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	q, args := buildPrCommitsQuery(99, &ts)
	assert.Contains(t, q, "AND c.AUTHOR_DATE > ?")
	assert.Equal(t, ts, args[1])
}
