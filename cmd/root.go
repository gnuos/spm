// Package cmd
package cmd

import (
	"log"
	"os"

	"spm/pkg/config"
	"spm/pkg/utils"

	"github.com/spf13/cobra"
)

var (
	showVersion bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   utils.RuntimeModuleName,
	Short: utils.RuntimeModuleName + " cli",
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			execVersionCmd(cmd, args)
			os.Exit(0)
		}

		_ = cmd.Usage()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Configure cobra completion
	rootCmd.CompletionOptions.HiddenDefaultCmd = false
	rootCmd.CompletionOptions.DisableDefaultCmd = false
	rootCmd.CompletionOptions.DisableDescriptions = false

	// Set global flags
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Print version and exit")
	rootCmd.PersistentFlags().StringVarP(&config.LogLevelFlag, "loglevel", "l", "", "Set log level")
	rootCmd.PersistentFlags().StringVarP(&config.WorkDirFlag, "workdir", "w", "", "The path to the work directory")
	rootCmd.PersistentFlags().StringVarP(&config.ProcfileFlag, "procfile", "p", "", "The path to the Procfile")

	// Register persistent function for all commands
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		execRootPersistentPreRun()
	}
}

func execRootPersistentPreRun() {
	utils.InitEnv()
}
