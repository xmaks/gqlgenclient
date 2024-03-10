package cmd

import (
	"syscall"

	"github.com/99designs/gqlgen/api"
	"github.com/spf13/cobra"
	"github.com/xmaks/gqlgenclient/config"
	"github.com/xmaks/gqlgenclient/plugin/clientgen"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use: "generate",

	RunE: func(cmd *cobra.Command, args []string) error {
		generateConfig := config.FromContext(cmd.Context())

		defer syscall.Unlink(generateConfig.Exec.Filename)

		genetareOptions := []api.Option{}
		if generateConfig.Client.IsDefined() {
			clientgenPlugin := clientgen.New(&generateConfig.ExtendedConfig)
			genetareOptions = append(genetareOptions, api.AddPlugin(clientgenPlugin))
		}

		return api.Generate(generateConfig.Config, genetareOptions...)
	},

	PreRunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := config.NewWithContext(cmd.Context())
		if err == nil {
			cmd.SetContext(ctx)
		}
		return err
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
