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
	"github.com/apache/incubator-devlake/core/runner"
	"github.com/apache/incubator-devlake/plugins/github_snowflake/impl"
	"github.com/spf13/cobra"
)

// PluginEntry is exported for the DevLake framework to discover and load this plugin.
var PluginEntry impl.GithubSnowflake //nolint

func main() {
	cmd := &cobra.Command{Use: "github_snowflake"}
	connectionId := cmd.Flags().Uint64P("connectionId", "c", 0, "github_snowflake connection id")
	githubId := cmd.Flags().IntP("githubId", "g", 0, "GitHub repository numeric ID")
	fullName := cmd.Flags().StringP("fullName", "n", "", "GitHub repository full name (owner/repo)")
	timeAfter := cmd.Flags().StringP("timeAfter", "a", "", "only sync records created/updated after this time (RFC3339)")
	_ = cmd.MarkFlagRequired("connectionId")
	_ = cmd.MarkFlagRequired("githubId")
	_ = cmd.MarkFlagRequired("fullName")

	cmd.Run = func(cmd *cobra.Command, args []string) {
		runner.DirectRun(cmd, args, PluginEntry, map[string]interface{}{
			"connectionId": *connectionId,
			"githubId":     *githubId,
			"name":         *fullName,
			"fullName":     *fullName,
		}, *timeAfter)
	}
	runner.RunCmd(cmd)
}
