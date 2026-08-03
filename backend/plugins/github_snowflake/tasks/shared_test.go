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

func TestDeriveRepoHTMLUrl(t *testing.T) {
	assert.Equal(t, "https://github.com/konflux-ci/build-service", deriveRepoHTMLUrl("konflux-ci/build-service"))
}

func TestDeriveRepoCloneUrl(t *testing.T) {
	assert.Equal(t, "https://github.com/konflux-ci/build-service.git", deriveRepoCloneUrl("konflux-ci/build-service"))
}

func TestDerivePullRequestURL(t *testing.T) {
	assert.Equal(t, "https://github.com/konflux-ci/build-service/pull/42", derivePullRequestURL("konflux-ci/build-service", 42))
}

func TestNullStr(t *testing.T) {
	assert.Equal(t, "", nullStr(nil))
	s := "hello"
	assert.Equal(t, "hello", nullStr(&s))
}

func TestNullInt(t *testing.T) {
	assert.Equal(t, 0, nullInt(nil))
	v := int64(99)
	assert.Equal(t, 99, nullInt(&v))
}

func TestRepoShortName(t *testing.T) {
	assert.Equal(t, "build-service", RepoShortName("konflux-ci/build-service"))
	assert.Equal(t, "alone", RepoShortName("alone"))
}
