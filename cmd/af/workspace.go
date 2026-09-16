package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tobiasosborne/vibefeld/internal/cli"
	"github.com/tobiasosborne/vibefeld/internal/config"
	aferrors "github.com/tobiasosborne/vibefeld/internal/errors"
	"github.com/tobiasosborne/vibefeld/internal/ledger"
)

// newWorkspaceCmd creates the `af workspace` parent command.
func newWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace",
		GroupID: GroupAdmin,
		Short:   "Manage the workspace format",
		Long: `Manage the on-disk workspace format recorded in meta.json.

New workspaces are stamped with the current format at init time. An older
workspace (format 1.0) can be upgraded deliberately to a newer format once
every writer has stopped and the newer binary is installed.`,
	}

	cmd.AddCommand(newWorkspaceUpgradeCmd())
	return cmd
}

// newWorkspaceUpgradeCmd creates the `af workspace upgrade` subcommand.
func newWorkspaceUpgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the workspace format stamp",
		Long: `Upgrade a workspace's format stamp to a newer format.

The upgrade takes the ledger lock, copies ledger/*.json and meta.json into
backup/<UTC timestamp>/ inside the workspace, fsyncs the copies, re-reads the
ledger sequence, and writes the new stamp atomically. It is idempotent: if the
workspace is already at the target it prints a no-op and exits 0.

Downgrading is refused. To revert, restore the backup directory.

Stop every writer (provers, verifiers, scripts) before upgrading. The format
stamp names which event types the workspace may contain; a 1.0 workspace
refuses 1.1 event types until it is upgraded.

Examples:
  af workspace upgrade --to 1.1
  af workspace upgrade --to 1.1 --dry-run
  af workspace upgrade --to 1.1 -f json`,
		RunE: runWorkspaceUpgrade,
	}

	cmd.Flags().StringP("dir", "d", ".", "Proof directory path")
	cmd.Flags().String("to", "", "Target workspace format (required, e.g. 1.1)")
	cmd.Flags().StringP("format", "f", "text", "Output format (text or json)")

	// The upgrade implements a real preview; opt in so the global guard lets
	// --dry-run through instead of refusing it.
	markDryRunSupported(cmd)

	return cmd
}

// workspaceUpgradeResult is the structured outcome of an upgrade attempt.
type workspaceUpgradeResult struct {
	Current string `json:"current"`
	Target  string `json:"target"`
	Changed bool   `json:"changed"`
	DryRun  bool   `json:"dry_run"`
	Backup  string `json:"backup,omitempty"`
	Events  int    `json:"events,omitempty"`
}

// runWorkspaceUpgrade executes `af workspace upgrade`.
func runWorkspaceUpgrade(cmd *cobra.Command, args []string) error {
	dir := cli.MustString(cmd, "dir")
	target := strings.TrimSpace(cli.MustString(cmd, "to"))
	format := strings.ToLower(strings.TrimSpace(cli.MustString(cmd, "format")))
	dryRun := isDryRun(cmd)

	if format != "" && format != "text" && format != "json" {
		return fmt.Errorf("invalid format %q: must be 'text' or 'json'", format)
	}
	if target == "" {
		return fmt.Errorf("--to is required (e.g. --to %s)", config.FormatCurrent)
	}
	if !config.IsFormatReadable(target) {
		return aferrors.Newf(aferrors.INVALID_TARGET,
			"target workspace format %q is not readable by this af (reads %s)",
			target, strings.Join(config.FormatsReadable, ", "))
	}

	metaPath := filepath.Join(dir, "meta.json")
	cfg, err := config.Load(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return aferrors.Newf(aferrors.INVALID_STATE, "no workspace at %s (meta.json not found)", dir)
		}
		return fmt.Errorf("error reading workspace config: %w", err)
	}

	current := cfg.Version
	cmp := config.CompareFormats(target, current)
	if cmp < 0 {
		return aferrors.Newf(aferrors.INVALID_STATE,
			"refusing to downgrade workspace format from %s to %s; restore a backup instead",
			current, target)
	}

	// Already at the target: no-op, exit 0.
	if cmp == 0 {
		return outputWorkspaceUpgrade(cmd, format, workspaceUpgradeResult{
			Current: current, Target: target, Changed: false, DryRun: dryRun,
		})
	}

	backupPath := filepath.Join(dir, "backup", time.Now().UTC().Format("20060102T150405Z"))

	if dryRun {
		return outputWorkspaceUpgrade(cmd, format, workspaceUpgradeResult{
			Current: current, Target: target, Changed: true, DryRun: true, Backup: backupPath,
		})
	}

	// Serialise the upgrade against ledger writers.
	ledgerDir := filepath.Join(dir, "ledger")
	lock := ledger.NewLedgerLock(ledgerDir)
	if err := lock.Acquire("workspace-upgrade", cfg.LockTimeout); err != nil {
		return fmt.Errorf("error acquiring ledger lock: %w", err)
	}

	// The lock is released explicitly on the success path and via defer on any
	// error path, so an interrupted upgrade cannot strand it.
	released := false
	defer func() {
		if !released {
			_ = lock.Release()
		}
	}()

	// 1. Copy ledger/*.json and meta.json into the backup directory.
	events, err := backupWorkspace(dir, ledgerDir, metaPath, backupPath)
	if err != nil {
		return err
	}

	// 2. Re-read the sequence after the copy so the reported count is stable.
	if _, err := ledger.NextSequence(ledgerDir); err != nil {
		return fmt.Errorf("error reading ledger sequence: %w", err)
	}

	// 3. Persist the new stamp atomically.
	cfg.Version = target
	if err := config.Save(cfg, metaPath); err != nil {
		return fmt.Errorf("error writing workspace format: %w", err)
	}

	if err := lock.Release(); err != nil {
		return fmt.Errorf("error releasing ledger lock: %w", err)
	}
	released = true

	return outputWorkspaceUpgrade(cmd, format, workspaceUpgradeResult{
		Current: current, Target: target, Changed: true, Backup: backupPath, Events: events,
	})
}

// backupWorkspace copies the ledger event files and meta.json into
// backupPath/{ledger,meta.json} and fsyncs the copies and their directories.
// It returns the number of event files copied.
func backupWorkspace(dir, ledgerDir, metaPath, backupPath string) (int, error) {
	backupLedger := filepath.Join(backupPath, "ledger")
	if err := os.MkdirAll(backupLedger, 0755); err != nil {
		return 0, fmt.Errorf("error creating backup directory: %w", err)
	}

	entries, err := os.ReadDir(ledgerDir)
	if err != nil {
		return 0, fmt.Errorf("error reading ledger directory: %w", err)
	}

	copied := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		src := filepath.Join(ledgerDir, entry.Name())
		dst := filepath.Join(backupLedger, entry.Name())
		if err := copyFileSync(src, dst); err != nil {
			return copied, fmt.Errorf("error backing up %s: %w", entry.Name(), err)
		}
		copied++
	}

	if err := copyFileSync(metaPath, filepath.Join(backupPath, "meta.json")); err != nil {
		return copied, fmt.Errorf("error backing up meta.json: %w", err)
	}

	// fsync the contents and their directories so the backup survives a crash.
	for _, d := range []string{backupLedger, backupPath, filepath.Join(dir, "backup")} {
		if err := syncDirIfExists(d); err != nil {
			return copied, fmt.Errorf("error syncing backup directory: %w", err)
		}
	}

	return copied, nil
}

// copyFileSync copies src to dst and fsyncs the destination file.
func copyFileSync(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// syncDirIfExists fsyncs dir if it exists; a missing directory (e.g. an empty
// backup parent) is not an error.
func syncDirIfExists(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer d.Close()
	return d.Sync()
}

// outputWorkspaceUpgrade renders the result in text or JSON.
func outputWorkspaceUpgrade(cmd *cobra.Command, format string, res workspaceUpgradeResult) error {
	if format == "json" {
		data, err := json.Marshal(res)
		if err != nil {
			return fmt.Errorf("error marshaling JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	out := cmd.OutOrStdout()
	switch {
	case !res.Changed && res.DryRun:
		// A dry-run at the target is still a no-op.
		fmt.Fprintf(out, "Workspace already at format %s; nothing to do.\n", res.Target)
	case !res.Changed:
		fmt.Fprintf(out, "Workspace already at format %s; nothing to do.\n", res.Target)
	case res.DryRun:
		fmt.Fprintln(out, "[dry-run] Workspace upgrade preview (no changes written):")
		fmt.Fprintf(out, "  current format: %s\n", res.Current)
		fmt.Fprintf(out, "  target format:  %s\n", res.Target)
		fmt.Fprintf(out, "  backup:         %s\n", res.Backup)
		fmt.Fprintf(out, "  would copy ledger/*.json and meta.json, fsync them, then write the new stamp\n")
	default:
		fmt.Fprintf(out, "Upgraded workspace format from %s to %s.\n", res.Current, res.Target)
		fmt.Fprintf(out, "  backup: %s (%d event files)\n", res.Backup, res.Events)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newWorkspaceCmd())
}
