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
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/apache/incubator-devlake/core/errors"
	"github.com/apache/incubator-devlake/core/models/common"
	"github.com/apache/incubator-devlake/core/plugin"
	"github.com/apache/incubator-devlake/helpers/dbhelper"
	"github.com/apache/incubator-devlake/plugins/testregistry/models"
	testTasks "github.com/apache/incubator-devlake/plugins/testregistry/tasks"
	webhookModels "github.com/apache/incubator-devlake/plugins/webhook/models"
)

const maxJUnitFilesPerRequest = 100

// PostTestResults handles pushing CI test results via webhook by connection ID.
// Accepts multipart/form-data with job metadata as form fields and JUnit XML as file uploads.
func PostTestResults(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &webhookModels.WebhookConnection{}
	err := connectionHelper.First(connection, input.Params)
	if err != nil {
		return nil, err
	}
	return postTestResultsImpl(input, connection.ID)
}

// PostTestResultsByName handles pushing CI test results via webhook by connection name.
func PostTestResultsByName(input *plugin.ApiResourceInput) (*plugin.ApiResourceOutput, errors.Error) {
	connection := &webhookModels.WebhookConnection{}
	err := connectionHelper.FirstByName(connection, input.Params)
	if err != nil {
		return nil, err
	}
	return postTestResultsImpl(input, connection.ID)
}

func postTestResultsImpl(input *plugin.ApiResourceInput, connectionId uint64) (*plugin.ApiResourceOutput, errors.Error) {
	if input.Request == nil {
		return nil, errors.BadInput.New("request must be multipart/form-data with job metadata as form fields and JUnit XML as file uploads (field name: junit)")
	}

	if err := input.Request.ParseMultipartForm(32 << 20); err != nil {
		return nil, errors.BadInput.Wrap(err, "failed to parse multipart form")
	}

	// Read required form fields
	jobId := input.Request.FormValue("jobId")
	jobName := input.Request.FormValue("jobName")
	organization := input.Request.FormValue("organization")
	repository := input.Request.FormValue("repository")
	result := input.Request.FormValue("result")

	if jobId == "" || jobName == "" || organization == "" || repository == "" || result == "" {
		return nil, errors.BadInput.New("required form fields: jobId, jobName, organization, repository, result")
	}

	// Read optional form fields
	jobType := input.Request.FormValue("jobType")
	commitSha := input.Request.FormValue("commitSha")
	pullRequestAuthor := input.Request.FormValue("pullRequestAuthor")
	triggerType := input.Request.FormValue("triggerType")
	viewUrl := input.Request.FormValue("viewUrl")
	scopeId := input.Request.FormValue("scopeId")

	var pullRequestNumber *int
	if prStr := input.Request.FormValue("pullRequestNumber"); prStr != "" {
		if prNum, err := strconv.Atoi(prStr); err == nil {
			pullRequestNumber = &prNum
		}
	}

	var startedAt, finishedAt *time.Time
	if s := input.Request.FormValue("startedAt"); s != "" {
		if t, err := common.ConvertStringToTime(s); err == nil {
			startedAt = &t
		}
	}
	if s := input.Request.FormValue("finishedAt"); s != "" {
		if t, err := common.ConvertStringToTime(s); err == nil {
			finishedAt = &t
		}
	}

	var durationSec *float64
	if s := input.Request.FormValue("durationSec"); s != "" {
		if d, err := strconv.ParseFloat(s, 64); err == nil {
			durationSec = &d
		}
	} else if startedAt != nil && finishedAt != nil {
		d := finishedAt.Sub(*startedAt).Seconds()
		durationSec = &d
	}

	if scopeId == "" {
		scopeId = repository
	}

	// Enforce file count limit
	junitFiles := input.Request.MultipartForm.File["junit"]
	if len(junitFiles) > maxJUnitFilesPerRequest {
		return nil, errors.BadInput.New(fmt.Sprintf("too many JUnit files; maximum is %d", maxJUnitFilesPerRequest))
	}

	// Clean up multipart temp files when done
	defer func() {
		if input.Request.MultipartForm != nil {
			_ = input.Request.MultipartForm.RemoveAll()
		}
	}()

	// Wrap all writes in a transaction for consistency
	var err errors.Error
	txHelper := dbhelper.NewTxHelper(basicRes, &err)
	defer txHelper.End()
	db := txHelper.Begin()

	// Build domain job ID with webhook prefix
	domainJobId := fmt.Sprintf("webhook:%d:%s", connectionId, jobId)

	// Save CI job
	ciJob := &models.TestRegistryCIJob{
		ConnectionId:      connectionId,
		JobId:             domainJobId,
		JobName:           jobName,
		JobType:           jobType,
		Organization:      organization,
		Repository:        repository,
		CommitSHA:         commitSha,
		PullRequestNumber: pullRequestNumber,
		PullRequestAuthor: pullRequestAuthor,
		TriggerType:       triggerType,
		Result:            result,
		StartedAt:         startedAt,
		FinishedAt:        finishedAt,
		DurationSec:       durationSec,
		ViewURL:           viewUrl,
		ScopeId:           scopeId,
	}

	if dbErr := db.CreateOrUpdate(ciJob); dbErr != nil {
		return nil, errors.Default.Wrap(dbErr, "failed to save CI job")
	}

	savedSuites := 0
	savedCases := 0

	// Parse JUnit XML files from multipart uploads
	for _, fileHeader := range junitFiles {
		f, err := fileHeader.Open()
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("failed to open uploaded file %s", fileHeader.Filename))
		}

		xmlContent, err := io.ReadAll(io.LimitReader(f, 10*1024*1024))
		f.Close()
		if err != nil {
			return nil, errors.Default.Wrap(err, fmt.Sprintf("failed to read uploaded file %s", fileHeader.Filename))
		}

		// Parse XML — try <testsuites> first, then bare <testsuite>
		var suitesXml testTasks.TestSuites
		_ = xml.Unmarshal(xmlContent, &suitesXml)

		// Handle bare <testsuite> root element (or failed <testsuites> parse)
		if len(suitesXml.Suites) == 0 {
			var singleSuite testTasks.TestSuite
			if xmlErr := xml.Unmarshal(xmlContent, &singleSuite); xmlErr == nil && singleSuite.Name != "" {
				suitesXml.Suites = []*testTasks.TestSuite{&singleSuite}
			}
		}

		// Save each suite and its test cases
		for _, suite := range suitesXml.Suites {
			if suite == nil || suite.Name == "" {
				continue
			}

			suiteId := generateWebhookUID()
			testSuite := &models.TestSuite{
				ConnectionId: connectionId,
				JobId:        domainJobId,
				SuiteId:      suiteId,
				Name:         suite.Name,
				NumTests:     suite.NumTests,
				NumFailed:    suite.NumFailed,
				NumSkipped:   suite.NumSkipped,
				Duration:     suite.Duration,
			}

			if dbErr := db.CreateOrUpdate(testSuite); dbErr != nil {
				return nil, errors.Default.Wrap(dbErr, fmt.Sprintf("failed to save test suite %s", suite.Name))
			}
			savedSuites++

			for _, tc := range suite.TestCases {
				if tc == nil {
					continue
				}

				testCaseId := generateWebhookUID()
				status := "passed"
				var failureMsg, failureOut, skipMsg *string

				if tc.FailureOutput != nil {
					status = "failed"
					msg := tc.FailureOutput.Message
					failureMsg = &msg
					out := tc.FailureOutput.Output
					failureOut = &out
				} else if tc.SkipMessage != nil {
					status = "skipped"
					msg := tc.SkipMessage.Message
					skipMsg = &msg
				}

				testCase := &models.TestCase{
					ConnectionId:   connectionId,
					JobId:          domainJobId,
					SuiteId:        suiteId,
					TestCaseId:     testCaseId,
					Name:           tc.Name,
					Classname:      tc.Classname,
					Duration:       tc.Duration,
					Status:         status,
					FailureMessage: failureMsg,
					FailureOutput:  failureOut,
					SkipMessage:    skipMsg,
				}

				if dbErr := db.CreateOrUpdate(testCase); dbErr != nil {
					return nil, errors.Default.Wrap(dbErr, fmt.Sprintf("failed to save test case %s", tc.Name))
				}
				savedCases++
			}
		}
	}

	return &plugin.ApiResourceOutput{
		Body: map[string]interface{}{
			"jobId":       domainJobId,
			"suitesSaved": savedSuites,
			"casesSaved":  savedCases,
			"filesRead":   len(junitFiles),
		},
		Status: http.StatusOK,
	}, nil
}

// generateWebhookUID creates a random 16-char hex string for unique IDs.
func generateWebhookUID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
