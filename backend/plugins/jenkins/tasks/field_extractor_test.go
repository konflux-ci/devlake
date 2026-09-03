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
	"testing"

	"github.com/apache/incubator-devlake/plugins/jenkins/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBuildParameters(t *testing.T) {
	body := &models.ApiBuildResponse{
		Actions: []models.Action{
			{
				Parameters: []models.Parameter{
					{Name: "OCP_VERSION", Value: "4.22"},
					{Name: "ARCH", Value: "arm64"},
				},
			},
		},
	}

	params := ExtractBuildParameters(body)
	assert.Equal(t, "4.22", params["OCP_VERSION"])
	assert.Equal(t, "arm64", params["ARCH"])
}

func TestExtractBuildParametersFromJSON(t *testing.T) {
	raw := []byte(`{
		"actions": [{
			"parameters": [
				{"name": "OCP_VERSION", "value": "4.22"},
				{"name": "DRY_RUN", "value": false},
				{"name": "BUILD_NUM", "value": 4954}
			]
		}]
	}`)

	body := &models.ApiBuildResponse{}
	require.NoError(t, json.Unmarshal(raw, body))

	params := ExtractBuildParameters(body)
	assert.Equal(t, "4.22", params["OCP_VERSION"])
	assert.Equal(t, "false", params["DRY_RUN"])
	assert.Equal(t, "4954", params["BUILD_NUM"])
}

func TestFieldExtractorApply(t *testing.T) {
	scopeConfig := &models.JenkinsScopeConfig{
		FieldExtractors: []models.FieldExtractorRule{
			{
				Key:     "ocp_version",
				Sources: []string{"parameter:OCP_VERSION"},
			},
			{
				Key:     "pipeline",
				Sources: []string{"full_name"},
				Pattern: `^(.+)#\d+$`,
				Group:   1,
			},
		},
	}

	extractor, err := NewFieldExtractor(scopeConfig, nil)
	require.NoError(t, err)

	build := &models.JenkinsBuild{
		FullName: "main-pipelines/downstream-pipeline-3#142",
		JobName:  "downstream-pipeline-3",
	}
	ctx := &BuildExtractionContext{
		FullName:   build.FullName,
		JobName:    build.JobName,
		Parameters: map[string]string{"OCP_VERSION": "4.22"},
	}

	extractor.Apply(build, ctx)

	assert.Equal(t, "4.22", build.Metadata["ocp_version"])
	assert.Equal(t, "main-pipelines/downstream-pipeline-3", build.Metadata["pipeline"])
}
