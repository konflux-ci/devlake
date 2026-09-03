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
	"regexp"
	"strings"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/plugins/jenkins/models"
)

// FieldExtractor applies scope-configured extraction rules to Jenkins builds.
type FieldExtractor struct {
	rules []compiledFieldExtractorRule
}

type compiledFieldExtractorRule struct {
	models.FieldExtractorRule
	regex *regexp.Regexp
}

// BuildExtractionContext holds source values used by field extractors.
type BuildExtractionContext struct {
	FullName    string
	JobName     string
	TriggeredBy string
	Parameters  map[string]string
}

// NewFieldExtractor compiles scope-config extraction rules.
func NewFieldExtractor(scopeConfig *models.JenkinsScopeConfig, logger log.Logger) (*FieldExtractor, errors.Error) {
	if scopeConfig == nil || len(scopeConfig.FieldExtractors) == 0 {
		return &FieldExtractor{}, nil
	}

	extractor := &FieldExtractor{
		rules: make([]compiledFieldExtractorRule, 0, len(scopeConfig.FieldExtractors)),
	}

	for _, rule := range scopeConfig.FieldExtractors {
		if rule.Key == "" {
			continue
		}
		if len(rule.Sources) == 0 {
			if logger != nil {
				logger.Warn(nil, "field extractor rule has no sources, skipping", "key", rule.Key)
			}
			continue
		}

		compiled := compiledFieldExtractorRule{FieldExtractorRule: rule}
		if rule.Pattern != "" {
			regex, err := regexp.Compile(rule.Pattern)
			if err != nil {
				return nil, errors.BadInput.Wrap(err, "invalid field extractor pattern for key "+rule.Key)
			}
			compiled.regex = regex
		}
		extractor.rules = append(extractor.rules, compiled)
	}

	return extractor, nil
}

// Apply enriches a Jenkins build metadata map using configured rules.
func (f *FieldExtractor) Apply(build *models.JenkinsBuild, ctx *BuildExtractionContext) {
	if build == nil || ctx == nil || len(f.rules) == 0 {
		return
	}

	if build.Metadata == nil {
		build.Metadata = make(map[string]string)
	}

	for _, rule := range f.rules {
		f.applyRule(build, ctx, rule)
	}
}

func (f *FieldExtractor) applyRule(build *models.JenkinsBuild, ctx *BuildExtractionContext, rule compiledFieldExtractorRule) {
	if rule.OnlyIfEmpty {
		if existing, ok := build.Metadata[rule.Key]; ok && existing != "" {
			return
		}
	}

	value := f.extractValue(ctx, rule)
	if value == "" {
		value = rule.Default
	}
	if value == "" {
		return
	}

	build.Metadata[rule.Key] = value
}

func (f *FieldExtractor) extractValue(ctx *BuildExtractionContext, rule compiledFieldExtractorRule) string {
	for _, source := range rule.Sources {
		sourceValue := resolveBuildSourceValue(ctx, source)
		if sourceValue == "" {
			continue
		}

		if rule.regex == nil {
			return sourceValue
		}

		matches := rule.regex.FindStringSubmatch(sourceValue)
		if len(matches) == 0 {
			continue
		}

		group := rule.Group
		if group <= 0 {
			return matches[0]
		}
		if group < len(matches) {
			return matches[group]
		}
	}
	return ""
}

func resolveBuildSourceValue(ctx *BuildExtractionContext, source string) string {
	switch source {
	case "full_name":
		return ctx.FullName
	case "job_name":
		return ctx.JobName
	case "triggered_by":
		return ctx.TriggeredBy
	default:
		if strings.HasPrefix(source, "parameter:") && ctx.Parameters != nil {
			return ctx.Parameters[strings.TrimPrefix(source, "parameter:")]
		}
	}
	return ""
}

// ExtractBuildParameters reads parameter actions from a Jenkins build API response.
func ExtractBuildParameters(body *models.ApiBuildResponse) map[string]string {
	parameters := make(map[string]string)
	if body == nil {
		return parameters
	}

	for _, action := range body.Actions {
		for _, parameter := range action.Parameters {
			if parameter.Name == "" {
				continue
			}
			parameters[parameter.Name] = parameter.Value
		}
	}
	return parameters
}
