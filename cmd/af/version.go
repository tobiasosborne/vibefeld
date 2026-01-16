// Package main contains the af version command for displaying build information.
package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information, set at build time via ldflags.
// Example build command:
//
//	go build -ldflags "-X main.VersionInfo=1.0.0 -X main.GitCommit=$(git rev-parse --short HEAD) -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	VersionInfo = "dev"
	GitCommit   = "unknown"
	BuildDate   = "unknown"
)

// versionJSON represents the JSON output structure for the version command.
type versionJSON struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
}

// newVersionCmd creates the version command for displaying build information.
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display version and build information",
		Long: `Display version and build information for the af CLI tool.

Shows:
  - Version string (set at build time)
  - Git commit hash (short form)
  - Build date
  - Go version used to build

The version information is set at build time via ldflags. When running
a development build, default values are shown.

Examples:
  af version           Show version in human-readable format
  af version --json    Show version in JSON format for scripting`,
		RunE: runVersion,
	}

	cmd.Flags().Bool("json", false, "Output version information in JSON format")

	return cmd
}

// runVersion executes the version command.
func runVersion(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	goVersion := runtime.Version()

	if jsonOutput {
		return outputVersionJSON(cmd, goVersion)
	}

	return outputVersionText(cmd, goVersion)
}

// outputVersionJSON outputs version information in JSON format.
func outputVersionJSON(cmd *cobra.Command, goVersion string) error {
	output := versionJSON{
		Version:   VersionInfo,
		Commit:    GitCommit,
		BuildDate: BuildDate,
		GoVersion: goVersion,
	}

	data, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}

// outputVersionText outputs version information in human-readable text format.
func outputVersionText(cmd *cobra.Command, goVersion string) error {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "af version %s\n", VersionInfo)
	fmt.Fprintf(out, "  Commit:  %s\n", GitCommit)
	fmt.Fprintf(out, "  Built:   %s\n", BuildDate)
	fmt.Fprintf(out, "  Go:      %s\n", goVersion)

	return nil
}

func init() {
	rootCmd.AddCommand(newVersionCmd())
}
