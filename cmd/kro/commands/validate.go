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
	"fmt"
	"os"
	"path/filepath"

	"github.com/kro-run/kro/cmd/kro/validator"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the ResourceGraphDefinition",
	Long: `Validate the ResourceGraphDefinition. This command checks ` +
		`if the ResourceGraphDefinition is valid and can be used to create a ResourceGraph.`,
}

var validateRGDCmd = &cobra.Command{
	Use:   "rgd [FILE]",
	Short: "Validate a ResourceGraphDefinition file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		config, err := rest.InClusterConfig()
		if err != nil {
			kubeconfigPath := cmd.Flag("kubeconfig").Value.String()
			if kubeconfigPath == "" {
				kubeconfigPath = os.Getenv("KUBECONFIG")
			}
			if kubeconfigPath == "" {
				home, err := os.UserHomeDir()
				if err == nil {
					kubeconfigPath = filepath.Join(home, ".kube", "config")
				}
			}

			config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
			if err != nil {
				return fmt.Errorf("failed to get kubernetes config: %w", err)
			}
		}

		validator, err := validator.NewResourceGraphDefinitionValidator(config)
		if err != nil {
			return fmt.Errorf("failed to create validator: %w", err)
		}

		warnings, err := validator.ValidateFile(filePath)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if len(warnings) > 0 {
			fmt.Println("Validation completed with warnings:")
			for _, warning := range warnings {
				fmt.Printf("- %s\n", warning)
			}
			return nil
		}
		fmt.Println("Validation successful! The ResourceGraphDefinition is valid.")
		return nil
	},
}

func AddValidateCommands(rootCmd *cobra.Command) {
	validateCmd.AddCommand(validateRGDCmd)
	rootCmd.AddCommand(validateCmd)
}
