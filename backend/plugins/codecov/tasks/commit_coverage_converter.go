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

var ConvertCommitCoverageMeta = plugin.SubTaskMeta{
	Name:             "ConvertCommitCoverage",
	EntryPoint:       ConvertCommitCoverage,
	EnabledByDefault: true,
	Description:      "Convert overall and modified coverage per commit from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ConvertComparisonMeta},
	DependencyTables: []string{RAW_COMMIT_TOTALS_TABLE, RAW_COMPARISONS_TABLE},
}

func ConvertCommitCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)
	db := taskCtx.GetDal()

	// Extract overall coverage from commit totals and join with comparison data
	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMMIT_TOTALS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get commit SHA (more reliable than API response)
			var input CommitInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

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
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &totals))
			if err != nil {
				return nil, err
			}

			// Use commit SHA from input (more reliable than API response which may be empty)
			commitSha := input.CommitSha
			if commitSha == "" && totals.Commitid != "" {
				commitSha = totals.Commitid
			}
			if commitSha == "" {
				// No commit SHA available, skip this record
				return nil, nil
			}

			// Get commit info
			var commit models.CodecovCommit
			err = db.First(&commit, dal.Where("connection_id = ? AND repo_id = ? AND commit_sha = ?", data.Options.ConnectionId, data.Options.FullName, commitSha))
			if err != nil {
				return nil, nil // Skip if commit not found
			}

			// Get modified coverage from comparison data if available
			var modifiedCoverage float64
			var filesChanged int
			var methodsCovered, methodsTotal int

			// Try to find comparison data for this commit (overall, flag_name = "")
			var comparison ComparisonData
			err = db.First(&comparison, dal.Where("connection_id = ? AND repo_id = ? AND commit_sha = ? AND flag_name = ?", data.Options.ConnectionId, data.Options.FullName, commitSha, ""))
			if err == nil {
				// Found comparison data
				modifiedCoverage = comparison.ModifiedCoverage
				filesChanged = comparison.FilesChanged
				methodsCovered = comparison.MethodsCovered
				methodsTotal = comparison.MethodsTotal
			} else {
				// No comparison data, use overall totals for methods
				// Modified coverage stays 0
				methodsCovered = totals.Totals.Methods
				methodsTotal = totals.Totals.Methods
			}

			codecovCommitCoverage := &models.CodecovCommitCoverage{
				NoPKModel:        common.NoPKModel{},
				ConnectionId:     data.Options.ConnectionId,
				RepoId:           data.Options.FullName,
				CommitSha:        commitSha,
				Branch:           commit.Branch,
				CommitTimestamp:  commit.CommitTimestamp,
				OverallCoverage:  totals.Totals.Coverage,
				ModifiedCoverage: modifiedCoverage,
				FilesChanged:     filesChanged,
				MethodsCovered:   methodsCovered,
				MethodsTotal:     methodsTotal,
			}

			return []interface{}{codecovCommitCoverage}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
