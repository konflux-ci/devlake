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

package pr

import (
	"database/sql"
	"net/http"

	prhandlers "github.com/apache/incubator-devlake/plugins/metrics/api/routes/pr/handlers"
)

func Register(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/metrics/pr/key-metrics", prhandlers.KeyMetrics(db))
	mux.HandleFunc("POST /api/metrics/pr/stages", prhandlers.Stages(db))
	mux.HandleFunc("POST /api/metrics/pr/cycle-time", prhandlers.CycleTime(db))
	mux.HandleFunc("POST /api/metrics/pr/productivity", prhandlers.Productivity(db))
	mux.HandleFunc("POST /api/metrics/pr/flow", prhandlers.Flow(db))
	mux.HandleFunc("POST /api/metrics/pr/z-score", prhandlers.ZScore(db))
	mux.HandleFunc("POST /api/metrics/pr/scatter", prhandlers.Scatter(db))
}
