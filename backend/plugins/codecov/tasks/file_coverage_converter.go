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
	codecovModels "github.com/apache/incubator-devlake/plugins/codecov/models"
)

var ConvertFileCoverageMeta = plugin.SubTaskMeta{
	Name:             "ConvertFileCoverage",
	EntryPoint:       ConvertFileCoverage,
	EnabledByDefault: true,
	Description:      "Convert file-level coverage data from report API",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractCommitsMeta},
	DependencyTables: []string{RAW_FILE_COVERAGES_TABLE},
}

func ConvertFileCoverage(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FILE_COVERAGES_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get commit, flag, and branch
			var input FileCoverageInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

			// Parse report response to get file-level data
			// Codecov report API returns files as an array with coverage data
			var report struct {
				Commitid string `json:"commitid"`
				Files    []struct {
					Name   string `json:"name"`
					Totals struct {
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
				} `json:"files"`
				Totals struct {
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
			err = errors.Convert(json.Unmarshal(resData.Data, &report))
			if err != nil {
				return nil, err
			}

			// Get commit info to extract timestamp
			var commit codecovModels.CodecovCommit
			db := taskCtx.GetDal()
			err = db.First(&commit, dal.Where("connection_id = ? AND repo_id = ? AND commit_sha = ?", data.Options.ConnectionId, data.Options.FullName, input.CommitSha))
			if err != nil {
				// If commit not found, skip this record
				return nil, nil
			}

			var results []interface{}

			// Extract file-level coverage data from report
			for _, file := range report.Files {
				// Only include files that have coverage data
				if file.Totals.Lines > 0 {
					// Truncate file path if too long (max 150 chars for primary key)
					truncatedPath := file.Name
					if len(truncatedPath) > 150 {
						truncatedPath = truncatedPath[:150]
					}

					fileCoverage := &codecovModels.CodecovFileCoverage{
						Model:              common.Model{},
						ConnectionId:       data.Options.ConnectionId,
						RepoId:             data.Options.FullName,
						FlagName:           input.FlagName,
						Branch:             input.Branch,
						CommitSha:          input.CommitSha,
						FilePath:           truncatedPath,
						CommitTimestamp:    commit.CommitTimestamp,
						CoveragePercentage: file.Totals.Coverage,
						LinesCovered:       file.Totals.Hits,
						LinesTotal:         file.Totals.Lines,
						LinesMissed:        file.Totals.Misses,
						Hits:               file.Totals.Hits,
						Misses:             file.Totals.Misses,
						Partials:           file.Totals.Partials,
					}
					results = append(results, fileCoverage)
				}
			}

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
