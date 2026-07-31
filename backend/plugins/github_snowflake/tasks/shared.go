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

package tasks

import (
	"fmt"
)

// deriveRepoHTMLUrl builds the GitHub HTML URL from FULL_NAME.
func deriveRepoHTMLUrl(fullName string) string {
	return fmt.Sprintf("https://github.com/%s", fullName)
}

// deriveRepoCloneUrl builds the GitHub clone URL from FULL_NAME.
func deriveRepoCloneUrl(fullName string) string {
	return fmt.Sprintf("https://github.com/%s.git", fullName)
}

// derivePullRequestURL builds the GitHub PR URL.
func derivePullRequestURL(fullName string, number int) string {
	return fmt.Sprintf("https://github.com/%s/pull/%d", fullName, number)
}

// nullStr returns the string value or empty string when nil.
func nullStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// nullInt returns the int value or 0 when nil.
func nullInt(i *int64) int {
	if i == nil {
		return 0
	}
	return int(*i)
}
