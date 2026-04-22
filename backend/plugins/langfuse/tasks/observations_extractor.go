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
	"github.com/apache/incubator-devlake/plugins/langfuse/models"
)

var ExtractObservationsMeta = plugin.SubTaskMeta{
	Name:             "ExtractObservations",
	EntryPoint:       ExtractObservations,
	EnabledByDefault: true,
	Description:      "Extract observations from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_OBSERVATIONS_TABLE},
}

func ExtractObservations(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LangfuseTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LangfuseApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_OBSERVATIONS_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				Id                  string          `json:"id"`
				TraceId             string          `json:"traceId"`
				ParentObservationId string          `json:"parentObservationId"`
				Type                string          `json:"type"`
				Name                string          `json:"name"`
				Model               string          `json:"model"`
				StartTime           string          `json:"startTime"`
				EndTime             string          `json:"endTime"`
				CompletionStartTime string          `json:"completionStartTime"`
				Level               string          `json:"level"`
				StatusMessage       string          `json:"statusMessage"`
				Metadata            json.RawMessage `json:"metadata"`
				Input               json.RawMessage `json:"input"`
				Output              json.RawMessage `json:"output"`
				ModelParameters     json.RawMessage `json:"modelParameters"`
				Usage *struct {
					Input  int `json:"input"`
					Output int `json:"output"`
					Total  int `json:"total"`
				} `json:"usage"`
				UsageDetails *struct {
					Input  int `json:"input"`
					Output int `json:"output"`
					Total  int `json:"total"`
				} `json:"usageDetails"`
				CalculatedTotalCost float64 `json:"calculatedTotalCost"`
			}
			if err := json.Unmarshal(resData.Data, &raw); err != nil {
				return nil, errors.Default.Wrap(err, "failed to unmarshal observation")
			}

			parseTime := func(s string) *time.Time {
				if s == "" {
					return nil
				}
				if t, e := time.Parse(time.RFC3339Nano, s); e == nil {
					return &t
				}
				return nil
			}

			var latencyMs float64
			st := parseTime(raw.StartTime)
			et := parseTime(raw.EndTime)
			if st != nil && et != nil {
				latencyMs = float64(et.Sub(*st).Milliseconds())
			}

			truncate := func(b json.RawMessage, max int) string {
				s := string(b)
				if len(s) > max {
					return s[:max] + "..."
				}
				return s
			}

			obs := &models.LangfuseObservation{
				NoPKModel:           common.NoPKModel{},
				ConnectionId:        data.Options.ConnectionId,
				ProjectId:           data.Options.ProjectId,
				ObservationId:       raw.Id,
				TraceId:             raw.TraceId,
				ParentObservationId: raw.ParentObservationId,
				Type:                raw.Type,
				Name:                raw.Name,
				Model:               raw.Model,
				LatencyMs:           latencyMs,
				Level:               raw.Level,
				StatusMessage:       raw.StatusMessage,
				StartTime:           st,
				EndTime:             et,
				CompletionStartTime: parseTime(raw.CompletionStartTime),
				Metadata:            string(raw.Metadata),
				Input:               truncate(raw.Input, 1000),
				Output:              truncate(raw.Output, 1000),
				ModelParameters:     string(raw.ModelParameters),
				TotalCost:           raw.CalculatedTotalCost,
			}

			if raw.Usage != nil {
				obs.InputTokens = raw.Usage.Input
				obs.OutputTokens = raw.Usage.Output
				obs.TotalTokens = raw.Usage.Total
			} else if raw.UsageDetails != nil {
				obs.InputTokens = raw.UsageDetails.Input
				obs.OutputTokens = raw.UsageDetails.Output
				obs.TotalTokens = raw.UsageDetails.Total
			}

			return []interface{}{obs}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
