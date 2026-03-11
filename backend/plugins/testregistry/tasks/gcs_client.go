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
	client *storage.Client
	bkt    *storage.BucketHandle
}

// NewGCSBucketClient creates a new GCS client for accessing Openshift CI bucket
// Uses WithoutAuthentication for public bucket access
func NewGCSBucketClient(ctx context.Context) (*GCSBucket, errors.Error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, errors.Default.Wrap(err, "failed to create GCS client")
	}

	return &GCSBucket{
		client: client,
		bkt:    client.Bucket(OpenshiftCIBucketName),
	}, nil
}

// Close releases resources held by the GCS client
func (b *GCSBucket) Close() error {
	return b.client.Close()
}

// maxJUnitFilesPerJob limits the number of JUnit files collected per job to prevent excessive memory usage
const maxJUnitFilesPerJob = 50

// JUnitFile represents a single JUnit XML file fetched from GCS
type JUnitFile struct {
	Content []byte
	Path    string
}

// GetJobJunitContent retrieves all matching JUnit XML files from GCS for a specific job.
// It iterates through all objects in the artifact directory and returns every file
// matching the regex pattern, since jobs can produce multiple JUnit files across subdirectories.
//
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
//   - []JUnitFile: All matching JUnit files with their content and paths (may be partial on error)
//   - error: Non-nil if the GCS listing was interrupted (partial results may still be usable)
func (b *GCSBucket) GetJobJunitContent(ctx context.Context, orgName, repoName, pullNumber, jobId, jobType, jobName string, fileName *regexp.Regexp) ([]JUnitFile, error) {
	query := &storage.Query{}

	// Build GCS path prefix based on job type
	// For Openshift CI, the structure is:
	// - presubmit: pr-logs/pull/{org}_{repo}/{pullNumber}/{jobName}/{jobId}/artifacts
	// - postsubmit: logs/{jobName}/{jobId}/artifacts
	// - periodic: logs/{jobName}/{jobId}/artifacts
	// Reference: https://github.com/konflux-ci/quality-dashboard/blob/e846aa2dd9b3c1cad9ac4d16d18ddf677e3e6247/backend/api/server/prow_rotate.go#L64-L67
	switch jobType {
	case "presubmit":
		// Presubmit jobs: search the artifacts/ directory and its subdirectories
		// e.g., artifacts/junit_operator.xml, artifacts/step/artifacts/junit/*.xml
		query.Prefix = fmt.Sprintf("pr-logs/pull/%s_%s/%s/%s/%s/artifacts", orgName, repoName, pullNumber, jobName, jobId)
	case "postsubmit":
		// Postsubmit jobs: search artifacts/ directory and subdirectories
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	case "periodic":
		// Periodic jobs: search artifacts/ directory and subdirectories
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	default:
		// Unknown type, try postsubmit format as fallback
		query.Prefix = fmt.Sprintf("logs/%s/%s/artifacts", jobName, jobId)
	}

	var results []JUnitFile

	it := b.bkt.Objects(ctx, query)
	for {
		obj, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// Return partial results with the error so caller can log it
			return results, fmt.Errorf("GCS listing interrupted: %w", err)
		}

		// Check if file name matches the pattern
		if fileName != nil && fileName.MatchString(obj.Name) {
			content, err := b.GetContent(ctx, obj.Name)
			if err != nil {
				// Skip unreadable files — a single failure shouldn't block others
				continue
			}
			results = append(results, JUnitFile{Content: content, Path: obj.Name})
			if len(results) >= maxJUnitFilesPerJob {
				break
			}
		}
	}

	return results, nil
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

// maxJUnitFileSize limits individual JUnit XML file reads to 10 MB
const maxJUnitFileSize = 10 * 1024 * 1024

// GetContent retrieves the content of a file from GCS, limited to maxJUnitFileSize bytes
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

	content, err := io.ReadAll(io.LimitReader(gcsReader, maxJUnitFileSize))
	if err != nil {
		return nil, errors.Default.Wrap(err, "error reading GCS content")
	}

	return content, nil
}
