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

package validator

import (
	"fmt"
	"os"

	"github.com/kro-run/kro/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ResourceGraphDefinitionValidator struct{}

func (v *ResourceGraphDefinitionValidator) ValidateFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return v.ValidateBytes(data)
}

func (v *ResourceGraphDefinitionValidator) ValidateBytes(data []byte) ([]string, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(data, obj); err != nil {
		return nil, err
	}

	// Check if it's a ResourceGraphDefinition
	kind := obj.GetKind()
	if kind != "ResourceGraphDefinition" {
		return nil, fmt.Errorf("expected kind ResourceGraphDefinition, got %s", kind)
	}

	rgd := &v1alpha1.ResourceGraphDefinition{}
	if err := yaml.Unmarshal(data, rgd); err != nil {
		return nil, err
	}

	// Validation checks
	warnings := []string{}

	// Schema checks
	if rgd.Spec.Schema.APIVersion == "" {
		warnings = append(warnings, "schema.apiVersion is not specified")
	}
	if rgd.Spec.Schema.Kind == "" {
		warnings = append(warnings, "schema.kind is not specified")
	}

	// Resource checks
	if len(rgd.Spec.Resources) == 0 {
		warnings = append(warnings, "no resources defined in the ResourceGraphDefinition")
	}

	// Resource ID checks
	resourceIDs := make(map[string]bool)
	for i, res := range rgd.Spec.Resources {
		if res.ID == "" {
			warnings = append(warnings, fmt.Sprintf("resource %d does not have an ID", i))
			continue
		}

		if resourceIDs[res.ID] {
			warnings = append(warnings, fmt.Sprintf("duplicate resource ID found: %s", res.ID))
		}
		resourceIDs[res.ID] = true
	}
	return warnings, nil
}
