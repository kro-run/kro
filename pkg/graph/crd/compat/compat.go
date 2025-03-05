// Copyright 2025 The Kube Resource Orchestrator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package compat

import (
	"fmt"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// DiffSchema compares schema versions and returns compatibility details.
// This function expects exactly one version in each slice. If more versions are present,
// it will return an error as multi-version support is not implemented.
func DiffSchema(oldVersions []v1.CustomResourceDefinitionVersion, newVersions []v1.CustomResourceDefinitionVersion) (*DiffResult, error) {
	// Validate single version assumption
	if len(oldVersions) != 1 || len(newVersions) != 1 {
		return nil, fmt.Errorf("expected exactly one version in each CRD, got %d old and %d new versions",
			len(oldVersions), len(newVersions))
	}

	oldVersion := oldVersions[0]
	newVersion := newVersions[0]

	// Check version names match
	if oldVersion.Name != newVersion.Name {
		return &DiffResult{
			BreakingChanges: []Change{
				{
					Path:       "version",
					ChangeType: TypeChanged,
					OldValue:   oldVersion.Name,
					NewValue:   newVersion.Name,
				},
			},
		}, nil
	}

	// Verify schemas exist
	if oldVersion.Schema == nil || oldVersion.Schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("old version %s has no schema", oldVersion.Name)
	}

	if newVersion.Schema == nil || newVersion.Schema.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("new version %s has no schema", newVersion.Name)
	}

	// Compare schemas
	return compareSchemas(
		fmt.Sprintf("version.%s.schema", oldVersion.Name),
		oldVersion.Schema.OpenAPIV3Schema,
		newVersion.Schema.OpenAPIV3Schema,
	)
}
