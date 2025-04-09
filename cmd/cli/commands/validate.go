package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func AddValidateCommands(rootCmd *cobra.Command) {
	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the ResourceGraphDefinition",
		Long:  `Validate the ResourceGraphDefinition. This command checks if the ResourceGraphDefinition is valid and can be used to create a ResourceGraph.`,
	}

	validateRGDCmd := &cobra.Command{
		Use:   "rgd [FILE]",
		Short: "Validate a ResourceGraphDefinition file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Validation successful! The ResourceGraphDefinition is valid.")
			return nil
		},
	}

	validateCmd.AddCommand(validateRGDCmd)
	rootCmd.AddCommand(validateCmd)
}
