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

package impl

import (
	"encoding/json"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	coreModels "github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/plugins/dedup/models"
	"github.com/apache/incubator-devlake/plugins/dedup/models/migrationscripts"
	"github.com/apache/incubator-devlake/plugins/dedup/tasks"
)

// Verify interface implementation at compile time.
var _ interface {
	plugin.PluginMeta
	plugin.PluginInit
	plugin.PluginTask
	plugin.PluginModel
	plugin.PluginMetric
	plugin.PluginMigration
	plugin.MetricPluginBlueprintV200
} = (*Dedup)(nil)

// Dedup is the main plugin struct.
type Dedup struct{}

func (p Dedup) Init(_ context.BasicRes) errors.Error {
	return nil
}

func (p Dedup) Name() string {
	return "dedup"
}

func (p Dedup) Description() string {
	return "Populate deduplicating views for repos, pull requests, issues, and repo_commits to eliminate duplicate records caused by the same repository being collected via multiple connections"
}

func (p Dedup) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/dedup"
}

// IsProjectMetric returns true so the framework runs this plugin as a metric
// after source plugins complete.
func (p Dedup) IsProjectMetric() bool {
	return true
}

// RunAfter declares that this plugin must run after GitHub (and GitLab) have
// finished collecting data.
func (p Dedup) RunAfter() ([]string, errors.Error) {
	return []string{"github", "gitlab"}, nil
}

func (p Dedup) Settings() interface{} {
	return nil
}

func (p Dedup) RequiredDataEntities() ([]map[string]interface{}, errors.Error) {
	return []map[string]interface{}{
		{
			"model": "repos",
			"requiredFields": map[string]string{
				"id":  "string",
				"url": "string",
			},
		},
	}, nil
}

func (p Dedup) GetTablesInfo() []dal.Tabler {
	return []dal.Tabler{
		&models.DedupCanonicalScope{},
	}
}

func (p Dedup) SubTaskMetas() []plugin.SubTaskMeta {
	return []plugin.SubTaskMeta{
		tasks.CollectCanonicalScopesMeta,
	}
}

func (p Dedup) PrepareTaskData(taskCtx plugin.TaskContext, options map[string]interface{}) (interface{}, errors.Error) {
	op, err := tasks.DecodeTaskOptions(options)
	if err != nil {
		return nil, err
	}
	return &tasks.DedupTaskData{Options: op}, nil
}

func (p Dedup) MigrationScripts() []plugin.MigrationScript {
	return migrationscripts.All()
}

// MakeMetricPluginPipelinePlanV200 builds the pipeline plan for a project run.
func (p Dedup) MakeMetricPluginPipelinePlanV200(projectName string, options json.RawMessage) (coreModels.PipelinePlan, errors.Error) {
	opts := map[string]interface{}{
		"projectName": projectName,
	}
	plan := coreModels.PipelinePlan{
		{
			{
				Plugin:  "dedup",
				Options: opts,
				Subtasks: []string{
					tasks.CollectCanonicalScopesMeta.Name,
				},
			},
		},
	}
	return plan, nil
}
