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

func TestBuildReviewersQuery_FiltersUsersAndLatest(t *testing.T) {
	q, args := buildReviewersQuery(42, nil)
	assert.Contains(t, q, "FROM REQUESTED_REVIEWER_HISTORY h")
	assert.Contains(t, q, "LOWER(h.REQUESTED_REVIEWER_TYPE) = 'user'")
	assert.Contains(t, q, "QUALIFY ROW_NUMBER()")
	assert.Contains(t, q, "PARTITION BY h.PULL_REQUEST_ID, h.REQUESTED_ID")
	// REMOVED must be filtered after QUALIFY so a later removal wins.
	assert.Contains(t, q, "WHERE (latest.removed IS NULL OR latest.removed = FALSE)")
	assert.NotContains(t, q, "AND (h.REMOVED IS NULL OR h.REMOVED = FALSE)")
	assert.Equal(t, []interface{}{42}, args)
}

func TestBuildReviewersQuery_WithTimeFilter(t *testing.T) {
	ts := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	q, args := buildReviewersQuery(42, &ts)
	assert.Contains(t, q, "AND h.CREATED_AT > ?")
	assert.Equal(t, ts, args[1])
}
