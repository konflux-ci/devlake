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
	"fmt"
	"strings"
)

// placeholders returns a "(?, ?, ...)" string with n placeholders.
func placeholders(n int) string {
	if n == 0 {
		return "(NULL)" // prevents SQL syntax error on empty IN lists
	}
	return "(" + strings.Repeat("?,", n-1) + "?" + ")"
}

// repoArgs converts a []string to a []interface{} for use as query args.
func repoArgs(repos []string) []interface{} {
	out := make([]interface{}, len(repos))
	for i, r := range repos {
		out[i] = r
	}
	return out
}

// whitelistArgs converts a []string to []interface{}.
func whitelistArgs(wl []string) []interface{} {
	out := make([]interface{}, len(wl))
	for i, u := range wl {
		out[i] = u
	}
	return out
}

// whitelistSQL returns an optional AND <alias>.author_name IN (...) clause
// and appends the values to args.
func whitelistSQL(p Params, alias string, args *[]interface{}) string {
	if len(p.Whitelist) == 0 {
		return ""
	}
	clause := fmt.Sprintf("AND %s.author_name IN %s", alias, placeholders(len(p.Whitelist)))
	*args = append(*args, whitelistArgs(p.Whitelist)...)
	return clause
}
