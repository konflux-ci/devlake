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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/unithelper"
	mockconfig "github.com/apache/incubator-devlake/mocks/core/config"
	mockdal "github.com/apache/incubator-devlake/mocks/core/dal"
)

// useProgressRes swaps the package basicRes for a mock whose DAL is configured
// by setup, and makes final-flush retries instant.
func useProgressRes(t *testing.T, setup func(mockDal *mockdal.Dal)) {
	t.Helper()
	cfg := new(mockconfig.ConfigReader)
	cfg.On("GetBool", "SKIP_SUBTASK_PROGRESS").Return(false)
	res := unithelper.DummyBasicRes(setup)
	res.On("GetConfigReader").Return(cfg)

	prevRes, prevBackoff := basicRes, finalFlushBackoff
	basicRes = res
	finalFlushBackoff = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() { basicRes, finalFlushBackoff = prevRes, prevBackoff })
}

// runProgress feeds events to updateTaskProgress and waits for done.
func runProgress(t *testing.T, taskId uint64, events ...plugin.RunningProgress) {
	t.Helper()
	progress := make(chan plugin.RunningProgress, len(events))
	for _, e := range events {
		progress <- e
	}
	close(progress)
	done := make(chan struct{}, 1)
	go updateTaskProgress(done, taskId, progress)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("updateTaskProgress did not signal done")
	}
}

// Regression: when the task was missing from runningTasks, updateTaskProgress
// returned without draining the channel or signalling done, so the caller
// blocked forever on <-doneSignal.
func TestUpdateTaskProgress_MissingTaskStillDrainsAndSignalsDone(t *testing.T) {
	useProgressRes(t, func(mockDal *mockdal.Dal) {
		mockDal.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
		mockDal.On("UpdateColumn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	})

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

// No event follows the final flush, so a failed final write must be retried
// before updateTaskProgress returns, and retries must stop eventually.
func TestUpdateTaskProgress_RetriesFinalFlush(t *testing.T) {
	dbErr := errors.Default.New("db down")
	tests := []struct {
		name string
		// failures is how many final-flush writes fail before one succeeds.
		failures  int
		wantCalls int
		wantSaved int
	}{
		{"succeeds first time", 0, 2, 42},
		{"succeeds on retry", 2, 4, 42},
		{"gives up after bounded retries", 10, 1 + 1 + 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			calls, saved := 0, 0
			useProgressRes(t, func(mockDal *mockdal.Dal) {
				mockDal.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Return(
					func(_ interface{}, set []dal.DalSet, _ ...dal.Clause) errors.Error {
						mu.Lock()
						defer mu.Unlock()
						calls++
						n := set[0].Value.(int)
						// the first write (1 record) is the in-loop one
						if n != 1 && calls-1 <= tt.failures {
							return dbErr
						}
						if n != 1 {
							saved = n
						}
						return nil
					})
			})

			runProgress(t, 999_998,
				plugin.RunningProgress{Type: plugin.SetCurrentSubTask, SubTaskName: "Extract", SubTaskNumber: 1},
				plugin.RunningProgress{Type: plugin.SubTaskIncProgress, Current: 1, Total: -1},
				plugin.RunningProgress{Type: plugin.SubTaskIncProgress, Current: 42, Total: -1},
			)

			mu.Lock()
			defer mu.Unlock()
			if calls != tt.wantCalls {
				t.Errorf("UpdateColumns calls = %d, want %d", calls, tt.wantCalls)
			}
			if saved != tt.wantSaved {
				t.Errorf("saved final count = %d, want %d", saved, tt.wantSaved)
			}
		})
	}
}

// Stress test for the root cause of "error iterating raw data cursor: invalid
// connection": many concurrent extractors each report one progress event per
// record while every DB write costs a round trip. Previously each event did an
// UPDATE under runningTasks.mu, which all tasks share, so writes were
// serialized: 50 tasks x 1000 records x 2ms ≈ 100s, slowing every extractor to
// a crawl and keeping raw cursors open until MySQL dropped them. Now writes
// are throttled and made outside the lock, so the whole run takes well under a
// second and issues a few writes per task.
func TestUpdateTaskProgress_ConcurrentExtractorsAreNotSerialized(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test")
	}
	const (
		tasks       = 50
		records     = 1000
		writeCost   = 2 * time.Millisecond
		maxDuration = 10 * time.Second
	)
	var writes atomic.Int64
	useProgressRes(t, func(mockDal *mockdal.Dal) {
		slowWrite := func(mock.Arguments) {
			writes.Add(1)
			time.Sleep(writeCost)
		}
		mockDal.On("UpdateColumns", mock.Anything, mock.Anything, mock.Anything).Run(slowWrite).Return(nil)
		mockDal.On("UpdateColumn", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(slowWrite).Return(nil)
	})

	const baseId = 900_000
	for i := uint64(0); i < tasks; i++ {
		if err := runningTasks.Add(baseId+i, func() {}); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for i := uint64(0); i < tasks; i++ {
			_, _ = runningTasks.Remove(baseId + i)
		}
	})

	start := time.Now()
	var wg sync.WaitGroup
	for i := uint64(0); i < tasks; i++ {
		wg.Add(1)
		go func(taskId uint64) {
			defer wg.Done()
			// same channel size and send pattern as the real task runner
			progress := make(chan plugin.RunningProgress, 100)
			done := make(chan struct{}, 1)
			go updateTaskProgress(done, taskId, progress)
			progress <- plugin.RunningProgress{Type: plugin.TaskSetProgress, Current: 0, Total: 1}
			progress <- plugin.RunningProgress{Type: plugin.SetCurrentSubTask, SubTaskName: "Extract Users", SubTaskNumber: 1}
			progress <- plugin.RunningProgress{Type: plugin.SubTaskSetProgress, Current: 0, Total: -1}
			for r := 1; r <= records; r++ {
				progress <- plugin.RunningProgress{Type: plugin.SubTaskIncProgress, Current: r, Total: -1}
			}
			progress <- plugin.RunningProgress{Type: plugin.TaskIncProgress, Current: 1, Total: 1}
			close(progress)
			<-done
		}(baseId + i)
	}

	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(maxDuration):
		t.Fatalf("%d concurrent tasks x %d records did not finish within %s: progress writes are serialized", tasks, records, maxDuration)
	}
	elapsed := time.Since(start)

	// Per task: first record, final count, task progress. Allow slack for the
	// 2s throttle interval elapsing on a slow CI machine.
	if got, limit := writes.Load(), int64(tasks*6); got > limit {
		t.Errorf("DB writes = %d, want <= %d (one per record would be %d)", got, limit, tasks*records)
	}
	for i := uint64(0); i < tasks; i++ {
		if d := runningTasks.GetProgressDetail(baseId + i); d == nil || d.FinishedRecords != records {
			t.Fatalf("task %d progress detail = %+v, want %d finished records", baseId+i, d, records)
		}
	}
	t.Logf("%d tasks x %d records in %s with %d DB writes", tasks, records, elapsed, writes.Load())
}
