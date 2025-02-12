// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sign

import (
	"context"

	"golang.org/x/build/internal/relui/protos"
)

// Service is an interface for a release artifact signing service.
//
// Each call blocks until either the request has been acknowledged or the passed in context has been canceled.
// Setting a timeout on the context is recommended.
type Service interface {
	// SignArtifact creates a request to sign a release artifact.
	// The object URI must be URIs for file(s) on the service private GCS.
	SignArtifact(ctx context.Context, bt BuildType, objectURI []string) (jobID string, _ error)
	// ArtifactSigningStatus requests the status of an existing signing request message.
	// If the message is at the status of completed then the objectURI will be populated with the URIs for signed files in GCS.
	ArtifactSigningStatus(ctx context.Context, jobID string) (status Status, objectURI []string, err error)
	// CancelSigning marks a previous signing request as no longer needed,
	// possibly allowing resources to be freed sooner than otherwise.
	CancelSigning(ctx context.Context, jobID string) error
}

// Status of the signing request.
type Status int

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusFailed
	StatusCompleted
	StatusNotFound
)

// String is the string representation for the signing request status.
func (bs Status) String() string {
	switch bs {
	case StatusRunning:
		return "Running"
	case StatusFailed:
		return "Failed"
	case StatusCompleted:
		return "Completed"
	case StatusNotFound:
		return "NotFound"
	}
	return "Unknown"
}

// BuildType is the type of build the signing request is for.
type BuildType int

const (
	BuildUnspecified BuildType = iota
	BuildMacOS
	BuildWindows
	BuildGPG
)

// proto is the corresponding protobuf definition for the signing request build type.
func (bt BuildType) proto() protos.SignArtifactRequest_BuildType {
	switch bt {
	case BuildUnspecified:
		return protos.SignArtifactRequest_BUILD_TYPE_UNSPECIFIED
	case BuildMacOS:
		return protos.SignArtifactRequest_BUILD_TYPE_MACOS
	case BuildWindows:
		return protos.SignArtifactRequest_BUILD_TYPE_WINDOWS
	case BuildGPG:
		return protos.SignArtifactRequest_BUILD_TYPE_GPG
	}
	return protos.SignArtifactRequest_BUILD_TYPE_UNSPECIFIED
}
