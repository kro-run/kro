// Copyright 2025 The Kube Resource Orchestrator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"slices"

	"k8s.io/kube-openapi/pkg/validation/spec"
)

// GetResourceTopLevelFieldNames returns the top level field names
// of the resource. It walks through the schema of the resource and
// retrieves the top level fields including spec, status, metadata,
// etc.
//
// It is up to the caller to sort filter the field names they want.
func GetResourceTopLevelFieldNames(schema *spec.Schema) []string {
	fieldNames := []string{}
	if schema == nil || schema.Properties == nil {
		return fieldNames
	}
	for fieldName := range schema.Properties {
		if fieldName != "apiVersion" && fieldName != "kind" {
			fieldNames = append(fieldNames, fieldName)
		}
	}

	slices.Sort(fieldNames)
	return fieldNames
}
