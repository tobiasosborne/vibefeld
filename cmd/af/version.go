// Package main contains the af version command for displaying build information.
package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
)

// Version information. VersionInfo is the single source of truth for af's
// version — it backs both `af --version` (rootCmd.Version, main.go) and
// `af version --json`. Its default below is baked into source and MUST be
// bumped alongside the changelog (see TestVersionInfo_MatchesLatestChangelogEntry)
// so that even a plain unstamped `go build ./cmd/af` reports a real, current
// version — never the old "dev" placeholder, which left rk's `rk doctor`
// (the D6 stale-binary detector) unable to parse a version at all.
//
// GitCommit/BuildDate are ldflags-only (no source default makes sense for
// them); scripts/build.sh stamps all three at build/install time:
//
//	go build -ldflags "-X main.VersionInfo=0.1.8 -X main.GitCommit=$(git rev-parse --short HEAD) -X main.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	VersionInfo = "0.1.8"
	GitCommit   = "unknown"
	BuildDate   = "unknown"
)

// versionJSON represents the JSON output structure for the version command.
// format is the newest workspace format this binary reads (config.FormatCurrent);
// policy is the acceptance/claim policy number, which changes independently.
type versionJSON struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Format    string `json:"format"`
	Policy    string `json:"policy"`
}

// newVersionCmd creates the version command for displaying build information.
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		GroupID: GroupUtil,
		Short:   "Display version and build information",
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
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json; same as --json)")

	return cmd
}

// runVersion executes the version command.
func runVersion(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	format, _ := cmd.Flags().GetString("format")
	format = strings.ToLower(strings.TrimSpace(format))

	switch format {
	case "", "text":
		// fall through to the --json check below
	case "json":
		jsonOutput = true
	default:
		return aferrors.Newf(aferrors.INVALID_TYPE, "invalid format %q: must be 'text' or 'json'", format)
	}

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
		Format:    config.FormatCurrent,
		Policy:    config.PolicyVersion,
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
	fmt.Fprintf(out, "  Format:  %s\n", config.FormatCurrent)
	fmt.Fprintf(out, "  Policy:  %s\n", config.PolicyVersion)

	return nil
}

func init() {
	rootCmd.AddCommand(newVersionCmd())
}
