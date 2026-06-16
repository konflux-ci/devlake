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
	"github.com/apache/incubator-devlake/core/errors"
	helper "github.com/apache/incubator-devlake/helpers/pluginhelper/api"
)

// DedupOptions contains configuration options for the dedup plugin.
type DedupOptions struct {
	// ProjectName is passed by the metric plugin framework; the canonical-scope
	// computation is global, so this field is stored but not used in the subtask.
	ProjectName string `json:"projectName"`
}

// DedupTaskData is the shared task context passed to all subtasks.
type DedupTaskData struct {
	Options *DedupOptions
}

// DecodeTaskOptions decodes the raw options map into DedupOptions.
func DecodeTaskOptions(options map[string]interface{}) (*DedupOptions, errors.Error) {
	var op DedupOptions
	if err := helper.Decode(options, &op, nil); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to decode dedup options")
	}
	return &op, nil
}
