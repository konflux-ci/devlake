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
	"github.com/apache/incubator-devlake/core/dal"
	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/plugin"
)

type jiraSnowflakePrivateKeyPlain struct {
	ID         uint64 `gorm:"primaryKey"`
	PrivateKey string `gorm:"column:private_key"`
}

func (jiraSnowflakePrivateKeyPlain) TableName() string {
	return "_tool_jira_snowflake_connections"
}

// encryptPrivateKey encrypts existing plaintext private_key values so that the
// connection model's gorm serializer:encdec can decrypt them on read.
type encryptPrivateKey struct{}

func (*encryptPrivateKey) Up(basicRes context.BasicRes) errors.Error {
	db := basicRes.GetDal()
	encKey := basicRes.GetConfig(plugin.EncodeKeyEnvStr)
	if encKey == "" {
		return errors.BadInput.New("ENCRYPTION_SECRET is required to encrypt jira_snowflake private keys")
	}

	cursor, err := db.Cursor(dal.From(&jiraSnowflakePrivateKeyPlain{}))
	if err != nil {
		return err
	}
	defer cursor.Close()

	for cursor.Next() {
		row := &jiraSnowflakePrivateKeyPlain{}
		if err = db.Fetch(cursor, row); err != nil {
			return err
		}
		if row.PrivateKey == "" {
			continue
		}
		encrypted, err := plugin.Encrypt(encKey, row.PrivateKey)
		if err != nil {
			return err
		}
		err = db.UpdateColumns(
			row.TableName(),
			[]dal.DalSet{{ColumnName: "private_key", Value: encrypted}},
			dal.Where("id = ?", row.ID),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (*encryptPrivateKey) Version() uint64 {
	return 20260803000001
}

func (*encryptPrivateKey) Name() string {
	return "encrypt jira_snowflake connection private keys"
}
