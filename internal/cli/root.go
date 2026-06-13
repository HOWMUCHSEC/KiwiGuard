package cli

import (
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/howmuchsec/kiwiguard/internal/app"
	"github.com/howmuchsec/kiwiguard/internal/config"
	"github.com/spf13/cobra"
)

const version = "dev"

// NewRootCommand builds the KiwiGuard CLI root command.
func NewRootCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "kiwiguard",
		Short: "KiwiGuard protects OpenAI-compatible LLM traffic",
	}

	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Validate CLI wiring without starting runtime services")

	cmd.AddCommand(newModeCommand(app.ModeServe, "Run gateway, control API, GUI, and worker together", &dryRun))
	cmd.AddCommand(newModeCommand(app.ModeGateway, "Run the KiwiGuard data plane", &dryRun))
	cmd.AddCommand(newModeCommand(app.ModeControl, "Run the KiwiGuard control plane", &dryRun))
	cmd.AddCommand(newModeCommand(app.ModeWorker, "Run the KiwiGuard worker", &dryRun))
	cmd.AddCommand(newMigrateCommand(&dryRun))
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print KiwiGuard version",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "kiwiguard %s\n", version)
			return err
		},
	})

	return cmd
}

func newMigrateCommand(dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run PostgreSQL configuration migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if *dryRun {
				return nil
			}
			cfg, err := config.LoadFromEnv()
			if err != nil {
				return err
			}
			return configstore.RunMigrations(cfg.PostgresDSN)
		},
	}
}

func newModeCommand(mode app.Mode, short string, dryRun *bool) *cobra.Command {
	return &cobra.Command{
		Use:   string(mode),
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Config{}
			if !*dryRun {
				loaded, err := config.LoadFromEnv()
				if err != nil {
					return err
				}
				cfg = loaded
			}
			runner := app.NewRunner(app.RunnerOptions{
				Mode:    mode,
				Version: version,
				DryRun:  *dryRun,
				Config:  cfg,
			})
			return runner.Run(cmd.Context())
		},
	}
}
