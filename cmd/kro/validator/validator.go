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
	"github.com/kro-run/kro/pkg/graph"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

type ResourceGraphDefinitionValidator struct {
	builder *graph.Builder
}

func NewResourceGraphDefinitionValidator(config *rest.Config) (*ResourceGraphDefinitionValidator, error) {
	builder, err := graph.NewBuilder(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph builder: %w", err)
	}

	return &ResourceGraphDefinitionValidator{
		builder: builder,
	}, nil
}

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

	// Deep validation checks using graph builder
	_, err := v.builder.NewResourceGraphDefinition(rgd)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("deep validation failed: %v", err))
	}

	return warnings, nil
}
