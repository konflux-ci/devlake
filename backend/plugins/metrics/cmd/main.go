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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/apache/incubator-devlake/plugins/metrics/api"
	"github.com/apache/incubator-devlake/plugins/metrics/api/routes/pr"
	"github.com/apache/incubator-devlake/plugins/metrics/query"
)

func main() {
	cfg := loadConfig()

	db, err := query.Open(cfg.dsn)
	if err != nil {
		log.Fatalf("metrics-api: failed to connect to MySQL: %v", err)
	}
	defer func() { _ = db.Close() }()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", api.Healthz(db))
	mux.HandleFunc("/api/metrics/", api.NotImplemented)

	pr.Register(mux, db)

	srv := &http.Server{
		Addr:         cfg.addr,
		Handler:      api.NewServer(mux, cfg.allowedOrigin),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Printf("metrics-api: listening on %s (allowed origin: %s)", cfg.addr, cfg.allowedOrigin)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("metrics-api: server error: %v", err)
	}
}

type config struct {
	dsn           string
	addr          string
	allowedOrigin string
}

func loadConfig() config {
	host := envOrDefault("MYSQL_HOST", "localhost")
	port := envOrDefault("MYSQL_PORT", "3306")
	user := envOrDefault("MYSQL_USER", "merico")
	pass := os.Getenv("MYSQL_PASS")
	db := envOrDefault("MYSQL_DB", "lake")

	if pass == "" {
		log.Fatal("metrics-api: MYSQL_PASS environment variable is required")
	}

	return config{
		dsn:           fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4", user, pass, host, port, db),
		addr:          envOrDefault("METRICS_ADDR", ":8181"),
		allowedOrigin: envOrDefault("METRICS_ALLOWED_ORIGIN", "https://devtools.pages.redhat.com"),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
