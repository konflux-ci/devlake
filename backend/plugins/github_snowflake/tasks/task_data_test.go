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

func TestDecodeAndValidateTaskOptions_OK(t *testing.T) {
	op, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"githubId":     123,
		"name":         "konflux-ci/build-service",
	})
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), op.ConnectionId)
	assert.Equal(t, 123, op.GithubId)
	assert.Equal(t, "konflux-ci/build-service", op.Name)
	assert.Equal(t, "konflux-ci/build-service", op.FullName)
}

func TestDecodeAndValidateTaskOptions_FullNameFallback(t *testing.T) {
	op, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"githubId":     123,
		"fullName":     "owner/repo",
	})
	assert.Nil(t, err)
	assert.Equal(t, "owner/repo", op.Name)
}

func TestDecodeAndValidateTaskOptions_MissingConnection(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"githubId": 123,
		"name":     "owner/repo",
	})
	assert.NotNil(t, err)
}

func TestDecodeAndValidateTaskOptions_MissingGithubId(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"name":         "owner/repo",
	})
	assert.NotNil(t, err)
}

func TestDecodeAndValidateTaskOptions_MissingName(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"githubId":     123,
	})
	assert.NotNil(t, err)
}
