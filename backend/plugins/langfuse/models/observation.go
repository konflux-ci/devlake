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

package models

import (
	"time"

	"github.com/apache/incubator-devlake/core/models/common"
)

type LangfuseObservation struct {
	common.NoPKModel    `mapstructure:",squash" gorm:"embedded"`
	ConnectionId        uint64     `json:"connectionId" gorm:"primaryKey"`
	ProjectId           string     `json:"projectId" gorm:"primaryKey;type:varchar(255)"`
	ObservationId       string     `json:"observationId" gorm:"primaryKey;type:varchar(255)"`
	TraceId             string     `json:"traceId" gorm:"type:varchar(255);index"`
	ParentObservationId string     `json:"parentObservationId" gorm:"type:varchar(255)"`
	Type                string     `json:"type" gorm:"type:varchar(50)"`
	Name                string     `json:"name" gorm:"type:varchar(255);index"`
	Model               string     `json:"model" gorm:"type:varchar(255)"`
	InputTokens         int        `json:"inputTokens"`
	OutputTokens        int        `json:"outputTokens"`
	TotalTokens         int        `json:"totalTokens"`
	LatencyMs           float64    `json:"latencyMs"`
	Level               string     `json:"level" gorm:"type:varchar(50)"`
	StatusMessage       string     `json:"statusMessage" gorm:"type:text"`
	CompletionStartTime *time.Time `json:"completionStartTime"`
	StartTime           *time.Time `json:"startTime"`
	EndTime             *time.Time `json:"endTime"`
	Metadata            string     `json:"metadata" gorm:"type:text"`
	Input               string     `json:"input" gorm:"type:text"`
	Output              string     `json:"output" gorm:"type:text"`
	ModelParameters     string     `json:"modelParameters" gorm:"type:text"`
	TotalCost           float64    `json:"totalCost"`
}

func (LangfuseObservation) TableName() string {
	return "_tool_langfuse_observations"
}
