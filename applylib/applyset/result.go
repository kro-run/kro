// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package applyset

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type AppliedObject struct {
	ApplyableObject
	LastApplied *unstructured.Unstructured
	Error       error
	Message     string
}

func (ao *AppliedObject) HasClusterMutation() bool {
	// If there was an error applying, we consider the object to have not changed.
	if ao.Error != nil {
		return false
	}

	// If the object was not applied, we consider it to have not changed.
	if ao.LastApplied == nil {
		return false
	}

	return ao.lastReadRevision != ao.LastApplied.GetResourceVersion()
}

type PrunedObject struct {
	PruneObject
	Error error
}

type ApplyResult struct {
	Desired        int
	AppliedObjects []AppliedObject
	PrunedObjects  []PrunedObject
}

func (a *ApplyResult) PruneErrors() error {
	errorsSeen := []error{}
	for _, pruned := range a.PrunedObjects {
		if pruned.Error != nil {
			errorsSeen = append(errorsSeen, pruned.Error)
		}
	}
	return errors.Join(errorsSeen...)
}

func (a *ApplyResult) ApplyErrors() error {
	errorsSeen := []error{}
	if len(a.AppliedObjects) != a.Desired {
		errorsSeen = append(errorsSeen, fmt.Errorf("expected %d applied objects, got %d", a.Desired, len(a.AppliedObjects)))
	}
	for _, applied := range a.AppliedObjects {
		if applied.Error != nil {
			errorsSeen = append(errorsSeen, applied.Error)
		}
	}
	return errors.Join(errorsSeen...)
}

func (a *ApplyResult) AppliedUIDs() sets.Set[types.UID] {
	uids := sets.New[types.UID]()
	for _, applied := range a.AppliedObjects {
		if applied.Error != nil {
			continue
		}
		uids.Insert(applied.LastApplied.GetUID())
	}
	return uids
}

func (a *ApplyResult) HasClusterMutation() bool {
	for _, applied := range a.AppliedObjects {
		if applied.HasClusterMutation() {
			return true
		}
	}
	return false
}

func (a *ApplyResult) recordApplied(
	obj ApplyableObject,
	lastApplied *unstructured.Unstructured,
	err error,
) AppliedObject {
	ao := AppliedObject{
		ApplyableObject: obj,
		LastApplied:     lastApplied,
		Error:           err,
	}
	a.AppliedObjects = append(a.AppliedObjects, ao)
	return ao
}
