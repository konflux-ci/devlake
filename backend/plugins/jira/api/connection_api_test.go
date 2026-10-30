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
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJiraHTTPErrorDetail(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		header string
		want   string
	}{
		{
			name: "classic jira errorMessages",
			body: `{"errorMessages":["Client must be authenticated to access this resource."],"errors":{}}`,
			want: "Client must be authenticated to access this resource.",
		},
		{
			name: "atlassian gateway message",
			body: `{"code":401,"message":"Unauthorized; scope does not match"}`,
			want: "Unauthorized; scope does not match",
		},
		{
			name:   "www-authenticate when body empty",
			header: `Bearer error="insufficient_scope"`,
			want:   `Bearer error="insufficient_scope"`,
		},
		{
			name: "html body ignored",
			body: `<html><body>Unauthorized</body></html>`,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			if tt.header != "" {
				res.Header.Set("WWW-Authenticate", tt.header)
			}
			if got := jiraHTTPErrorDetail(res); got != tt.want {
				t.Errorf("jiraHTTPErrorDetail() = %q, want %q", got, tt.want)
			}
		})
	}
}
