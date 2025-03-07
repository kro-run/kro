// Copyright 2025 The Kube Resource Orchestrator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package graph

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResource_Dependencies(t *testing.T) {
	tests := []struct {
		name         string
		dependencies []string
		checkDep     string
		hasDep       bool
		addDeps      []string
		finalDeps    []string
	}{
		{
			name:         "empty dependencies",
			dependencies: []string{},
			checkDep:     "test",
			hasDep:       false,
			addDeps:      []string{"test1", "test2"},
			finalDeps:    []string{"test1", "test2"},
		},
		{
			name:         "existing dependency",
			dependencies: []string{"test1", "test2"},
			checkDep:     "test1",
			hasDep:       true,
			addDeps:      []string{"test3", "test1"}, // test1 is duplicate
			finalDeps:    []string{"test1", "test2", "test3"},
		},
		{
			name:         "multiple additions",
			dependencies: []string{"test1"},
			checkDep:     "test3",
			hasDep:       false,
			addDeps:      []string{"test2", "test3", "test4"},
			finalDeps:    []string{"test1", "test2", "test3", "test4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Resource{
				dependencies: tt.dependencies,
			}

			// Test HasDependency
			assert.Equal(t, tt.hasDep, r.HasDependency(tt.checkDep))

			// Test AddDependencies
			r.addDependencies(tt.addDeps...)

			// Verify final dependencies
			assert.ElementsMatch(t, tt.finalDeps, r.GetDependencies())
		})
	}
}

func TestGetDependencies(t *testing.T) {
	tests := []struct {
		name     string
		resource *Resource
		want     []string
	}{
		{
			name: "implicit dependencies only",
			resource: &Resource{
				dependencies: []string{"dep1", "dep2"},
			},
			want: []string{"dep1", "dep2"},
		},
		{
			name: "explicit dependencies only",
			resource: &Resource{
				explicitDependencies: []string{"dep3", "dep4"},
			},
			want: []string{"dep3", "dep4"},
		},
		{
			name: "both implicit and explicit dependencies",
			resource: &Resource{
				dependencies:         []string{"dep1", "dep2"},
				explicitDependencies: []string{"dep2", "dep3"}, // dep2 is duplicate
			},
			want: []string{"dep1", "dep2", "dep3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resource.GetDependencies()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasDependency(t *testing.T) {
	tests := []struct {
		name     string
		resource *Resource
		checkDep string
		want     bool
	}{
		{
			name: "has implicit dependency",
			resource: &Resource{
				dependencies: []string{"dep1", "dep2"},
			},
			checkDep: "dep1",
			want:     true,
		},
		{
			name: "has explicit dependency",
			resource: &Resource{
				explicitDependencies: []string{"dep3", "dep4"},
			},
			checkDep: "dep3",
			want:     true,
		},
		{
			name: "has dependency in both",
			resource: &Resource{
				dependencies:         []string{"dep1", "dep2"},
				explicitDependencies: []string{"dep2", "dep3"},
			},
			checkDep: "dep2",
			want:     true,
		},
		{
			name: "no such dependency",
			resource: &Resource{
				dependencies:         []string{"dep1", "dep2"},
				explicitDependencies: []string{"dep3", "dep4"},
			},
			checkDep: "dep5",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resource.HasDependency(tt.checkDep)
			if got != tt.want {
				t.Errorf("HasDependency() = %v, want %v", got, tt.want)
			}
		})
	}
}
