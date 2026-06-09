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

package parser

import (
	"testing"
	"time"
)

func TestChooseCloneStrategy(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		since          *time.Time
		forceFullClone bool
		noShallowClone bool
		want           cloneStrategy
	}{
		{
			name:           "full sync (since=nil)",
			since:          nil,
			forceFullClone: false,
			noShallowClone: false,
			want:           strategyFull,
		},
		{
			name:           "full sync takes precedence over noShallowClone",
			since:          nil,
			forceFullClone: false,
			noShallowClone: true,
			want:           strategyFull,
		},
		{
			name:           "forceFullClone overrides incremental to full",
			since:          &now,
			forceFullClone: true,
			noShallowClone: false,
			want:           strategyFull,
		},
		{
			name:           "forceFullClone takes precedence over noShallowClone",
			since:          &now,
			forceFullClone: true,
			noShallowClone: true,
			want:           strategyFull,
		},
		{
			name:           "forceFullClone with nil since is still full",
			since:          nil,
			forceFullClone: true,
			noShallowClone: false,
			want:           strategyFull,
		},
		{
			name:           "forceFullClone with nil since takes precedence over noShallowClone",
			since:          nil,
			forceFullClone: true,
			noShallowClone: true,
			want:           strategyFull,
		},
		{
			name:           "noShallowClone on incremental uses doubleClone",
			since:          &now,
			forceFullClone: false,
			noShallowClone: true,
			want:           strategyDoubleClone,
		},
		{
			name:           "default incremental uses shallow",
			since:          &now,
			forceFullClone: false,
			noShallowClone: false,
			want:           strategyShallow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := chooseCloneStrategy(tt.since, tt.forceFullClone, tt.noShallowClone)
			if got != tt.want {
				t.Errorf("chooseCloneStrategy() = %d, want %d", got, tt.want)
			}
		})
	}
}
