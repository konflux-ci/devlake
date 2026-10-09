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

package e2e

import (
	"testing"

	"github.com/apache/incubator-devlake/core/models/domainlayer/code"
	"github.com/apache/incubator-devlake/core/models/domainlayer/crossdomain"
	"github.com/apache/incubator-devlake/helpers/e2ehelper"
	"github.com/apache/incubator-devlake/plugins/aireview/impl"
	"github.com/apache/incubator-devlake/plugins/aireview/models"
	"github.com/apache/incubator-devlake/plugins/aireview/tasks"
)

func TestExtractFullsendAiReviews(t *testing.T) {
	var plug impl.AiReview
	dataflowTester := e2ehelper.NewDataFlowTester(t, "aireview", plug)

	taskData := &tasks.AiReviewTaskData{
		Options: &tasks.AiReviewOptions{
			RepoId:      "github:GithubRepo:1:300",
			ScopeConfig: models.GetDefaultScopeConfig(),
		},
	}
	if err := tasks.CompilePatterns(taskData); err != nil {
		t.Fatalf("failed to compile patterns: %v", err)
	}

	dataflowTester.FlushTabler(&crossdomain.Account{})
	dataflowTester.FlushTabler(&code.PullRequest{})
	dataflowTester.FlushTabler(&code.PullRequestComment{})
	dataflowTester.FlushTabler(&models.AiReview{})

	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_requests_fullsend.csv", &code.PullRequest{})
	dataflowTester.ImportCsvIntoTabler("./raw_tables/pull_request_comments_fullsend.csv", &code.PullRequestComment{})

	dataflowTester.Subtask(tasks.ExtractAiReviewsMeta, taskData)

	var reviews []models.AiReview
	if err := dataflowTester.Dal.All(&reviews); err != nil {
		t.Fatalf("failed to query reviews: %v", err)
	}

	if len(reviews) != 2 {
		t.Fatalf("expected 2 Fullsend reviews, got %d", len(reviews))
	}

	byComment := make(map[string]models.AiReview)
	for _, r := range reviews {
		byComment[r.ReviewId] = r
	}

	for _, r := range reviews {
		if r.AiTool != models.AiToolFullsend {
			t.Errorf("expected AiTool=%s, got %s (review_id=%s)", models.AiToolFullsend, r.AiTool, r.ReviewId)
		}
		if r.SourcePlatform != "github" {
			t.Errorf("expected SourcePlatform=github, got %s", r.SourcePlatform)
		}
		if r.Summary == "" {
			t.Errorf("expected non-empty summary for review_id=%s", r.ReviewId)
		}
	}

	// Review comment from fullsend-ai-review[bot] — structured findings with "breaking" keyword → High risk.
	if review, ok := byComment["github:GithubPrComment:1:5368024626"]; ok {
		if review.AiToolUser != "fullsend-ai-review[bot]" {
			t.Errorf("expected AiToolUser=fullsend-ai-review[bot], got %s", review.AiToolUser)
		}
		if review.RiskLevel != models.RiskLevelHigh {
			t.Errorf("review comment should be high risk (breaking keyword), got %s", review.RiskLevel)
		}
	} else {
		t.Error("review comment from fullsend-ai-review[bot] not found in results")
	}

	// Coder fix comment from fullsend-ai-coder[bot].
	if coder, ok := byComment["github:GithubPrComment:1:5115522730"]; ok {
		if coder.AiToolUser != "fullsend-ai-coder[bot]" {
			t.Errorf("expected AiToolUser=fullsend-ai-coder[bot], got %s", coder.AiToolUser)
		}
	} else {
		t.Error("coder comment from fullsend-ai-coder[bot] not found in results")
	}
}
