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
	"net/url"
	"testing"
)

func TestSanitizeQuery_RemovesCaCert(t *testing.T) {
	q := url.Values{}
	q.Set("charset", "utf8mb4")
	q.Set("ca-cert", "/etc/ssl/rds-ca.pem")
	q.Set("loc", "UTC")

	result := sanitizeQuery(q)

	if q.Get("ca-cert") != "" {
		t.Error("sanitizeQuery should remove ca-cert from query")
	}
	if q.Get("loc") != "UTC" {
		t.Error("sanitizeQuery should preserve existing loc value")
	}
	// ca-cert should not appear in the encoded result
	if result != "charset=utf8mb4&loc=UTC" {
		t.Errorf("unexpected query string: %s", result)
	}
}

func TestSanitizeQuery_SetsDefaultLoc(t *testing.T) {
	q := url.Values{}
	q.Set("charset", "utf8mb4")

	sanitizeQuery(q)

	if q.Get("loc") != "Local" {
		t.Errorf("expected default loc=Local, got %s", q.Get("loc"))
	}
}
