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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- buildRawContentURL ---

func TestBuildRawContentURL_GitHub(t *testing.T) {
	url := buildRawContentURL("github", "konflux-ci", "build-service", "main", ".codecov.yml")
	assert.Equal(t, "https://raw.githubusercontent.com/konflux-ci/build-service/main/.codecov.yml", url)
}

func TestBuildRawContentURL_GitLab(t *testing.T) {
	url := buildRawContentURL("gitlab", "konflux-ci", "build-service", "main", "codecov.yml")
	assert.Contains(t, url, "gitlab.com/api/v4/projects/")
	assert.Contains(t, url, "raw?ref=main")
}

func TestBuildRawContentURL_GitLabEnterprise(t *testing.T) {
	url := buildRawContentURL("gitlab_enterprise", "myorg", "myrepo", "develop", ".codecov.yml")
	assert.Contains(t, url, "gitlab.cee.redhat.com/api/v4/projects/")
	assert.Contains(t, url, "raw?ref=develop")
}

func TestBuildRawContentURL_UnsupportedService(t *testing.T) {
	url := buildRawContentURL("bitbucket", "owner", "repo", "main", "codecov.yml")
	assert.Equal(t, "", url)
}

// --- fetchFile ---

func TestFetchFile_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("coverage:\n  status:\n    patch:\n      default:\n        target: 80%\n"))
	}))
	defer ts.Close()

	body, err := fetchFile(context.Background(), ts.URL)
	assert.NoError(t, err)
	assert.Contains(t, body, "target: 80%")
}

func TestFetchFile_404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	_, err := fetchFile(context.Background(), ts.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestFetchFile_BadURL(t *testing.T) {
	_, err := fetchFile(context.Background(), "http://127.0.0.1:1/nonexistent")
	assert.Error(t, err)
}

func TestFetchFile_CancelledContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchFile(ctx, ts.URL)
	assert.Error(t, err)
}

// --- parseCodecovYaml ---

func TestParseCodecovYaml_FullConfig(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: 80%
        threshold: 1%
        informational: false
    patch:
      default:
        target: 90%
        threshold: 5%
        informational: true
`
	cfg := parseCodecovYaml(raw)

	assert.NotNil(t, cfg.projectTarget)
	assert.Equal(t, 80.0, *cfg.projectTarget)
	assert.False(t, cfg.projectTargetAuto)
	assert.NotNil(t, cfg.projectThreshold)
	assert.Equal(t, 1.0, *cfg.projectThreshold)
	assert.False(t, cfg.projectInformational)

	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 90.0, *cfg.patchTarget)
	assert.False(t, cfg.patchTargetAuto)
	assert.NotNil(t, cfg.patchThreshold)
	assert.Equal(t, 5.0, *cfg.patchThreshold)
	assert.True(t, cfg.patchInformational)
}

func TestParseCodecovYaml_AutoTarget(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: auto
    patch:
      default:
        target: auto
`
	cfg := parseCodecovYaml(raw)

	assert.Nil(t, cfg.projectTarget)
	assert.True(t, cfg.projectTargetAuto)
	assert.Nil(t, cfg.patchTarget)
	assert.True(t, cfg.patchTargetAuto)
}

func TestParseCodecovYaml_NumericTargets(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      default:
        target: 75
        threshold: 2
    patch:
      default:
        target: 85.5
        threshold: 0.5
`
	cfg := parseCodecovYaml(raw)

	assert.NotNil(t, cfg.projectTarget)
	assert.Equal(t, 75.0, *cfg.projectTarget)
	assert.NotNil(t, cfg.projectThreshold)
	assert.Equal(t, 2.0, *cfg.projectThreshold)

	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 85.5, *cfg.patchTarget)
	assert.NotNil(t, cfg.patchThreshold)
	assert.Equal(t, 0.5, *cfg.patchThreshold)
}

func TestParseCodecovYaml_Empty(t *testing.T) {
	cfg := parseCodecovYaml("")
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
	assert.False(t, cfg.projectTargetAuto)
	assert.False(t, cfg.patchTargetAuto)
}

func TestParseCodecovYaml_InvalidYaml(t *testing.T) {
	cfg := parseCodecovYaml("{{{{not yaml")
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
}

func TestParseCodecovYaml_NoDefault(t *testing.T) {
	raw := `
coverage:
  status:
    project:
      custom-status:
        target: 50%
    patch:
      custom-status:
        target: 60%
`
	cfg := parseCodecovYaml(raw)
	assert.Nil(t, cfg.projectTarget)
	assert.Nil(t, cfg.patchTarget)
}

func TestParseCodecovYaml_OnlyPatch(t *testing.T) {
	raw := `
coverage:
  status:
    patch:
      default:
        target: 70%
        informational: true
`
	cfg := parseCodecovYaml(raw)
	assert.Nil(t, cfg.projectTarget)
	assert.NotNil(t, cfg.patchTarget)
	assert.Equal(t, 70.0, *cfg.patchTarget)
	assert.True(t, cfg.patchInformational)
	assert.False(t, cfg.projectInformational)
}

// --- parseTarget ---

func TestParseTarget_Nil(t *testing.T) {
	target, auto := parseTarget(nil)
	assert.Nil(t, target)
	assert.False(t, auto)
}

func TestParseTarget_Auto(t *testing.T) {
	target, auto := parseTarget("auto")
	assert.Nil(t, target)
	assert.True(t, auto)
}

func TestParseTarget_AutoUpperCase(t *testing.T) {
	target, auto := parseTarget("AUTO")
	assert.Nil(t, target)
	assert.True(t, auto)
}

func TestParseTarget_StringPercent(t *testing.T) {
	target, auto := parseTarget("80%")
	assert.NotNil(t, target)
	assert.Equal(t, 80.0, *target)
	assert.False(t, auto)
}

func TestParseTarget_StringNumber(t *testing.T) {
	target, auto := parseTarget("75.5")
	assert.NotNil(t, target)
	assert.Equal(t, 75.5, *target)
	assert.False(t, auto)
}

func TestParseTarget_Int(t *testing.T) {
	target, auto := parseTarget(80)
	assert.NotNil(t, target)
	assert.Equal(t, 80.0, *target)
	assert.False(t, auto)
}

func TestParseTarget_Float(t *testing.T) {
	target, auto := parseTarget(85.5)
	assert.NotNil(t, target)
	assert.Equal(t, 85.5, *target)
	assert.False(t, auto)
}

func TestParseTarget_InvalidString(t *testing.T) {
	target, auto := parseTarget("not-a-number")
	assert.Nil(t, target)
	assert.False(t, auto)
}

func TestParseTarget_StringWithSpaces(t *testing.T) {
	target, auto := parseTarget("  90%  ")
	assert.NotNil(t, target)
	assert.Equal(t, 90.0, *target)
	assert.False(t, auto)
}

// --- parsePercentage ---

func TestParsePercentage_Nil(t *testing.T) {
	assert.Nil(t, parsePercentage(nil))
}

func TestParsePercentage_StringPercent(t *testing.T) {
	result := parsePercentage("1%")
	assert.NotNil(t, result)
	assert.Equal(t, 1.0, *result)
}

func TestParsePercentage_StringNumber(t *testing.T) {
	result := parsePercentage("5.5")
	assert.NotNil(t, result)
	assert.Equal(t, 5.5, *result)
}

func TestParsePercentage_Int(t *testing.T) {
	result := parsePercentage(3)
	assert.NotNil(t, result)
	assert.Equal(t, 3.0, *result)
}

func TestParsePercentage_Float(t *testing.T) {
	result := parsePercentage(2.5)
	assert.NotNil(t, result)
	assert.Equal(t, 2.5, *result)
}

func TestParsePercentage_InvalidString(t *testing.T) {
	assert.Nil(t, parsePercentage("abc"))
}

// --- parseBool ---

func TestParseBool_Nil(t *testing.T) {
	assert.False(t, parseBool(nil))
}

func TestParseBool_True(t *testing.T) {
	assert.True(t, parseBool(true))
}

func TestParseBool_False(t *testing.T) {
	assert.False(t, parseBool(false))
}

func TestParseBool_StringTrue(t *testing.T) {
	assert.True(t, parseBool("true"))
	assert.True(t, parseBool("TRUE"))
	assert.True(t, parseBool("True"))
}

func TestParseBool_StringFalse(t *testing.T) {
	assert.False(t, parseBool("false"))
	assert.False(t, parseBool("something"))
}

func TestParseBool_OtherType(t *testing.T) {
	assert.False(t, parseBool(42))
}
