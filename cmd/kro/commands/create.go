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

package commands

import (
	"context"
	"fmt"
	"os"

	krov1alpha1 "github.com/kro-run/kro/api/v1alpha1"
	"github.com/kro-run/kro/pkg/controller/instance/delta"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	sigsYaml "sigs.k8s.io/yaml"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "create the ResourceGraphDefinition",
	Long: `Create the ResourceGraphDefinition. This command checks 
	Creates the ResourceGraphDefinition.`,
}

var dryRun bool

var CreateRGDCmd = &cobra.Command{
	// Add Functionality to create a RGD from ECR
	Use:   "rgd FILE",
	Short: "Creates RGD from file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		yamlBytes, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		c, err := getKubeClient()
		if err != nil {
			return err
		}

		var newRGD krov1alpha1.ResourceGraphDefinition
		if err := sigsYaml.Unmarshal(yamlBytes, &newRGD); err != nil {
			return fmt.Errorf("error parsing RGD: %w", err)
		}

		// Try to get old RGD if Exists
		oldRGD, exists, err := getRGDIfExists(c, newRGD.Name)
		if err != nil {
			return err
		}
		// Handle dry run first
		if dryRun {
			if exists {
				// Compare and show what would change
				if err != nil {
					return err
				}
				fmt.Printf("Would update ResourceGraphDefinition: %s\n", newRGD.Name)
				if err := showDeltaComparison(oldRGD, newRGD); err != nil {
					fmt.Printf("Error showing delta: %v\n", err)
				}
			} else {
				// Show what would be created
				fmt.Printf("Would create new ResourceGraphDefinition: %s\n", newRGD.Name)
			}
			return nil
		}
		// Create operation (not dry run)
		if exists {
			newRGD.ResourceVersion = oldRGD.ResourceVersion
			if err := c.Update(context.Background(), &newRGD); err != nil {
				return fmt.Errorf("failed to update RGD: %w", err)
			}
			fmt.Printf("Updated ResourceGraphDefinition: %s\n", newRGD.Name)
		} else {
			// Create new resource if old one does not exist
			if err := c.Create(context.Background(), &newRGD); err != nil {
				return fmt.Errorf("failed to create RGD: %w", err)
			}
			fmt.Printf("Created ResourceGraphDefinition: %s\n", newRGD.Name)
		}
		return nil
	},
}

func showDeltaComparison(oldRGD krov1alpha1.ResourceGraphDefinition, newRGD krov1alpha1.ResourceGraphDefinition) error {
	oldRGD.APIVersion = "kro.run/v1alpha1"
	oldRGD.Kind = "ResourceGraphDefinition"
	newRGD.APIVersion = "kro.run/v1alpha1"
	newRGD.Kind = "ResourceGraphDefinition"

	// Convert to unstructured
	oldUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&oldRGD)
	if err != nil {
		return fmt.Errorf("error converting old RGD: %w", err)
	}

	newUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newRGD)
	if err != nil {
		return fmt.Errorf("error converting new RGD: %w", err)
	}

	oldObj := &unstructured.Unstructured{Object: oldUnstructured}
	newObj := &unstructured.Unstructured{Object: newUnstructured}

	// Use the delta package to compare the objects
	differences, err := delta.Compare(newObj, oldObj)
	if err != nil {
		return fmt.Errorf("error comparing resources: %w", err)
	}

	// Print the differences in a readable format
	fmt.Println("Delta comparison results:")
	if len(differences) == 0 {
		fmt.Println("No differences found")
		return nil
	}

	for i, diff := range differences {
		fmt.Printf("%d. Path: %s\n", i+1, diff.Path)
		fmt.Printf("   - Old: %v\n", diff.Observed)
		fmt.Printf("   + New: %v\n", diff.Desired)
		fmt.Println()
	}

	return nil
}

func AddCreateCommands(rootCmd *cobra.Command) {
	CreateRGDCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the object without applying it")
	createCmd.AddCommand(CreateRGDCmd)
	rootCmd.AddCommand(createCmd)
}
