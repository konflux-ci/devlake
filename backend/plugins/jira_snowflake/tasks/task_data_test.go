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
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// DecodeAndValidateTaskOptions
// ---------------------------------------------------------------------------

func TestDecodeAndValidateTaskOptions_Valid(t *testing.T) {
	opts, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
		"projectKeys":  []string{"KONFLUX", "HELM"},
	})
	require.Nil(t, err)
	assert.Equal(t, uint64(1), opts.ConnectionId)
	assert.Equal(t, uint64(42), opts.BoardId)
	assert.Equal(t, []string{"KONFLUX", "HELM"}, opts.ProjectKeys)
}

func TestDecodeAndValidateTaskOptions_MissingConnectionId(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"boardId":     uint64(42),
		"projectKeys": []string{"KONFLUX"},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "connectionId")
}

func TestDecodeAndValidateTaskOptions_MissingBoardId(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"projectKeys":  []string{"KONFLUX"},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "boardId")
}

func TestDecodeAndValidateTaskOptions_EmptyProjectKeys(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
		"projectKeys":  []string{},
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "projectKeys")
}

func TestDecodeAndValidateTaskOptions_MissingProjectKeys(t *testing.T) {
	_, err := DecodeAndValidateTaskOptions(map[string]interface{}{
		"connectionId": uint64(1),
		"boardId":      uint64(42),
	})
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "projectKeys")
}

// ---------------------------------------------------------------------------
// stringVal
// ---------------------------------------------------------------------------

func TestStringVal(t *testing.T) {
	t.Run("nil pointer returns empty string", func(t *testing.T) {
		assert.Equal(t, "", stringVal(nil))
	})
	t.Run("non-nil pointer returns value", func(t *testing.T) {
		s := "hello"
		assert.Equal(t, "hello", stringVal(&s))
	})
	t.Run("empty string pointer returns empty string", func(t *testing.T) {
		s := ""
		assert.Equal(t, "", stringVal(&s))
	})
}
