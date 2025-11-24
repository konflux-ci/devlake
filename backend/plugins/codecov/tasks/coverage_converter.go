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

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

var ConvertCoverageMeta = plugin.SubTaskMeta{
	Name:             "ConvertCoverage",
	EntryPoint:       ConvertCoverage,
	EnabledByDefault: true,
	Description:      "Convert coverage data per flag per commit from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertComparisonMeta},
	DependencyTables: []string{RAW_COMMIT_COVERAGES_TABLE},
}

func ConvertCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_COVERAGES_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get flag name and commit SHA
			var input CommitFlagInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

			// Parse API response - when flag is specified, response contains only totals for that flag
			var totals struct {
				Commitid string `json:"commitid"`
				Totals   struct {
					Files      int     `json:"files"`
					Lines      int     `json:"lines"`
					Hits       int     `json:"hits"`
					Misses     int     `json:"misses"`
					Partials   int     `json:"partials"`
					Coverage   float64 `json:"coverage"`
					Branches   int     `json:"branches"`
					Methods    int     `json:"methods"`
					Messages   int     `json:"messages"`
					Sessions   int     `json:"sessions"`
					Complexity float64 `json:"complexity"`
				} `json:"totals"`
				// Flags map may or may not be present depending on API response
				Flags map[string]struct {
					Files      int     `json:"files"`
					Lines      int     `json:"lines"`
					Hits       int     `json:"hits"`
					Misses     int     `json:"misses"`
					Partials   int     `json:"partials"`
					Coverage   float64 `json:"coverage"`
					Branches   int     `json:"branches"`
					Methods    int     `json:"methods"`
					Messages   int     `json:"messages"`
					Sessions   int     `json:"sessions"`
					Complexity float64 `json:"complexity"`
				} `json:"flags"`
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &totals))
			if err != nil {
				return nil, err
			}

			// Use commit SHA from input (more reliable)
			commitSha := input.CommitSha
			if totals.Commitid != "" {
				commitSha = totals.Commitid
			}

			// Get commit info to extract branch and timestamp
			var commit models.CodecovCommit
			db := taskCtx.GetDal()
			err = db.First(&commit, dal.Where("connection_id = ? AND repo_id = ? AND commit_sha = ?", data.Options.ConnectionId, data.Options.FullName, commitSha))
			if err != nil {
				// If commit not found, skip this record
				return nil, nil
			}

			var results []interface{}

			// Determine which flag this coverage is for
			flagName := input.FlagName

			// Extract coverage data - when flag is specified, API returns flag-specific in totals
			// When no flag, API returns overall coverage in totals
			var coveragePercentage float64
			var linesCovered, linesTotal, linesMissed int
			var hits, partials, misses int
			var methodsCovered, methodsTotal int

			if flagName != "" && totals.Flags != nil {
				// Check if flag-specific data exists in flags map
				if flagTotals, ok := totals.Flags[flagName]; ok {
					coveragePercentage = flagTotals.Coverage
					linesCovered = flagTotals.Hits
					linesTotal = flagTotals.Lines
					linesMissed = flagTotals.Misses
					hits = flagTotals.Hits
					partials = flagTotals.Partials
					misses = flagTotals.Misses
					methodsCovered = flagTotals.Methods
					methodsTotal = flagTotals.Methods
				} else {
					// Flag not in map, use totals (API returned flag-specific in totals)
					coveragePercentage = totals.Totals.Coverage
					linesCovered = totals.Totals.Hits
					linesTotal = totals.Totals.Lines
					linesMissed = totals.Totals.Misses
					hits = totals.Totals.Hits
					partials = totals.Totals.Partials
					misses = totals.Totals.Misses
					methodsCovered = totals.Totals.Methods
					methodsTotal = totals.Totals.Methods
				}
			} else {
				// No flag specified, use overall totals
				coveragePercentage = totals.Totals.Coverage
				linesCovered = totals.Totals.Hits
				linesTotal = totals.Totals.Lines
				linesMissed = totals.Totals.Misses
				hits = totals.Totals.Hits
				partials = totals.Totals.Partials
				misses = totals.Totals.Misses
				methodsCovered = totals.Totals.Methods
				methodsTotal = totals.Totals.Methods
			}

			// Get modified coverage from comparison data if available (per flag)
			var modifiedCoverage float64
			var comparison ComparisonData
			err = db.First(&comparison, dal.Where("connection_id = ? AND repo_id = ? AND commit_sha = ? AND flag_name = ?", data.Options.ConnectionId, data.Options.FullName, commitSha, flagName))
			if err == nil {
				modifiedCoverage = comparison.ModifiedCoverage
			}

			// Create one coverage record for this flag/commit combination
			results = append(results, &models.CodecovCoverage{
				Model:              common.Model{},
				ConnectionId:       data.Options.ConnectionId,
				RepoId:             data.Options.FullName,
				FlagName:           flagName,
				Branch:             commit.Branch,
				CommitSha:          commitSha,
				CommitTimestamp:    commit.CommitTimestamp,
				CoveragePercentage: coveragePercentage,
				ModifiedCoverage:   modifiedCoverage,
				LinesCovered:       linesCovered,
				LinesTotal:         linesTotal,
				LinesMissed:        linesMissed,
				Hits:               hits,
				Partials:           partials,
				Misses:             misses,
				MethodsCovered:     methodsCovered,
				MethodsTotal:       methodsTotal,
			})

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
