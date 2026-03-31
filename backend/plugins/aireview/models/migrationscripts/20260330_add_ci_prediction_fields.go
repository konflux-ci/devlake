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

var _ plugin.MigrationScript = (*addCiPredictionFields)(nil)

type addCiPredictionFields struct{}

func (script *addCiPredictionFields) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	if err := db.AutoMigrate(&scopeConfigAddWarningThreshold20260330{}); err != nil {
		return errors.Default.Wrap(err, "failed to add warning_threshold to _tool_aireview_scope_configs")
	}
	if err := db.AutoMigrate(&failurePredictionAddCiFields20260330{}); err != nil {
		return errors.Default.Wrap(err, "failed to add ci fields to _tool_aireview_failure_predictions")
	}
	if err := db.AutoMigrate(&predictionMetricsAddAucFields20260330{}); err != nil {
		return errors.Default.Wrap(err, "failed to add auc fields to _tool_aireview_prediction_metrics")
	}

	return nil
}

func (script *addCiPredictionFields) Version() uint64 {
	return 20260330000001
}

func (script *addCiPredictionFields) Name() string {
	return "aireview add CI prediction fields and AUC metrics"
}

type scopeConfigAddWarningThreshold20260330 struct {
	WarningThreshold int `gorm:"default:50"`
}

func (scopeConfigAddWarningThreshold20260330) TableName() string {
	return "_tool_aireview_scope_configs"
}

type failurePredictionAddCiFields20260330 struct {
	PullRequestKey string `gorm:"type:varchar(255);index"`
	RepoShortName  string `gorm:"type:varchar(255)"`
}

func (failurePredictionAddCiFields20260330) TableName() string {
	return "_tool_aireview_failure_predictions"
}

type predictionMetricsAddAucFields20260330 struct {
	Specificity      float64
	FprPct           float64
	PrAuc            float64
	RocAuc           float64
	WarningThreshold int `gorm:"default:50"`
}

func (predictionMetricsAddAucFields20260330) TableName() string {
	return "_tool_aireview_prediction_metrics"
}
