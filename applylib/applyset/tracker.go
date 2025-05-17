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

type ApplyableObject struct {
	Applyable

	// Lifecycle hints
	CreateOnly bool
	DeleteOnly bool
	UpdateOnly bool
	ReadOnly   bool

	marshalled []byte
}

type objectKeyType struct {
	schema.GroupVersionKind
	types.NamespacedName
}

type tracker struct {
	// objects is a list of objects we are applying.
	objects []ApplyableObject

	// objectMappings is a map of object key to object.
	objectMappings map[objectKeyType]bool
}

func (a *ApplyableObject) Json() []byte {
	return a.marshalled
}

func NewTracker() *tracker {
	return &tracker{
		objectMappings: make(map[objectKeyType]bool),
	}
}

func (t *tracker) Add(obj ApplyableObject) error {
	gvk := obj.GroupVersionKind()
	objectKey := objectKeyType{
		GroupVersionKind: gvk,
		NamespacedName: types.NamespacedName{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		},
	}

	// detect duplicates in the objects list
	if _, found := t.objectMappings[objectKey]; found {
		return fmt.Errorf("duplicate object %v", objectKey)
	}
	t.objectMappings[objectKey] = true

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
