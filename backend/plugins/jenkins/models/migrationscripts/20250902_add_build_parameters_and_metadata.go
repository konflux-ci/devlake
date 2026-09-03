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

package migrationscripts

import (
	"github.com/apache/incubator-devlake/core/context"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/helpers/migrationhelper"
	"github.com/apache/incubator-devlake/plugins/jenkins/models"
)

type addBuildParametersAndMetadata struct{}

type jenkinsBuild20250902 struct {
	Metadata map[string]string `gorm:"type:json;serializer:json" json:"metadata"`
}

func (jenkinsBuild20250902) TableName() string {
	return "_tool_jenkins_builds"
}

type jenkinsScopeConfig20250902 struct {
	FieldExtractors []models.FieldExtractorRule `gorm:"type:json;serializer:json" json:"fieldExtractors" mapstructure:"fieldExtractors"`
}

func (jenkinsScopeConfig20250902) TableName() string {
	return "_tool_jenkins_scope_configs"
}

func (*addBuildParametersAndMetadata) Up(baseRes context.BasicRes) errors.Error {
	return migrationhelper.AutoMigrateTables(
		baseRes,
		&models.JenkinsBuildParameter{},
		&jenkinsBuild20250902{},
		&jenkinsScopeConfig20250902{},
	)
}

func (*addBuildParametersAndMetadata) Version() uint64 {
	return 20250902100001
}

func (*addBuildParametersAndMetadata) Name() string {
	return "add jenkins build parameters metadata and field extractors"
}
