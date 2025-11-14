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

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// ComparisonData stores comparison results for linking to commit coverage per flag
type ComparisonData struct {
	common.Model
	common.RawDataOrigin `mapstructure:",squash"`
	ConnectionId     uint64  `gorm:"primaryKey;type:bigint"`
	RepoId           string  `gorm:"primaryKey;type:varchar(200);index"`
	CommitSha        string  `gorm:"primaryKey;type:varchar(64);index"`
	FlagName         string  `gorm:"primaryKey;type:varchar(100);index"`
	ParentSha        string  `gorm:"type:varchar(64)"`
	ModifiedCoverage float64 `gorm:"type:double"`
	FilesChanged     int     `gorm:"type:int"`
	MethodsCovered   int     `gorm:"type:int"`
	MethodsTotal     int     `gorm:"type:int"`
	LinesCovered     int     `gorm:"type:int"` // Lines covered in modified code
	LinesTotal       int     `gorm:"type:int"` // Total lines in modified code
	LinesMissed      int     `gorm:"type:int"` // Lines missed in modified code
}

func (ComparisonData) TableName() string {
	return "_tool_codecov_comparisons"
}

var ConvertComparisonMeta = plugin.SubTaskMeta{
	Name:             "ConvertComparison",
	EntryPoint:       ConvertComparison,
	EnabledByDefault: true,
	Description:      "Convert comparison data (modified/patch coverage) from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	Dependencies:     []*plugin.SubTaskMeta{&ExtractCommitsMeta},
	DependencyTables: []string{RAW_COMPARISONS_TABLE},
}

func ConvertComparison(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_COMPARISONS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			// Read input to get commit and parent SHA
			var input ComparisonInput
			err := errors.Convert(json.Unmarshal(resData.Input, &input))
			if err != nil {
				return nil, err
			}

			// Parse comparison response
			var comparison struct {
				BaseCommitid string `json:"base_commitid"`
				HeadCommitid string `json:"head_commitid"`
				Diff         struct {
					Files []struct {
						Name string `json:"name"`
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
				} `json:"diff"`
			}
			err = errors.Convert(json.Unmarshal(resData.Data, &comparison))
			if err != nil {
				return nil, err
			}

			// Store comparison data for later use in coverage conversion (per flag)
			comparisonData := &ComparisonData{
				Model:            common.Model{},
				ConnectionId:     data.Options.ConnectionId,
				RepoId:           data.Options.FullName,
				CommitSha:        input.CommitSha,
				FlagName:         input.FlagName,
				ParentSha:        input.ParentSha,
				ModifiedCoverage: comparison.Diff.Totals.Coverage,
				FilesChanged:     len(comparison.Diff.Files),
				MethodsCovered:   comparison.Diff.Totals.Methods, // This might need adjustment based on API
				MethodsTotal:     comparison.Diff.Totals.Methods, // This might need adjustment based on API
				LinesCovered:     comparison.Diff.Totals.Hits,   // Lines covered in modified code
				LinesTotal:       comparison.Diff.Totals.Lines,    // Total lines in modified code
				LinesMissed:      comparison.Diff.Totals.Misses,  // Lines missed in modified code
			}

			return []interface{}{comparisonData}, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}

