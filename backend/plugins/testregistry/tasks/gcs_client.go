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
	"context"
	"fmt"
	"io"
	"regexp"

	"cloud.google.com/go/storage"
	"github.com/apache/incubator-devlake/core/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const OpenshiftCIBucketName = "test-platform-results"

// GCSBucket wraps a Google Cloud Storage bucket client for accessing Openshift CI test results
type GCSBucket struct {
	bkt *storage.BucketHandle
}

// NewGCSBucketClient creates a new GCS client for accessing Openshift CI bucket
// Uses WithoutAuthentication for public bucket access
func NewGCSBucketClient(ctx context.Context) (*GCSBucket, errors.Error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create GCS client")
	}

	return &GCSBucket{
		bkt: client.Bucket(OpenshiftCIBucketName),
	}, nil
}

// GetJobJunitContent retrieves JUnit XML content from GCS for a specific job
// Based on the quality-dashboard implementation:
// https://github.com/konflux-ci/quality-dashboard/blob/main/backend/pkg/connectors/gcs/gcs_authentication.go
//
// Parameters:
//   - orgName: GitHub organization name (e.g., "redhat-appstudio")
//   - repoName: Repository name (e.g., "infra-deployments")
//   - pullNumber: Pull request number (for presubmit jobs) or empty string (for postsubmit/periodic)
//   - jobId: Prow job build ID
//   - jobType: Job type - "pull_request" (presubmit), "push" (postsubmit), or "periodic"
//   - jobName: Prow job name (e.g., "periodic-ci-redhat-appstudio-infra-deployments-main-appstudio-e2e-tests-periodic")
//   - fileName: Regex pattern to match the JUnit file name (e.g., regexp.MustCompile(".*junit.*\\.xml"))
//
// Returns:
//   - []byte: File content if found, nil otherwise
//   - string: Full GCS path of the matched file, empty string if not found
func (b *GCSBucket) GetJobJunitContent(orgName, repoName, pullNumber, jobId, jobType, jobName string, fileName *regexp.Regexp) ([]byte, string) {
	query := &storage.Query{}

	// Build GCS path prefix based on job type
	// For Openshift CI, the structure is:
	// - presubmit: pr-logs/pull/{org}_{repo}/{pullNumber}/{jobName}/{jobId}/artifacts
	// - postsubmit: logs/{jobName}/{jobId}
	// - periodic: logs/{jobName}/{jobId}
	// Reference: https://github.com/konflux-ci/quality-dashboard/blob/e846aa2dd9b3c1cad9ac4d16d18ddf677e3e6247/backend/api/server/prow_rotate.go#L64-L67
	switch jobType {
	case "presubmit":
		// Presubmit jobs need org, repo, and PR number in the path
		query.Prefix = fmt.Sprintf("pr-logs/pull/%s_%s/%s/%s/%s/artifacts", orgName, repoName, pullNumber, jobName, jobId)
	case "postsubmit":
		// Postsubmit jobs: logs/{jobName}/{jobId}
		query.Prefix = fmt.Sprintf("logs/%s/%s", jobName, jobId)
	case "periodic":
		// Periodic jobs: logs/{jobName}/{jobId}
		query.Prefix = fmt.Sprintf("logs/%s/%s", jobName, jobId)
	default:
		// Unknown type, try postsubmit format as fallback
		query.Prefix = fmt.Sprintf("logs/%s/%s", jobName, jobId)
	}

	it := b.bkt.Objects(context.Background(), query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			// No more objects
			break
		}
		if err != nil {
			// Error iterating, continue to next
			continue
		}

		// Check if file name matches the pattern
		if fileName != nil && fileName.MatchString(obj.Name) {
			if b.ContentExists(context.Background(), obj.Name) {
				content, err := b.GetContent(context.Background(), obj.Name)
				if err == nil {
					// Return content and file path
					// Note: We can only return one match, so we return the first one found
					return content, obj.Name
				}
			}
		}
	}

	return nil, ""
}

// ContentExists checks if a file exists at the given GCS path
func (b *GCSBucket) ContentExists(ctx context.Context, path string) bool {
	if len(path) == 0 {
		return false
	}

	// Get an Object handle for the path
	obj := b.bkt.Object(path)

	// If we can get the attrs then the object exists
	// Otherwise presume it doesn't
	_, err := obj.Attrs(ctx)
	return err == nil
}

// GetContent retrieves the content of a file from GCS
func (b *GCSBucket) GetContent(ctx context.Context, path string) ([]byte, errors.Error) {
	if len(path) == 0 {
		return nil, errors.BadInput.New("missing path to GCS content")
	}

	// Get an Object handle for the path
	obj := b.bkt.Object(path)

	// Use the object attributes to try to get the latest generation to avoid cached content
	objAttrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS attributes")
	}
	obj = obj.Generation(objAttrs.Generation)

	// Get an io.Reader for the object
	gcsReader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS content")
	}
	defer gcsReader.Close()

	content, err := io.ReadAll(gcsReader)
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS content")
	}

	return content, nil
}
