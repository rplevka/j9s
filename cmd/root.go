// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of j9s

package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-colorable"
	"github.com/roman-plevka/j9s/internal/config"
	"github.com/roman-plevka/j9s/internal/view"
	"github.com/spf13/cobra"
)

const (
	appName      = "j9s"
	shortAppDesc = "A graphical CLI for your Jenkins instance management."
	longAppDesc  = "j9s is a CLI to view and manage your Jenkins instances."
)

var (
	version, commit, date = "dev", "dev", "N/A"
	j9sFlags              *config.Flags

	rootCmd = &cobra.Command{
		Use:   appName,
		Short: shortAppDesc,
		Long:  longAppDesc,
		RunE:  run,
	}

	out = colorable.NewColorableStdout()
)

func init() {
	rootCmd.AddCommand(versionCmd())
	initJ9sFlags()
}

// Execute root command.
func Execute() error {
	return rootCmd.Execute()
}

func run(*cobra.Command, []string) error {
	if err := config.InitLocs(); err != nil {
		return err
	}

	// Use default log file if not specified
	logFilePath := *j9sFlags.LogFile
	if logFilePath == "" {
		logFilePath = config.AppLogFile
	}

	logFile, err := os.OpenFile(
		logFilePath,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0600,
	)
	if err != nil {
		return fmt.Errorf("log file %q init failed: %w", logFilePath, err)
	}
	defer func() {
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	defer func() {
		if err := recover(); err != nil {
			slog.Error("Boom!! j9s init failed", "error", err)
			slog.Error("", "stack", string(debug.Stack()))
			fmt.Printf("Boom!! %v.\n", err)
		}
	}()

	slog.SetDefault(slog.New(tint.NewHandler(logFile, &tint.Options{
		Level:      parseLevel(*j9sFlags.LogLevel),
		TimeFormat: time.RFC3339,
	})))

	cfg, err := loadConfiguration()
	if err != nil {
		slog.Warn("Failed to load configuration", "error", err)
	}

	app := view.NewApp(cfg)
	if err := app.Init(displayVersion(version, commit), int(*j9sFlags.RefreshRate)); err != nil {
		return err
	}
	if err := app.Run(); err != nil {
		return err
	}

	return nil
}

func loadConfiguration() (*config.Config, error) {
	slog.Info("🚀 j9s starting up...")

	cfg := config.NewConfig()
	if err := cfg.Load(config.AppConfigFile); err != nil {
		return cfg, err
	}
	cfg.Override(j9sFlags)

	return cfg, nil
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func initJ9sFlags() {
	j9sFlags = config.NewFlags()
	rootCmd.Flags().Float32VarP(
		j9sFlags.RefreshRate,
		"refresh", "r",
		config.DefaultRefreshRate,
		"Specify the default refresh rate as a float (sec)",
	)
	rootCmd.Flags().StringVarP(
		j9sFlags.LogLevel,
		"logLevel", "l",
		config.DefaultLogLevel,
		"Specify a log level (error, warn, info, debug)",
	)
	rootCmd.Flags().StringVarP(
		j9sFlags.LogFile,
		"logFile", "",
		config.AppLogFile,
		"Specify the log file",
	)
	rootCmd.Flags().BoolVar(
		j9sFlags.Headless,
		"headless",
		false,
		"Turn j9s header off",
	)
	rootCmd.Flags().BoolVar(
		j9sFlags.Logoless,
		"logoless",
		false,
		"Turn j9s logo off",
	)
	rootCmd.Flags().StringVarP(
		j9sFlags.Command,
		"command", "c",
		config.DefaultCommand,
		"Overrides the default resource to load when the application launches",
	)
	rootCmd.Flags().BoolVar(
		j9sFlags.ReadOnly,
		"readonly",
		false,
		"Sets readOnly mode by overriding readOnly configuration setting",
	)
	rootCmd.Flags().StringVar(
		j9sFlags.Context,
		"context",
		"",
		"The name of the Jenkins context to use",
	)
}
