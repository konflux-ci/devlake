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
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
	"github.com/apache/incubator-devlake/plugins/codecov/models"
)

var ConvertCoverageTrendMeta = plugin.SubTaskMeta{
	Name:             "ConvertCoverageTrend",
	EntryPoint:       ConvertCoverageTrend,
	EnabledByDefault: true,
	Description:      "Convert coverage trend data per flag from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CODE},
	DependencyTables: []string{RAW_FLAG_COVERAGE_TRENDS_TABLE},
}

func ConvertCoverageTrend(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*CodecovTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: CodecovApiParams{
				ConnectionId: data.Options.ConnectionId,
				Name:         data.Options.FullName,
			},
			Table: RAW_FLAG_COVERAGE_TRENDS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var trend struct {
				FlagName string `json:"flag_name"`
				Trend    []struct {
					Date     string  `json:"date"`
					Coverage float64 `json:"coverage"`
					Lines    int     `json:"lines"`
					Hits     int     `json:"hits"`
					Methods  int     `json:"methods"`
				} `json:"trend"`
			}
			err := errors.Convert(json.Unmarshal(resData.Data, &trend))
			if err != nil {
				return nil, err
			}

			var results []interface{}

			// Extract branch from scope config or default to main
			branch := "main"
			if data.Options.ScopeConfig != nil {
				// Could add branch config here if needed
			}

			// Convert each trend point
			for _, point := range trend.Trend {
				date, err := time.Parse("2006-01-02", point.Date)
				if err != nil {
					continue // Skip invalid dates
				}

				codecovTrend := &models.CodecovCoverageTrend{
					Model:              common.Model{},
					ConnectionId:       data.Options.ConnectionId,
					RepoId:             data.Options.FullName,
					FlagName:           trend.FlagName,
					Branch:             branch,
					Date:               date,
					CoveragePercentage: point.Coverage,
					LinesCovered:       point.Hits,
					LinesTotal:         point.Lines,
					MethodsCovered:     point.Methods, // Approximate
					MethodsTotal:       point.Methods, // Approximate
				}

				results = append(results, codecovTrend)
			}

			return results, nil
		},
	})

	if err != nil {
		return err
	}

	return extractor.Execute()
}
