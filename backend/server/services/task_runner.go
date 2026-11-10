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
	"context"
	"fmt"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/log"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/core/runner"
	"sync"
)

// RunningTaskData FIXME ...
type RunningTaskData struct {
	Cancel         context.CancelFunc
	ProgressDetail *models.TaskProgressDetail
}

// RunningTask FIXME ...
type RunningTask struct {
	mu    sync.Mutex
	tasks map[uint64]*RunningTaskData
}

// Add FIXME ...
func (rt *RunningTask) Add(taskId uint64, cancel context.CancelFunc) errors.Error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if _, ok := rt.tasks[taskId]; ok {
		return errors.Default.New(fmt.Sprintf("task with id %d already running", taskId))
	}
	rt.tasks[taskId] = &RunningTaskData{
		Cancel:         cancel,
		ProgressDetail: &models.TaskProgressDetail{},
	}
	return nil
}

// FillProgressDetailToTasks lock less times than GetProgressDetail
func (rt *RunningTask) FillProgressDetailToTasks(tasks []*models.Task) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	for index, task := range tasks {
		taskId := task.ID
		if task, ok := rt.tasks[taskId]; ok {
			tasks[index].ProgressDetail = task.ProgressDetail
		}
	}
}

// GetProgressDetail FIXME ...
func (rt *RunningTask) GetProgressDetail(taskId uint64) *models.TaskProgressDetail {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if task, ok := rt.tasks[taskId]; ok {
		return task.ProgressDetail
	}
	return nil
}

// Remove FIXME ...
func (rt *RunningTask) Remove(taskId uint64) (context.CancelFunc, errors.Error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if d, ok := rt.tasks[taskId]; ok {
		delete(rt.tasks, taskId)
		return d.Cancel, nil
	}
	return nil, errors.NotFound.New(fmt.Sprintf("task with id %d not found", taskId))
}

var runningTasks RunningTask

func init() {
	// set all previous unfinished tasks to status failed
	runningTasks.tasks = make(map[uint64]*RunningTaskData)
}

func runTaskStandalone(parentLog log.Logger, taskId uint64) errors.Error {
	// deferring cleaning up
	defer func() {
		_, _ = runningTasks.Remove(taskId)
	}()
	// for task cancelling
	ctx, cancel := context.WithCancel(context.Background())
	err := runningTasks.Add(taskId, cancel)
	if err != nil {
		return err
	}
	// now , create a progress update channel and kick off
	progress := make(chan plugin.RunningProgress, 100)
	doneSignal := make(chan struct{})
	go updateTaskProgress(doneSignal, taskId, progress)
	err = runner.RunTask(
		ctx,
		basicRes.ReplaceLogger(parentLog),
		progress,
		taskId,
	)
	close(progress)
	// wait all progresses are handled
	<-doneSignal
	return err
}

func getRunningTaskById(taskId uint64) *RunningTaskData {
	runningTasks.mu.Lock()
	defer runningTasks.mu.Unlock()

	return runningTasks.tasks[taskId]
}

// updateTaskProgress drains the task's progress channel until it is closed,
// then signals done. runningTasks.mu is shared by every running task, so it is
// held only for the in-memory update; DB writes happen outside of it and are
// throttled by runner.ProgressTracker. Holding the lock across a DB round trip
// per record serialized all concurrent extractors.
func updateTaskProgress(done chan struct{}, taskId uint64, progress chan plugin.RunningProgress) {
	defer func() { done <- struct{}{} }()
	progressDetail := &models.TaskProgressDetail{}
	if data := getRunningTaskById(taskId); data != nil {
		progressDetail = data.ProgressDetail
	}
	// SKIP_SUBTASK_PROGRESS is read once per task: it is an environment variable
	// and cannot change while the process runs.
	tracker := runner.NewProgressTracker(progressDetail, basicRes.GetConfigReader().GetBool("SKIP_SUBTASK_PROGRESS"))
	for p := range progress {
		runningTasks.mu.Lock()
		writes := tracker.Apply(&p)
		runningTasks.mu.Unlock()
		saveTaskProgress(tracker, taskId, writes)
	}
	runningTasks.mu.Lock()
	writes := tracker.Flush()
	runningTasks.mu.Unlock()
	saveTaskProgress(tracker, taskId, writes)
}

// saveTaskProgress persists writes outside runningTasks.mu and hands failed
// subtask writes back to the tracker for retry. Progress is informational, so
// failures are logged and never fail the task.
func saveTaskProgress(tracker *runner.ProgressTracker, taskId uint64, writes runner.ProgressWrites) {
	failed, err := runner.SaveProgressWrites(basicRes, taskId, writes)
	if err == nil {
		return
	}
	basicRes.GetLogger().Error(errors.Convert(err), "failed to save progress of task %d", taskId)
	if len(failed) > 0 {
		runningTasks.mu.Lock()
		tracker.Unsaved(failed)
		runningTasks.mu.Unlock()
	}
}
