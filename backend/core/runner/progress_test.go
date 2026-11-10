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
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/unithelper"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestTracker(skip bool) (*ProgressTracker, *models.TaskProgressDetail, *fakeClock) {
	detail := &models.TaskProgressDetail{}
	clock := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	tr := NewProgressTracker(detail, skip)
	tr.now = clock.now
	return tr, detail, clock
}

func subtaskWrites(ws ...ProgressWrites) []SubtaskRecords {
	var out []SubtaskRecords
	for _, w := range ws {
		out = append(out, w.Subtasks...)
	}
	return out
}

func setSubtask(name string) *plugin.RunningProgress {
	return &plugin.RunningProgress{Type: plugin.SetCurrentSubTask, SubTaskName: name, SubTaskNumber: 1}
}

func incRecords(n int) *plugin.RunningProgress {
	return &plugin.RunningProgress{Type: plugin.SubTaskIncProgress, Current: n, Total: -1}
}

// Regression: extractors report total -1 and one event per record. Each event
// used to trigger an UPDATE of _devlake_subtasks under a lock shared by every
// running task, throttling all extractors to roughly one row per second.
func TestProgressTracker_ThrottlesPerRecordWrites(t *testing.T) {
	tr, detail, clock := newTestTracker(false)
	var ws []ProgressWrites
	ws = append(ws, tr.Apply(setSubtask("Extract Users")))
	ws = append(ws, tr.Apply(&plugin.RunningProgress{Type: plugin.SubTaskSetProgress, Current: 0, Total: -1}))
	for i := 1; i <= 10_000; i++ {
		ws = append(ws, tr.Apply(incRecords(i)))
	}
	got := subtaskWrites(ws...)
	want := []SubtaskRecords{{Name: "Extract Users", FinishedRecords: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("writes within one interval = %v, want %v", got, want)
	}
	if detail.FinishedRecords != 10_000 {
		t.Errorf("in-memory FinishedRecords = %d, want 10000", detail.FinishedRecords)
	}

	clock.advance(SubtaskProgressInterval)
	got = subtaskWrites(tr.Apply(incRecords(10_001)))
	want = []SubtaskRecords{{Name: "Extract Users", FinishedRecords: 10_001}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("writes after interval = %v, want %v", got, want)
	}
}

func TestProgressTracker_PersistsFinalCount(t *testing.T) {
	tests := []struct {
		name   string
		finish func(tr *ProgressTracker) ProgressWrites
		want   []SubtaskRecords
	}{
		{
			name: "task inc progress after subtask ends",
			finish: func(tr *ProgressTracker) ProgressWrites {
				return tr.Apply(&plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: 1, Total: 3})
			},
			want: []SubtaskRecords{{Name: "A", FinishedRecords: 50}},
		},
		{
			name: "switch to next subtask",
			finish: func(tr *ProgressTracker) ProgressWrites {
				return tr.Apply(setSubtask("B"))
			},
			want: []SubtaskRecords{{Name: "A", FinishedRecords: 50}},
		},
		{
			name:   "flush on channel close (failed subtask)",
			finish: func(tr *ProgressTracker) ProgressWrites { return tr.Flush() },
			want:   []SubtaskRecords{{Name: "A", FinishedRecords: 50}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, _, _ := newTestTracker(false)
			tr.Apply(setSubtask("A"))
			for i := 1; i <= 50; i++ {
				tr.Apply(incRecords(i))
			}
			got := subtaskWrites(tt.finish(tr))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("final writes = %v, want %v", got, tt.want)
			}
			if again := subtaskWrites(tr.Flush()); len(again) != 0 {
				t.Errorf("second flush wrote %v, want nothing", again)
			}
		})
	}
}

func TestProgressTracker_NewSubtaskWritesImmediately(t *testing.T) {
	tr, detail, _ := newTestTracker(false)
	tr.Apply(setSubtask("A"))
	tr.Apply(incRecords(1))
	tr.Apply(incRecords(2))

	got := subtaskWrites(tr.Apply(setSubtask("B")), tr.Apply(incRecords(1)))
	want := []SubtaskRecords{{Name: "A", FinishedRecords: 2}, {Name: "B", FinishedRecords: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("writes = %v, want %v", got, want)
	}
	if detail.SubTaskName != "B" || detail.FinishedRecords != 1 {
		t.Errorf("detail = %+v, want subtask B with 1 record", detail)
	}
}

func TestProgressTracker_SkipSubtaskProgress(t *testing.T) {
	tr, detail, clock := newTestTracker(true)
	var ws []ProgressWrites
	ws = append(ws, tr.Apply(setSubtask("A")))
	for i := 1; i <= 5; i++ {
		clock.advance(SubtaskProgressInterval)
		ws = append(ws, tr.Apply(incRecords(i)))
	}
	ws = append(ws, tr.Apply(&plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: 1, Total: 2}))
	ws = append(ws, tr.Flush())

	if got := subtaskWrites(ws...); len(got) != 0 {
		t.Errorf("subtask writes = %v, want none", got)
	}
	if detail.FinishedRecords != 5 {
		t.Errorf("in-memory FinishedRecords = %d, want 5", detail.FinishedRecords)
	}
	last := ws[len(ws)-2]
	if last.TaskProgress == nil || *last.TaskProgress != 0.5 {
		t.Errorf("task progress = %v, want 0.5", last.TaskProgress)
	}
}

func TestProgressTracker_TaskProgress(t *testing.T) {
	tests := []struct {
		name    string
		current int
		total   int
		want    *float32
	}{
		{"half done", 2, 4, ptr(float32(0.5))},
		{"unknown total writes nothing", 1, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, detail, _ := newTestTracker(false)
			w := tr.Apply(&plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: tt.current, Total: tt.total})
			if !reflect.DeepEqual(w.TaskProgress, tt.want) {
				t.Errorf("TaskProgress = %v, want %v", w.TaskProgress, tt.want)
			}
			if detail.FinishedSubTasks != tt.current {
				t.Errorf("FinishedSubTasks = %d, want %d", detail.FinishedSubTasks, tt.current)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestProgressTracker_RetriesFailedWrites(t *testing.T) {
	tests := []struct {
		name string
		// run drives the tracker; fail is the write reported as unsaved.
		run  func(tr *ProgressTracker) (fail []SubtaskRecords, next func() ProgressWrites)
		want []SubtaskRecords
	}{
		{
			name: "current subtask final count is retried on flush",
			run: func(tr *ProgressTracker) ([]SubtaskRecords, func() ProgressWrites) {
				w := tr.Apply(&plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: 1, Total: 2})
				return w.Subtasks, tr.Flush
			},
			want: []SubtaskRecords{{Name: "A", FinishedRecords: 50}},
		},
		{
			name: "previous subtask final count is retried with the next write",
			run: func(tr *ProgressTracker) ([]SubtaskRecords, func() ProgressWrites) {
				w := tr.Apply(setSubtask("B"))
				return w.Subtasks, tr.Flush
			},
			want: []SubtaskRecords{{Name: "A", FinishedRecords: 50}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, _, _ := newTestTracker(false)
			tr.Apply(setSubtask("A"))
			for i := 1; i <= 50; i++ {
				tr.Apply(incRecords(i))
			}
			fail, next := tt.run(tr)
			if len(fail) == 0 {
				t.Fatal("expected a write to fail")
			}
			tr.Unsaved(fail)
			if got := subtaskWrites(next()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("retried writes = %v, want %v", got, tt.want)
			}
			if again := subtaskWrites(tr.Flush()); len(again) != 0 {
				t.Errorf("flush after successful retry wrote %v, want nothing", again)
			}
		})
	}
}

func TestProgressTracker_UnsavedIgnoresStaleCount(t *testing.T) {
	tr, _, clock := newTestTracker(false)
	tr.Apply(setSubtask("A"))
	stale := tr.Apply(incRecords(1)).Subtasks
	clock.advance(SubtaskProgressInterval)
	tr.Apply(incRecords(2)) // saved successfully after the stale one
	tr.Unsaved(stale)
	if got := subtaskWrites(tr.Flush()); len(got) != 0 {
		t.Errorf("flush wrote %v, want nothing: newer count 2 was already saved", got)
	}
}

func TestSaveProgressWrites(t *testing.T) {
	dbErr := errors.Default.New("db down")
	tests := []struct {
		name       string
		failName   string
		wantFailed []SubtaskRecords
		wantErr    bool
	}{
		{"all succeed", "", nil, false},
		{"one subtask fails", "B", []SubtaskRecords{{Name: "B", FinishedRecords: 2}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := unithelper.DummyBasicRes(func(mockDal *mockdal.Dal) {
				mockDal.On("UpdateColumn", mock.Anything, "progress", float32(0.5), mock.Anything).Return(nil).Once()
				mockDal.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Return(
					func(_ interface{}, set []dal.DalSet, _ ...dal.Clause) errors.Error {
						if tt.failName != "" && set[0].Value == 2 {
							return dbErr
						}
						return nil
					}).Twice()
			})
			pct := float32(0.5)
			failed, err := SaveProgressWrites(res, 7, ProgressWrites{
				TaskProgress: &pct,
				Subtasks:     []SubtaskRecords{{Name: "A", FinishedRecords: 1}, {Name: "B", FinishedRecords: 2}},
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(failed, tt.wantFailed) {
				t.Errorf("failed = %v, want %v", failed, tt.wantFailed)
			}
			mockDal := res.GetDal().(*mockdal.Dal)
			mockDal.AssertNumberOfCalls(t, "UpdateColumn", 1)
			mockDal.AssertNumberOfCalls(t, "UpdateColumns", 2)
		})
	}
}
