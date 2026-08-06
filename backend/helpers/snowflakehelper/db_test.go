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

package snowflakehelper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestPKCS8PEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	return string(pem.EncodeToMemory(block))
}

func TestOpen_InvalidAuthType(t *testing.T) {
	_, err := Open("acct", "user", "keypai", "", "db", "schema", "", "")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported authType")
}

func TestOpen_EmptyPrivateKeyForKeypair(t *testing.T) {
	for _, authType := range []string{"", "keypair"} {
		t.Run(fmt.Sprintf("authType=%q", authType), func(t *testing.T) {
			_, err := Open("acct", "user", authType, "", "db", "schema", "", "")
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), "privateKey is required for keypair auth")
		})
	}
}

func TestParseRSAPrivateKey_ValidPKCS8(t *testing.T) {
	pemStr := generateTestPKCS8PEM(t)
	key, err := ParseRSAPrivateKey(pemStr)
	require.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 2048, key.N.BitLen())
}

func TestParseRSAPrivateKey_EmptyString(t *testing.T) {
	_, err := ParseRSAPrivateKey("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PEM")
}

func TestParseRSAPrivateKey_InvalidPEM(t *testing.T) {
	_, err := ParseRSAPrivateKey("not a pem block")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PEM")
}

func TestParseRSAPrivateKey_WrongKeyType(t *testing.T) {
	// PKCS#1 format (BEGIN RSA PRIVATE KEY) is not PKCS#8, should fail ParsePKCS8PrivateKey
	key, genErr := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, genErr)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pemStr := string(pem.EncodeToMemory(block))

	_, err := ParseRSAPrivateKey(pemStr)
	assert.Error(t, err, "PKCS#1 key should be rejected; expected PKCS#8")
}
