//go:build ignore

// TODO: reimplement using file-based store

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/package-register/mocode/internal/config"
	"github.com/package-register/mocode/internal/db"
	"github.com/package-register/mocode/internal/session"
	"github.com/spf13/cobra"
)

// sessionSnapshotServices holds the services needed for snapshot operations.
type sessionSnapshotServices struct {
	sessions  session.Service
	snapshots *session.SnapshotService
	cfg       *config.ConfigStore
}

func snapshotSetup(cmd *cobra.Command) (context.Context, *sessionSnapshotServices, func(), error) {
	dataDir, _ := cmd.Flags().GetString("data-dir")
	ctx := cmd.Context()

	cfg, err := config.Init("", dataDir, false)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize config: %w", err)
	}
	if dataDir == "" {
		dataDir = cfg.Config().Options.DataDirectory
	}

	conn, err := db.Connect(ctx, dataDir)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := db.New(conn)
	svc := &sessionSnapshotServices{
		sessions:  session.NewService(queries, conn),
		snapshots: session.NewSnapshotService(conn, queries),
		cfg:       cfg,
	}
	return ctx, svc, func() { conn.Close() }, nil
}

var (
	sessionLogJSON    bool
	sessionBranchJSON bool
	sessionRevertJSON bool
)

// mocode session log <session-id>
var sessionLogCmd = &cobra.Command{
	Use:   "log <session-id>",
	Short: "Show snapshot history for a session",
	Long:  "Show snapshot/checkpoint history for a session. Use --json for machine-readable output.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionLog,
}

// mocode session branch <session-id> --snapshot <snapshot-id> --title "Branch name"
var sessionBranchCmd = &cobra.Command{
	Use:   "branch <session-id>",
	Short: "Branch a session from a snapshot point",
	Long:  "Create a new session that forks from an existing session at a specific snapshot.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionBranch,
}

// mocode session revert <session-id> --to <snapshot-id>
var sessionRevertCmd = &cobra.Command{
	Use:   "revert <session-id>",
	Short: "Revert session files to a previous snapshot",
	Long:  "Restore file states to a previous checkpoint. Current state is auto-snapshotted before revert.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionRevert,
}

func init() {
	sessionLogCmd.Flags().BoolVar(&sessionLogJSON, "json", false, "output in JSON format")
	sessionBranchCmd.Flags().String("snapshot", "", "snapshot ID to branch from (required)")
	sessionBranchCmd.Flags().String("title", "", "title for the branch session")
	sessionBranchCmd.Flags().BoolVar(&sessionBranchJSON, "json", false, "output in JSON format")
	sessionRevertCmd.Flags().String("to", "", "target snapshot ID to revert to (required)")
	sessionRevertCmd.Flags().BoolVar(&sessionRevertJSON, "json", false, "output in JSON format")

	// Register subcommands under existing session command
	sessionCmd.AddCommand(sessionLogCmd)
	sessionCmd.AddCommand(sessionBranchCmd)
	sessionCmd.AddCommand(sessionRevertCmd)
}

func runSessionLog(cmd *cobra.Command, args []string) error {
	ctx, svc, cleanup, err := snapshotSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	snapshots, err := svc.snapshots.ListSnapshots(ctx, sess.ID, 50, "")
	if err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	if sessionLogJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(snapshots)
	}

	if len(snapshots) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No snapshots for this session.")
		return nil
	}

	w, cleanupPager, _ := sessionWriter(ctx, len(snapshots)+5)
	defer cleanupPager()

	hashStyle := lipgloss.NewStyle().Foreground(charmtone.Malibu)
	descStyle := lipgloss.NewStyle().Foreground(charmtone.Pepper)
	dateStyle := lipgloss.NewStyle().Foreground(charmtone.Damson)

	width := sessionOutputWidth
	if tw, _, err := term.GetSize(os.Stdout.Fd()); err == nil && tw > 0 {
		width = tw
	}

	fmt.Fprintf(w, "Snapshot history for session %s (%s):\n\n",
		session.HashID(sess.ID)[:7], sess.Title)

	for i, snap := range snapshots {
		hash := snap.ID[:8]
		date := time.UnixMilli(snap.TimeCreated).Format(time.RFC3339)
		desc := strings.ReplaceAll(snap.Description, "\n", " ")
		desc = ansi.Truncate(desc, width-50, "…")

		marker := "  "
		if i == 0 {
			marker = "→ " // Latest
		}

		line := fmt.Sprintf("%s%s %s %s", marker, hashStyle.Render(hash), dateStyle.Render(date), descStyle.Render(desc))
		fmt.Fprintln(w, line)

		if snap.ParentSnapshotID != "" && i < len(snapshots)-1 {
			fmt.Fprintf(w, "  %s parent: %s\n",
				strings.Repeat(" ", 9),
				hashStyle.Render(snap.ParentSnapshotID[:8]))
		}
	}

	return nil
}

func runSessionBranch(cmd *cobra.Command, args []string) error {
	snapshotID, _ := cmd.Flags().GetString("snapshot")
	if snapshotID == "" {
		return fmt.Errorf("--snapshot flag is required")
	}
	title, _ := cmd.Flags().GetString("title")
	if title == "" {
		title = "Branch from " + args[0]
	}

	ctx, svc, cleanup, err := snapshotSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	newSession, err := svc.snapshots.Branch(ctx, svc.sessions, sess.ID, snapshotID, title)
	if err != nil {
		return fmt.Errorf("failed to branch session: %w", err)
	}

	if sessionBranchJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(map[string]interface{}{
			"session_id": newSession.ID,
			"title":      newSession.Title,
			"parent_id":  sess.ID,
		})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created branch session %s %q (parent: %s)\n",
		session.HashID(newSession.ID)[:12],
		newSession.Title,
		session.HashID(sess.ID)[:7])
	return nil
}

func runSessionRevert(cmd *cobra.Command, args []string) error {
	targetSnapshotID, _ := cmd.Flags().GetString("to")
	if targetSnapshotID == "" {
		return fmt.Errorf("--to flag is required (target snapshot ID)")
	}

	ctx, svc, cleanup, err := snapshotSetup(cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	sess, err := resolveSessionID(ctx, svc.sessions, args[0])
	if err != nil {
		return err
	}

	// Show preview
	snap, err := svc.snapshots.GetSnapshot(ctx, targetSnapshotID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Reverting session %s to snapshot %s (%s)\n",
		session.HashID(sess.ID)[:7],
		targetSnapshotID[:8],
		snap.Description)
	fmt.Fprintf(cmd.ErrOrStderr(), "Files in snapshot: %d\n", len(snap.FileStates))

	// Execute revert
	diffs, err := svc.snapshots.RevertTo(ctx, sess.ID, targetSnapshotID)
	if err != nil {
		return fmt.Errorf("revert failed: %w", err)
	}

	if sessionRevertJSON {
		type revertResult struct {
			FilesChanged int                `json:"files_changed"`
			Diffs        []session.FileDiff `json:"diffs"`
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetEscapeHTML(false)
		return enc.Encode(revertResult{
			FilesChanged: len(diffs),
			Diffs:        diffs,
		})
	}

	if len(diffs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No files changed (already at target state).")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nReverted %d file(s):\n\n", len(diffs))
	for _, d := range diffs {
		status := "M"
		if d.BeforeVersion == 0 {
			status = "A"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s (+%d/-%d) v%d→v%d\n",
			status, d.Path, d.Additions, d.Deletions, d.BeforeVersion, d.AfterVersion)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Auto-snapshot created before revert. Use `mocode session log` to see it.")

	return nil
}
