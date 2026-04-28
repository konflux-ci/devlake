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
	"encoding/xml"
	"testing"

	testTasks "github.com/apache/incubator-devlake/plugins/testregistry/tasks"
)

func TestGenerateWebhookUID(t *testing.T) {
	// Should generate unique IDs
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateWebhookUID()
		if len(id) != 16 {
			t.Errorf("generateWebhookUID() length = %d, want 16", len(id))
		}
		if ids[id] {
			t.Errorf("generateWebhookUID() produced duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateWebhookUID_IsHex(t *testing.T) {
	id := generateWebhookUID()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateWebhookUID() contains non-hex character: %c in %s", c, id)
		}
	}
}

func TestParseJUnitXML_TestSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="step graph" tests="3" failures="1" skipped="0" time="456.5">
    <testcase name="pre phase" time="120.3"/>
    <testcase name="test phase" time="300.2">
      <failure message="test failed">Expected healthy cluster</failure>
    </testcase>
    <testcase name="post phase" time="36.0"/>
  </testsuite>
</testsuites>`)

	var suitesXml testTasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse testsuites XML: %v", err)
	}

	if len(suitesXml.Suites) != 1 {
		t.Fatalf("Expected 1 suite, got %d", len(suitesXml.Suites))
	}

	suite := suitesXml.Suites[0]
	if suite.Name != "step graph" {
		t.Errorf("Suite name = %q, want %q", suite.Name, "step graph")
	}
	if suite.NumTests != 3 {
		t.Errorf("Suite NumTests = %d, want 3", suite.NumTests)
	}
	if suite.NumFailed != 1 {
		t.Errorf("Suite NumFailed = %d, want 1", suite.NumFailed)
	}
	if len(suite.TestCases) != 3 {
		t.Fatalf("Expected 3 test cases, got %d", len(suite.TestCases))
	}

	// Verify passed test
	if suite.TestCases[0].Name != "pre phase" {
		t.Errorf("TestCase[0].Name = %q, want %q", suite.TestCases[0].Name, "pre phase")
	}
	if suite.TestCases[0].FailureOutput != nil {
		t.Error("TestCase[0] should not have failure output")
	}

	// Verify failed test
	if suite.TestCases[1].FailureOutput == nil {
		t.Fatal("TestCase[1] should have failure output")
	}
	if suite.TestCases[1].FailureOutput.Message != "test failed" {
		t.Errorf("TestCase[1].FailureOutput.Message = %q, want %q", suite.TestCases[1].FailureOutput.Message, "test failed")
	}
}

func TestParseJUnitXML_BareTestSuite(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="e2e-console" tests="3" failures="0" skipped="1" time="60">
  <testcase name="render dashboard" classname="DashboardPage" time="12.3"/>
  <testcase name="create project" classname="ProjectPage" time="8.7"/>
  <testcase name="test GPU" classname="GPUPage" time="0">
    <skipped message="GPU not available"/>
  </testcase>
</testsuite>`)

	// First attempt with TestSuites should fail
	var suitesXml testTasks.TestSuites
	err := xml.Unmarshal(xmlContent, &suitesXml)
	if err == nil && len(suitesXml.Suites) > 0 {
		t.Fatal("Expected TestSuites parse to fail or return empty suites for bare <testsuite>")
	}

	// Fallback to single TestSuite should work
	var singleSuite testTasks.TestSuite
	if err := xml.Unmarshal(xmlContent, &singleSuite); err != nil {
		t.Fatalf("Failed to parse bare testsuite XML: %v", err)
	}

	if singleSuite.Name != "e2e-console" {
		t.Errorf("Suite name = %q, want %q", singleSuite.Name, "e2e-console")
	}
	if singleSuite.NumTests != 3 {
		t.Errorf("Suite NumTests = %d, want 3", singleSuite.NumTests)
	}
	if singleSuite.NumSkipped != 1 {
		t.Errorf("Suite NumSkipped = %d, want 1", singleSuite.NumSkipped)
	}
	if len(singleSuite.TestCases) != 3 {
		t.Fatalf("Expected 3 test cases, got %d", len(singleSuite.TestCases))
	}

	// Verify classname
	if singleSuite.TestCases[0].Classname != "DashboardPage" {
		t.Errorf("TestCase[0].Classname = %q, want %q", singleSuite.TestCases[0].Classname, "DashboardPage")
	}

	// Verify skipped test
	if singleSuite.TestCases[2].SkipMessage == nil {
		t.Fatal("TestCase[2] should have skip message")
	}
	if singleSuite.TestCases[2].SkipMessage.Message != "GPU not available" {
		t.Errorf("TestCase[2].SkipMessage = %q, want %q", singleSuite.TestCases[2].SkipMessage.Message, "GPU not available")
	}
}

func TestParseJUnitXML_EmptyTestSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
</testsuites>`)

	var suitesXml testTasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse empty testsuites XML: %v", err)
	}

	if len(suitesXml.Suites) != 0 {
		t.Errorf("Expected 0 suites, got %d", len(suitesXml.Suites))
	}
}

func TestParseJUnitXML_InvalidXML(t *testing.T) {
	xmlContent := []byte(`this is not XML at all`)

	var suitesXml testTasks.TestSuites
	err := xml.Unmarshal(xmlContent, &suitesXml)
	if err == nil {
		t.Error("Expected error parsing invalid XML")
	}
}

func TestParseJUnitXML_MultipleSuites(t *testing.T) {
	xmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<testsuites>
  <testsuite name="unit" tests="2" failures="0" time="5">
    <testcase name="TestAdd" time="0.1"/>
    <testcase name="TestSub" time="0.2"/>
  </testsuite>
  <testsuite name="integration" tests="1" failures="1" time="300">
    <testcase name="TestE2E" time="300">
      <failure message="timeout">Connection refused</failure>
    </testcase>
  </testsuite>
</testsuites>`)

	var suitesXml testTasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse multi-suite XML: %v", err)
	}

	if len(suitesXml.Suites) != 2 {
		t.Fatalf("Expected 2 suites, got %d", len(suitesXml.Suites))
	}

	if suitesXml.Suites[0].Name != "unit" {
		t.Errorf("Suite[0].Name = %q, want %q", suitesXml.Suites[0].Name, "unit")
	}
	if suitesXml.Suites[1].Name != "integration" {
		t.Errorf("Suite[1].Name = %q, want %q", suitesXml.Suites[1].Name, "integration")
	}
	if len(suitesXml.Suites[0].TestCases) != 2 {
		t.Errorf("Suite[0] test cases = %d, want 2", len(suitesXml.Suites[0].TestCases))
	}
	if suitesXml.Suites[1].TestCases[0].FailureOutput == nil {
		t.Error("Suite[1].TestCase[0] should have failure output")
	}
}

func TestTestCaseStatusDetection(t *testing.T) {
	xmlContent := []byte(`<testsuites>
  <testsuite name="status-test" tests="3" failures="1" skipped="1">
    <testcase name="passed-test" time="1.0"/>
    <testcase name="failed-test" time="2.0">
      <failure message="assertion failed">expected true, got false</failure>
    </testcase>
    <testcase name="skipped-test">
      <skipped message="not ready"/>
    </testcase>
  </testsuite>
</testsuites>`)

	var suitesXml testTasks.TestSuites
	if err := xml.Unmarshal(xmlContent, &suitesXml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	cases := suitesXml.Suites[0].TestCases
	if len(cases) != 3 {
		t.Fatalf("Expected 3 cases, got %d", len(cases))
	}

	// Passed: no failure, no skip
	if cases[0].FailureOutput != nil || cases[0].SkipMessage != nil {
		t.Error("passed-test should have no failure/skip")
	}

	// Failed: has failure output
	if cases[1].FailureOutput == nil {
		t.Fatal("failed-test should have failure output")
	}
	if cases[1].FailureOutput.Message != "assertion failed" {
		t.Errorf("failure message = %q, want %q", cases[1].FailureOutput.Message, "assertion failed")
	}
	if cases[1].FailureOutput.Output != "expected true, got false" {
		t.Errorf("failure output = %q, want %q", cases[1].FailureOutput.Output, "expected true, got false")
	}

	// Skipped: has skip message
	if cases[2].SkipMessage == nil {
		t.Fatal("skipped-test should have skip message")
	}
	if cases[2].SkipMessage.Message != "not ready" {
		t.Errorf("skip message = %q, want %q", cases[2].SkipMessage.Message, "not ready")
	}
}

func TestMaxJUnitFilesConstant(t *testing.T) {
	if maxJUnitFilesPerRequest != 100 {
		t.Errorf("maxJUnitFilesPerRequest = %d, want 100", maxJUnitFilesPerRequest)
	}
}
