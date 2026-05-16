package cmd

import (
	"fmt"
	"os"

	"github.com/lazycommit/lazycommit/cmd/core"
	"github.com/lazycommit/lazycommit/cmd/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "lazycommit",
	Short: "A silent, always-on Go daemon that watches and auto-pushes Git repos",
	Long: `lazyCommit is a daemon that watches multiple local Git repositories,
automatically pushes unpushed commits, and auto-commits forgotten file changes.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.lazycommit/config.toml)")

	// Atomic Core Commands
	rootCmd.AddCommand(core.NewPushCmd())
	rootCmd.AddCommand(core.NewCommitCmd())
	rootCmd.AddCommand(core.NewWatchCmd())
	rootCmd.AddCommand(core.NewScanCmd())
	rootCmd.AddCommand(core.NewScanAllCmd())

	// Complex Service Commands
	rootCmd.AddCommand(service.NewStartCmd())
	rootCmd.AddCommand(service.NewStopCmd())
	rootCmd.AddCommand(service.NewStatusCmd())
	rootCmd.AddCommand(service.NewLogsCmd())
	rootCmd.AddCommand(service.NewScheduleCmd())
	rootCmd.AddCommand(service.NewDaemonCmd())
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(fmt.Sprintf("%s/.lazycommit", home))
		viper.SetConfigType("toml")
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
	}
}
