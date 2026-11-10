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

package runner

import (
	"time"

	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
)

// SubtaskProgressInterval is the minimum time between two writes of a running
// subtask's finished_records to _devlake_subtasks. Subtasks report progress once
// per record, so writing every event costs one DB round trip per record.
const SubtaskProgressInterval = 2 * time.Second

// SubtaskRecords is a pending finished_records write for one subtask.
type SubtaskRecords struct {
	Name            string
	FinishedRecords int
}

// ProgressWrites holds the DB writes implied by one or more progress events.
// It is produced under the running-task lock and persisted outside of it.
type ProgressWrites struct {
	// TaskProgress is the new _devlake_tasks.progress value, if any.
	TaskProgress *float32
	// Subtasks are finished_records updates for _devlake_subtasks.
	Subtasks []SubtaskRecords
}

// Empty reports whether there is nothing to persist.
func (w ProgressWrites) Empty() bool {
	return w.TaskProgress == nil && len(w.Subtasks) == 0
}

// ProgressTracker applies RunningProgress events to a TaskProgressDetail and
// decides which DB writes they imply, throttling subtask record counts.
//
// A tracker belongs to a single task's progress goroutine and is not safe for
// concurrent use. Apply and Flush mutate the shared TaskProgressDetail, so the
// caller must hold whatever lock guards it; the returned writes should be
// persisted with SaveProgressWrites after releasing that lock.
type ProgressTracker struct {
	detail      *models.TaskProgressDetail
	skipSubtask bool
	interval    time.Duration
	now         func() time.Time

	lastWrite        time.Time
	persistedName    string
	persistedRecords int
}

// NewProgressTracker returns a tracker that updates detail in place.
// When skipSubtask is true, finished_records is never written to the DB.
func NewProgressTracker(detail *models.TaskProgressDetail, skipSubtask bool) *ProgressTracker {
	return &ProgressTracker{
		detail:      detail,
		skipSubtask: skipSubtask,
		interval:    SubtaskProgressInterval,
		now:         time.Now,
	}
}

// Apply records the event in the progress detail and returns the DB writes it
// requires, if any.
func (t *ProgressTracker) Apply(p *plugin.RunningProgress) ProgressWrites {
	var w ProgressWrites
	d := t.detail
	switch p.Type {
	case plugin.TaskSetProgress:
		d.TotalSubTasks = p.Total
		d.FinishedSubTasks = p.Current
	case plugin.TaskIncProgress:
		d.FinishedSubTasks = p.Current
		// a subtask just finished: persist its final record count
		t.appendPending(&w)
		if p.Total > 0 {
			pct := float32(p.Current) / float32(p.Total)
			w.TaskProgress = &pct
		}
	case plugin.SubTaskSetProgress:
		d.TotalRecords = p.Total
	case plugin.SubTaskIncProgress:
		d.FinishedRecords = p.Current
		if t.now().Sub(t.lastWrite) >= t.interval {
			t.appendPending(&w)
		}
	case plugin.SetCurrentSubTask:
		t.appendPending(&w)
		d.SubTaskName = p.SubTaskName
		d.SubTaskNumber = p.SubTaskNumber
		d.FinishedRecords = 0
		// let the first progress of the new subtask show up immediately
		t.lastWrite = time.Time{}
	}
	return w
}

// Flush returns the pending record count of the current subtask, if it has
// not been persisted yet. Call it once the progress channel is closed.
func (t *ProgressTracker) Flush() ProgressWrites {
	var w ProgressWrites
	t.appendPending(&w)
	return w
}

func (t *ProgressTracker) appendPending(w *ProgressWrites) {
	d := t.detail
	if t.skipSubtask || d.SubTaskName == "" {
		return
	}
	persisted := 0
	if t.persistedName == d.SubTaskName {
		persisted = t.persistedRecords
	}
	if d.FinishedRecords == persisted {
		return
	}
	w.Subtasks = append(w.Subtasks, SubtaskRecords{Name: d.SubTaskName, FinishedRecords: d.FinishedRecords})
	t.persistedName = d.SubTaskName
	t.persistedRecords = d.FinishedRecords
	t.lastWrite = t.now()
}

// SaveProgressWrites persists writes produced by a ProgressTracker. Failures
// are logged, not returned: progress is informational and must not fail a task.
func SaveProgressWrites(basicRes context.BasicRes, taskId uint64, w ProgressWrites) {
	if w.Empty() {
		return
	}
	db := basicRes.GetDal()
	if w.TaskProgress != nil {
		task := &models.Task{Model: common.Model{ID: taskId}}
		if err := db.UpdateColumn(task, "progress", *w.TaskProgress); err != nil {
			basicRes.GetLogger().Error(err, "failed to update progress")
		}
	}
	for _, s := range w.Subtasks {
		err := db.UpdateColumns(&models.Subtask{}, []dal.DalSet{
			{ColumnName: "finished_records", Value: s.FinishedRecords},
		}, dal.Where("task_id = ? and name = ?", taskId, s.Name))
		if err != nil {
			basicRes.GetLogger().Error(err, "failed to update _devlake_subtasks progress")
		}
	}
}
