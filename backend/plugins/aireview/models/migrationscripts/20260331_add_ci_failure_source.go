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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addCiFailureSource)(nil)

type addCiFailureSource struct{}

func (script *addCiFailureSource) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if err := db.AutoMigrate(&scopeConfigAddCiFailureSource20260331{}); err != nil {
		return errors.Default.Wrap(err, "failed to add ci_failure_source to _tool_aireview_scope_configs")
	}
	if err := db.AutoMigrate(&failurePredictionAddCiFailureSource20260331{}); err != nil {
		return errors.Default.Wrap(err, "failed to add ci_failure_source to _tool_aireview_failure_predictions")
	}
	if err := db.AutoMigrate(&predictionMetricsAddCiFailureSource20260331{}); err != nil {
		return errors.Default.Wrap(err, "failed to add ci_failure_source to _tool_aireview_prediction_metrics")
	}

	return nil
}

func (script *addCiFailureSource) Version() uint64 {
	return 20260331000001
}

func (script *addCiFailureSource) Name() string {
	return "aireview add ci_failure_source to scope config, predictions, and metrics"
}

type scopeConfigAddCiFailureSource20260331 struct {
	CiFailureSource string `gorm:"type:varchar(20);default:'both'"`
}

func (scopeConfigAddCiFailureSource20260331) TableName() string {
	return "_tool_aireview_scope_configs"
}

type failurePredictionAddCiFailureSource20260331 struct {
	CiFailureSource string `gorm:"type:varchar(20);index"`
}

func (failurePredictionAddCiFailureSource20260331) TableName() string {
	return "_tool_aireview_failure_predictions"
}

type predictionMetricsAddCiFailureSource20260331 struct {
	CiFailureSource string `gorm:"type:varchar(20);index"`
}

func (predictionMetricsAddCiFailureSource20260331) TableName() string {
	return "_tool_aireview_prediction_metrics"
}
