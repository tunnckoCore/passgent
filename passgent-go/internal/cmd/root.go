package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"passgent-go/internal/config"
	"passgent-go/internal/store"
)

var (
	GlobalConfig *config.Config
	StoreDir     string
)

var rootCmd = &cobra.Command{
	Use:   "passgent",
	Short: "Passgent is a minimalist, age-based secrets manager",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			// Allow setup and store new -g to proceed without config
			isStoreNew := cmd.Name() == "new" && cmd.HasParent() && cmd.Parent().Name() == "store"
			if cmd.Name() != "setup" && cmd.Name() != "help" && !isStoreNew {
				return fmt.Errorf("configuration not found. run 'passgent setup' first")
			}
		}
		GlobalConfig = cfg

		if cfg != nil {
			// Skip store resolution for certain commands if needed
			sd, err := store.ResolveStore(cfg, storeName)
			if err != nil {
				// Don't fail here for commands that don't need a store
				if cmd.Name() != "gen" && cmd.Name() != "id" && cmd.Name() != "store" && cmd.Name() != "config" && cmd.Name() != "ls" && cmd.Name() != "setup" {
					return err
				}
			}
			StoreDir = sd
		}

		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&storeName, "store", "S", "", "Store name to use")

	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(storeCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(genCmd)
	rootCmd.AddCommand(spectreCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(otpCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(idCmd)
	rootCmd.AddCommand(lsCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(configCmd)
}
