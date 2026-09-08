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
	"github.com/apache/incubator-devlake/core/plugin"
)

var _ plugin.MigrationScript = (*addFullsendConfig)(nil)

type addFullsendConfig struct{}

func (script *addFullsendConfig) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()

	err := db.AutoMigrate(&scopeConfigAddFullsend20260908{})
	if err != nil {
		return errors.Default.Wrap(err, "failed to add Fullsend columns to _tool_aireview_scope_configs")
	}

	return nil
}

func (script *addFullsendConfig) Version() uint64 {
	return 20260908000001
}

func (script *addFullsendConfig) Name() string {
	return "aireview add Fullsend configuration"
}

type scopeConfigAddFullsend20260908 struct {
	FullsendEnabled  bool   `gorm:"type:boolean;default:true"`
	FullsendUsername string `gorm:"type:varchar(255)"`
	FullsendPattern  string `gorm:"type:varchar(500)"`
}

func (scopeConfigAddFullsend20260908) TableName() string {
	return "_tool_aireview_scope_configs"
}
