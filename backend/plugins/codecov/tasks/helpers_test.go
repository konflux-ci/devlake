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
	"github.com/stretchr/testify/mock"
)

func TestParseFullName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"standard github format", "ansible/ansible-ui", "ansible", "ansible-ui", false},
		{"org with hyphens", "konflux-ci/build-service", "konflux-ci", "build-service", false},
		{"simple names", "owner/repo", "owner", "repo", false},
		{"empty string", "", "", "", true},
		{"no slash", "noslash", "", "", true},
		{"too many slashes", "a/b/c", "", "", true},
		{"only slash", "/", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseFullName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFullName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner {
				t.Errorf("ParseFullName(%q) owner = %q, want %q", tt.input, owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("ParseFullName(%q) repo = %q, want %q", tt.input, repo, tt.wantRepo)
			}
		})
	}
}

func TestDeleteConvertedRaw_NoTable(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_commit_coverages").Return(false)

	err := deleteConvertedRaw(mockCtx, RAW_COMMIT_COVERAGES_TABLE, CodecovApiParams{
		ConnectionId: 1,
		Name:         "owner/repo",
	})
	assert.Nil(t, err)
	mockDal.AssertNotCalled(t, "Delete")
}

func TestDeleteConvertedRaw_DeletesMatchingParams(t *testing.T) {
	mockCtx, mockDal, _ := setupCodecovMocks(t)
	mockDal.On("HasTable", "_raw_codecov_api_commit_coverages").Return(true)
	mockDal.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	err := deleteConvertedRaw(mockCtx, RAW_COMMIT_COVERAGES_TABLE, CodecovApiParams{
		ConnectionId: 1,
		Name:         "owner/repo",
	})
	assert.Nil(t, err)
	mockDal.AssertCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}
