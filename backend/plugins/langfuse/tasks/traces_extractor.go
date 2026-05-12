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

var ExtractTracesMeta = plugin.SubTaskMeta{
	Name:             "ExtractTraces",
	EntryPoint:       ExtractTraces,
	EnabledByDefault: true,
	Description:      "Extract traces from raw data",
	DomainTypes:      []string{plugin.DOMAIN_TYPE_CROSS},
	DependencyTables: []string{RAW_TRACES_TABLE},
}

func ExtractTraces(taskCtx plugin.SubTaskContext) errors.Error {
	data := taskCtx.GetData().(*LangfuseTaskData)

	extractor, err := helper.NewApiExtractor(helper.ApiExtractorArgs{
		RawDataSubTaskArgs: helper.RawDataSubTaskArgs{
			Ctx: taskCtx,
			Params: LangfuseApiParams{
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
			},
			Table: RAW_TRACES_TABLE,
		},
		Extract: func(resData *helper.RawData) ([]interface{}, errors.Error) {
			var raw struct {
				Id        string          `json:"id"`
				Name      string          `json:"name"`
				SessionId string          `json:"sessionId"`
				UserId    string          `json:"userId"`
				Timestamp string          `json:"timestamp"`
				Release   string          `json:"release"`
				Version   string          `json:"version"`
				Tags      []string        `json:"tags"`
				Metadata  json.RawMessage `json:"metadata"`
				Latency   float64         `json:"latency"`
				Usage     *struct {
					Input  int `json:"input"`
					Output int `json:"output"`
					Total  int `json:"total"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(resData.Data, &raw); err != nil {
				return nil, errors.Default.Wrap(err, "failed to unmarshal trace")
			}

			var ts *time.Time
			if raw.Timestamp != "" {
				if parsed, e := time.Parse(time.RFC3339Nano, raw.Timestamp); e == nil {
					ts = &parsed
				}
			}

			tagsJSON, _ := json.Marshal(raw.Tags)

			trace := &models.LangfuseTrace{
				NoPKModel:    common.NoPKModel{},
				ConnectionId: data.Options.ConnectionId,
				ProjectId:    data.Options.ProjectId,
				TraceId:      raw.Id,
				Name:         raw.Name,
				SessionId:    raw.SessionId,
				UserId:       raw.UserId,
				Timestamp:    ts,
				LatencyMs:    raw.Latency * 1000,
				Tags:         string(tagsJSON),
				Metadata:     string(raw.Metadata),
				Release:      raw.Release,
				Version:      raw.Version,
			}

			if raw.Usage != nil {
				trace.InputTokens = raw.Usage.Input
				trace.OutputTokens = raw.Usage.Output
				trace.TotalTokens = raw.Usage.Total
			}

			return []interface{}{trace}, nil
		},
	})
	if err != nil {
		return err
	}
	return extractor.Execute()
}
