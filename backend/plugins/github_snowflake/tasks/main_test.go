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
	"os"
	"testing"

	"github.com/apache/incubator-devlake/core/plugin"
)

func TestMain(m *testing.M) {
	_ = plugin.RegisterPlugin("github", githubPluginStub{})
	_ = plugin.RegisterPlugin("github_snowflake", GithubSnowflake{})
	os.Exit(m.Run())
}

// githubPluginStub lets didgen resolve plugins/github/models types in unit tests.
type githubPluginStub struct{}

func (githubPluginStub) Name() string { return "github" }
func (githubPluginStub) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/github"
}
func (githubPluginStub) Description() string { return "" }

// GithubSnowflake is a minimal PluginMeta stub for github_snowflake model types.
type GithubSnowflake struct{}

func (GithubSnowflake) Name() string { return "github_snowflake" }
func (GithubSnowflake) RootPkgPath() string {
	return "github.com/apache/incubator-devlake/plugins/github_snowflake"
}
func (GithubSnowflake) Description() string { return "" }
