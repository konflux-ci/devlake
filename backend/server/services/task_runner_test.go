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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/unithelper"
	mockconfig "github.com/apache/incubator-devlake/mocks/core/config"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
)

// Regression: when the task was missing from runningTasks, updateTaskProgress
// returned without draining the channel or signalling done, so the caller
// blocked forever on <-doneSignal.
func TestUpdateTaskProgress_MissingTaskStillDrainsAndSignalsDone(t *testing.T) {
	cfg := new(mockconfig.ConfigReader)
	cfg.On("GetBool", "SKIP_SUBTASK_PROGRESS").Return(false)
	res := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {
		mockDal.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		mockDal.On("UpdateColumn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	})
	res.On("GetConfigReader").Return(cfg)

	prevRes := basicRes
	basicRes = res
	t.Cleanup(func() { basicRes = prevRes })

	const missingTaskId = 999_999
	if getRunningTaskById(missingTaskId) != nil {
		t.Fatalf("task %d unexpectedly running", missingTaskId)
	}

	progress := make(chan plugin.RunningProgress, 10)
	done := make(chan struct{}, 2)
	progress <- plugin.RunningProgress{Type: plugin.SetCurrentSubTask, SubTaskName: "Extract", SubTaskNumber: 1}
	progress <- plugin.RunningProgress{Type: plugin.SubTaskIncProgress, Current: 1, Total: -1}
	progress <- plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: 1, Total: 1}
	close(progress)

	go updateTaskProgress(done, missingTaskId, progress)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("updateTaskProgress did not signal done")
	}
	if len(progress) != 0 {
		t.Errorf("progress channel not drained: %d events left", len(progress))
	}
	select {
	case <-done:
		t.Error("done signalled more than once")
	case <-time.After(50 * time.Millisecond):
	}
}
