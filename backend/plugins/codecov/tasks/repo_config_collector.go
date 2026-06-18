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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
	"gopkg.in/yaml.v3"
)

var CollectRepoConfigMeta = plugin.SubTaskMeta{
	Name:             "CollectRepoConfig",
	EntryPoint:       CollectRepoConfig,
	EnabledByDefault: true,
	Description:      "Fetch codecov.yml from the repository and parse coverage thresholds",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
}

var configFileNames = []string{
	".codecov.yml",
	"codecov.yml",
	".codecov.yaml",
	"codecov.yaml",
}

// codecovYaml represents the relevant parts of a codecov.yml file
type codecovYaml struct {
	Coverage struct {
		Status struct {
			Project map[string]statusConfig `yaml:"project"`
			Patch   map[string]statusConfig `yaml:"patch"`
		} `yaml:"status"`
	} `yaml:"coverage"`
}

type statusConfig struct {
	Target        interface{} `yaml:"target"`
	Threshold     interface{} `yaml:"threshold"`
	Informational interface{} `yaml:"informational"`
}

func CollectRepoConfig(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()
	logger := taskCtx.GetLogger()

	owner, repo, err := ParseFullName(data.Options.FullName)
	if err != nil {
		return err
	}

	if data.Repo == nil {
		logger.Warn(nil, "[Codecov] CollectRepoConfig: No repo data available for %s, skipping", data.Options.FullName)
		return nil
	}

	service := data.Repo.Service
	if service == "" {
		service = "github"
	}
	branch := data.Repo.Branch
	if branch == "" {
		logger.Warn(nil, "[Codecov] CollectRepoConfig: No branch configured for %s/%s, skipping", owner, repo)
		return nil
	}

	if data.Repo.Private {
		logger.Info("[Codecov] CollectRepoConfig: Skipping private repo %s/%s", owner, repo)
		return nil
	}

	logger.Info("[Codecov] CollectRepoConfig: Fetching codecov config for %s/%s (service=%s, branch=%s)", owner, repo, service, branch)

	var foundFile string
	var rawYaml string

	for _, filename := range configFileNames {
		rawURL := buildRawContentURL(service, owner, repo, branch, filename)
		if rawURL == "" {
			logger.Warn(nil, "[Codecov] CollectRepoConfig: unsupported service %q, skipping", service)
			break
		}

		body, fetchErr := fetchFile(taskCtx.GetContext(), rawURL)
		if fetchErr != nil {
			logger.Info("[Codecov] CollectRepoConfig: %s not found at %s", filename, rawURL)
			continue
		}

		foundFile = filename
		rawYaml = body
		logger.Info("[Codecov] CollectRepoConfig: Found %s for %s/%s (%d bytes)", filename, owner, repo, len(body))
		break
	}

	if foundFile == "" {
		logger.Info("[Codecov] CollectRepoConfig: No codecov config file found for %s/%s", owner, repo)
		return nil
	}

	config := parseCodecovYaml(rawYaml)

	repoConfig := &models.CodecovRepoConfig{
		NoPKModel:            common.NoPKModel{},
		ConnectionId:         data.Options.ConnectionId,
		RepoId:               data.Options.FullName,
		ConfigSource:         foundFile,
		RawYaml:              rawYaml,
		ProjectTarget:        config.projectTarget,
		ProjectTargetAuto:    config.projectTargetAuto,
		ProjectThreshold:     config.projectThreshold,
		ProjectInformational: config.projectInformational,
		PatchTarget:          config.patchTarget,
		PatchTargetAuto:      config.patchTargetAuto,
		PatchThreshold:       config.patchThreshold,
		PatchInformational:   config.patchInformational,
	}

	if err := db.CreateOrUpdate(repoConfig); err != nil {
		return errors.Default.Wrap(err, "failed to save repo config")
	}

	logger.Info("[Codecov] CollectRepoConfig: Saved config for %s/%s (patch_target=%v, project_target=%v)",
		owner, repo, config.patchTarget, config.projectTarget)
	return nil
}

func buildRawContentURL(service, owner, repo, branch, filename string) string {
	switch service {
	case "github":
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, filename)
	case "gitlab":
		projectPath := url.PathEscape(owner + "/" + repo)
		encodedFile := url.PathEscape(filename)
		return fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/repository/files/%s/raw?ref=%s", projectPath, encodedFile, branch)
	case "gitlab_enterprise":
		projectPath := url.PathEscape(owner + "/" + repo)
		encodedFile := url.PathEscape(filename)
		return fmt.Sprintf("https://gitlab.cee.redhat.com/api/v4/projects/%s/repository/files/%s/raw?ref=%s", projectPath, encodedFile, branch)
	default:
		return ""
	}
}

const fetchTimeout = 15 * time.Second
const maxConfigSize = 1 << 20 // 1 MiB

func fetchFile(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxConfigSize))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type parsedConfig struct {
	projectTarget        *float64
	projectTargetAuto    bool
	projectThreshold     *float64
	projectInformational bool
	patchTarget          *float64
	patchTargetAuto      bool
	patchThreshold       *float64
	patchInformational   bool
}

func parseCodecovYaml(raw string) parsedConfig {
	var cfg codecovYaml
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		return parsedConfig{}
	}

	result := parsedConfig{}

	if def, ok := cfg.Coverage.Status.Project["default"]; ok {
		result.projectTarget, result.projectTargetAuto = parseTarget(def.Target)
		result.projectThreshold = parsePercentage(def.Threshold)
		result.projectInformational = parseBool(def.Informational)
	}

	if def, ok := cfg.Coverage.Status.Patch["default"]; ok {
		result.patchTarget, result.patchTargetAuto = parseTarget(def.Target)
		result.patchThreshold = parsePercentage(def.Threshold)
		result.patchInformational = parseBool(def.Informational)
	}

	return result
}

// parseTarget handles target values: "auto", "80%", 80, 80.0
func parseTarget(v interface{}) (target *float64, isAuto bool) {
	if v == nil {
		return nil, false
	}
	switch val := v.(type) {
	case string:
		val = strings.TrimSpace(val)
		if strings.EqualFold(val, "auto") {
			return nil, true
		}
		val = strings.TrimSuffix(val, "%")
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return &f, false
		}
	case int:
		f := float64(val)
		return &f, false
	case float64:
		return &val, false
	}
	return nil, false
}

// parsePercentage handles threshold values: "1%", 1, 1.0
func parsePercentage(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		val = strings.TrimSpace(strings.TrimSuffix(val, "%"))
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return &f
		}
	case int:
		f := float64(val)
		return &f
	case float64:
		return &val
	}
	return nil
}

func parseBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return strings.EqualFold(val, "true")
	}
	return false
}
