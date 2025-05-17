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

type PrunedObject struct {
	PruneObject
	Error error
}

type ApplyResult struct {
	Desired        int
	AppliedObjects []AppliedObject
	PrunedObjects  []PrunedObject
}

func (a *ApplyResult) Success() bool {
	if len(a.AppliedObjects) != a.Desired {
		return false
	}
	for _, applied := range a.AppliedObjects {
		if applied.Error != nil {
			return false
		}
	}
	return true
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

func (a *ApplyResult) recordApplied(obj ApplyableObject, lastApplied *unstructured.Unstructured, err error) {
	a.AppliedObjects = append(a.AppliedObjects, AppliedObject{
		ApplyableObject: obj,
		LastApplied:     lastApplied,
		Error:           err,
	})
}

func (a *ApplyResult) recordPrune(obj PruneObject, err error) {
	a.PrunedObjects = append(a.PrunedObjects, PrunedObject{
		PruneObject: obj,
		Error:       err,
	})
}
