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
	goerrors "errors"
	"fmt"
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
// concurrent use. Apply, Flush and Unsaved read or mutate the shared
// TaskProgressDetail, so the caller must hold whatever lock guards it; the
// returned writes should be persisted with SaveProgressWrites after releasing
// that lock.
type ProgressTracker struct {
	detail      *models.TaskProgressDetail
	skipSubtask bool
	interval    time.Duration
	now         func() time.Time

	lastWrite        time.Time
	persistedName    string
	persistedRecords int
	// retry holds failed final counts of subtasks that are no longer current.
	retry []SubtaskRecords
}

// NewProgressTracker returns a tracker that updates detail in place.
// When skipSubtask is true, finished_records is never written to the DB.
// skipSubtask is fixed for the tracker's lifetime; it comes from an
// environment variable, which cannot change while the process runs.
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
func (pt *ProgressTracker) Apply(p *plugin.RunningProgress) ProgressWrites {
	var w ProgressWrites
	d := pt.detail
	switch p.Type {
	case plugin.TaskSetProgress:
		d.TotalSubTasks = p.Total
		d.FinishedSubTasks = p.Current
	case plugin.TaskIncProgress:
		d.FinishedSubTasks = p.Current
		// a subtask just finished: persist its final record count
		pt.appendPending(&w)
		if p.Total > 0 {
			pct := float32(p.Current) / float32(p.Total)
			w.TaskProgress = &pct
		}
	case plugin.SubTaskSetProgress:
		d.TotalRecords = p.Total
	case plugin.SubTaskIncProgress:
		d.FinishedRecords = p.Current
		if pt.now().Sub(pt.lastWrite) >= pt.interval {
			pt.appendPending(&w)
		}
	case plugin.SetCurrentSubTask:
		pt.appendPending(&w)
		d.SubTaskName = p.SubTaskName
		d.SubTaskNumber = p.SubTaskNumber
		d.FinishedRecords = 0
		// let the first progress of the new subtask show up immediately
		pt.lastWrite = time.Time{}
	}
	return w
}

// Flush returns the pending record count of the current subtask, if it has
// not been persisted yet, plus any failed writes still to retry. Call it once
// the progress channel is closed.
func (pt *ProgressTracker) Flush() ProgressWrites {
	var w ProgressWrites
	pt.appendPending(&w)
	return w
}

// Unsaved reports subtask writes that SaveProgressWrites failed to persist,
// so the next write or Flush retries them instead of assuming they were saved.
func (pt *ProgressTracker) Unsaved(failed []SubtaskRecords) {
	for _, s := range failed {
		if s.Name == pt.detail.SubTaskName {
			if pt.persistedName == s.Name && pt.persistedRecords == s.FinishedRecords {
				pt.persistedName = ""
				pt.persistedRecords = 0
			}
			continue
		}
		pt.retry = append(pt.retry, s)
	}
}

func (pt *ProgressTracker) appendPending(w *ProgressWrites) {
	if len(pt.retry) > 0 {
		w.Subtasks = append(w.Subtasks, pt.retry...)
		pt.retry = nil
		pt.lastWrite = pt.now()
	}
	d := pt.detail
	if pt.skipSubtask || d.SubTaskName == "" {
		return
	}
	persisted := 0
	if pt.persistedName == d.SubTaskName {
		persisted = pt.persistedRecords
	}
	if d.FinishedRecords == persisted {
		return
	}
	w.Subtasks = append(w.Subtasks, SubtaskRecords{Name: d.SubTaskName, FinishedRecords: d.FinishedRecords})
	pt.persistedName = d.SubTaskName
	pt.persistedRecords = d.FinishedRecords
	pt.lastWrite = pt.now()
}

// SaveProgressWrites persists writes produced by a ProgressTracker. It attempts
// every write and returns the subtask writes that failed, so the tracker can
// retry them, together with all failures joined into one error.
func SaveProgressWrites(basicRes context.BasicRes, taskId uint64, w ProgressWrites) ([]SubtaskRecords, error) {
	if w.Empty() {
		return nil, nil
	}
	db := basicRes.GetDal()
	var errs []error
	if w.TaskProgress != nil {
		task := &models.Task{Model: common.Model{ID: taskId}}
		if err := db.UpdateColumn(task, "progress", *w.TaskProgress); err != nil {
			errs = append(errs, fmt.Errorf("updating task %d progress: %w", taskId, err))
		}
	}
	var failed []SubtaskRecords
	for _, s := range w.Subtasks {
		err := db.UpdateColumns(&models.Subtask{}, []dal.DalSet{
			{ColumnName: "finished_records", Value: s.FinishedRecords},
		}, dal.Where("task_id = ? and name = ?", taskId, s.Name))
		if err != nil {
			failed = append(failed, s)
			errs = append(errs, fmt.Errorf("updating task %d subtask %q finished_records: %w", taskId, s.Name, err))
		}
	}
	return failed, goerrors.Join(errs...)
}
