// Package cmd
package cmd

import (
	"fmt"
	"log"
	"os"

	"spm/pkg/config"
	"spm/pkg/utils"
	"spm/pkg/utils/constants"

	"github.com/spf13/cobra"
)

var (
	cwd             string
	showVersion     bool
	defaultProcfile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:           utils.RuntimeModuleName,
	Short:         utils.RuntimeModuleName + " cli",
	SilenceErrors: true,
	SilenceUsage:  true,
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
	var err error

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cwd, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	defaultProcfile = fmt.Sprintf("%s/Procfile", cwd)

	// Configure cobra
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Set global flags
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "Print version and exit")
	rootCmd.PersistentFlags().StringVarP(&config.LogLevelFlag, "loglevel", "l", constants.DefaultLogLevel, "Set log Level")
	rootCmd.PersistentFlags().StringVarP(&config.WorkDirFlag, "workdir", "w", cwd, "The path to the work directory")
	rootCmd.PersistentFlags().StringVarP(&config.ProcfileFlag, "procfile", "p", defaultProcfile, "The path to the Procfile")

	// Register persistent function for all commands
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		execRootPersistentPreRun()
	}
}

func execRootPersistentPreRun() {
	utils.InitEnv()
}
