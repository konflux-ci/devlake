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

package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// NewServer wraps an already-routed mux with CORS and logging middleware.
// Route registration is the caller's responsibility — each handler group
// calls its own Register(mux, db) so this package stays import-cycle-free.
func NewServer(mux *http.ServeMux, allowedOrigin string) http.Handler {
	return corsMiddleware(allowedOrigin, loggingMiddleware(mux))
}

// Healthz returns a liveness handler that pings the database.
func Healthz(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, fmt.Sprintf("db ping failed: %v", err), http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// NotImplemented is a catch-all handler for routes not yet wired up.
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	http.Error(w, fmt.Sprintf("not implemented: %s", r.URL.Path), http.StatusNotImplemented)
}

// WriteJSON encodes v as JSON with a 200 status.
func WriteJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("metrics-api: WriteJSON error: %v", err)
	}
}
