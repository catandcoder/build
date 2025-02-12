// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package queue

import (
	"time"

	"golang.org/x/build/internal/buildgo"
)

type BuildletPriority int

const (
	// PriorityUrgent is reserved for Go releases.
	PriorityUrgent BuildletPriority = iota
	PriorityInteractive
	PriorityAutomated
	PriorityBatch
)

// SchedItem is a specification of a requested buildlet in its
// exported fields, and internal scheduler state used while waiting
// for that buildlet.
//
// SchedItem is safe for copying.
type SchedItem struct {
	buildgo.BuilderRev // not set for gomote
	HostType           string
	IsRelease          bool
	IsGomote           bool
	IsTry              bool
	IsHelper           bool
	Repo               string
	Branch             string

	// CommitTime is the latest commit date of the relevant repos
	// that make up the work being tested. (For example, x/foo
	// being tested against master can have either x/foo commit
	// being newer, or master being newer).
	CommitTime  time.Time
	RequestTime time.Time
	User        string
}

// Priority returns the BuildletPriority for a SchedItem.
func (s *SchedItem) Priority() BuildletPriority {
	switch {
	case s.IsRelease:
		return PriorityUrgent
	case s.IsGomote:
		return PriorityInteractive
	case s.IsTry:
		return PriorityAutomated
	default:
		return PriorityBatch
	}
}

func (s *SchedItem) sortTime() time.Time {
	if s.IsGomote || s.IsTry || s.CommitTime.IsZero() {
		return s.RequestTime
	}
	return s.CommitTime
}

// Less returns a boolean value of whether SchedItem is more important
// than the provided SchedItem.
func (s *SchedItem) Less(other *SchedItem) bool {
	if s.Priority() != other.Priority() {
		return s.Priority() < other.Priority()
	}
	if s.Priority() == PriorityBatch {
		// Batch items are completed in LIFO.
		return s.sortTime().After(other.sortTime())
	}
	return other.sortTime().After(s.sortTime())
}
