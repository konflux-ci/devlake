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
	"log"
	"net/http"

	"github.com/apache/incubator-devlake/plugins/metrics/api"
	qpr "github.com/apache/incubator-devlake/plugins/metrics/query/pr"
	tpr "github.com/apache/incubator-devlake/plugins/metrics/transform/pr"
)

// ZScore handles POST /api/metrics/pr/z-score.
func ZScore(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := api.DecodeQueryParams(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params := toParams(p)

		current, err := qpr.BasePRs(r.Context(), db, params)
		if err != nil {
			log.Printf("pr/z-score: current: %v", err)
			http.Error(w, "query error", http.StatusInternalServerError)
			return
		}

		const ninetyDays = 90 * 24 * 3600
		lookbackParams := params
		lookbackParams.From = params.To - ninetyDays
		lookbackParams.To = params.To
		lookback, err := qpr.BasePRs(r.Context(), db, lookbackParams)
		if err != nil {
			log.Printf("pr/z-score: lookback: %v", err)
			http.Error(w, "query error", http.StatusInternalServerError)
			return
		}

		api.WriteJSON(w, tpr.BuildZScore(lookback, current))
	}
}
