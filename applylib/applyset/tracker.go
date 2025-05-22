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
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// ApplyableObject is implemented by objects that can be applied to the cluster.
// We don't need much, so this might allow for more efficient implementations in future.
type Applyable interface {
	// GroupVersionKind returns the GroupVersionKind structure describing the type of the object
	GroupVersionKind() schema.GroupVersionKind
	// GetNamespace returns the namespace of the object
	GetNamespace() string
	// GetName returns the name of the object
	GetName() string

	// GetLabels returns the labels of the object
	GetLabels() map[string]string
	// SetLabels sets the labels of the object
	SetLabels(labels map[string]string)

	// The object should implement json marshalling
	json.Marshaler
}
type ApplyableObject struct {
	Applyable

	// User provided unique identifier for the object.
	ID string

	// Lifecycle hints
	// TODO(barney-s): need to exapnd on these: https://github.com/kro-run/kro/issues/542
	ExternalRef bool
	Decorate    bool

	// lastReadRevision is the revision of the object that was last read from the cluster.
	lastReadRevision string

	// marshalled is the marshalled json of the object.
	marshalled []byte
}

type k8sObjectKey struct {
	schema.GroupVersionKind
	types.NamespacedName
}

type tracker struct {
	// objects is a list of objects we are applying.
	objects []ApplyableObject

	// serverIDs is a map of object key to object.
	serverIDs map[k8sObjectKey]bool

	// clientIDs is a map of object key to object.
	clientIDs map[string]bool
}

func (a *ApplyableObject) Json() []byte {
	return a.marshalled
}

func NewTracker() *tracker {
	return &tracker{
		serverIDs: make(map[k8sObjectKey]bool),
		clientIDs: make(map[string]bool),
	}
}

func (t *tracker) Add(obj ApplyableObject) error {
	gvk := obj.GroupVersionKind()

	// Server side uniqueness check
	objectKey := k8sObjectKey{
		GroupVersionKind: gvk,
		NamespacedName: types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	// detect duplicates in the objects list
	if _, found := t.serverIDs[objectKey]; found {
		return fmt.Errorf("duplicate object %v", objectKey)
	}
	t.serverIDs[objectKey] = true

	// client side uniqueness check
	if _, found := t.clientIDs[obj.ID]; found {
		return fmt.Errorf("duplicate objecti ID %v", obj.ID)
	}
	t.clientIDs[obj.ID] = true

	// Ensure the object is marshallable
	if j, err := json.Marshal(obj); err != nil {
		return fmt.Errorf("object %v is not json marshallable: %w", objectKey, err)
	} else {
		obj.marshalled = j
	}

	// Add the object to the tracker
	t.objects = append(t.objects, obj)
	return nil
}

func (t *tracker) Len() int {
	return len(t.objects)
}
