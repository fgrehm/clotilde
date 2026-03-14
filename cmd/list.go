package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/ui"
	"github.com/fgrehm/clotilde/internal/util"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all sessions",
	Long:    `List all clotilde sessions in the current project, sorted by last used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nCreate a session with:")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  clotilde start <session-name>")
			return nil
		}

		// Load all sessions
		store := session.NewFileStore(clotildeRoot)
		sessions, err := store.List()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nCreate a session with:")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  clotilde start <session-name>")
			return nil
		}

		// Always use static table - dashboard has interactive list
		return showStaticTable(cmd, sessions, store)
	},
}

// showInteractiveTable displays sessions in an interactive TUI table with sorting
// If a session is selected, it returns the session. Otherwise returns nil.
func showInteractiveTable(sessions []*session.Session, store session.Store) (*session.Session, error) {
	// Build headers
	headers := []string{"Name", "Model", "Type", "Last Used"}

	// Build rows (rows will be in same order as sessions array initially)
	var rows [][]string
	for _, sess := range sessions {
		model, lastUsed := extractModelAndLastUsed(sess, store)
		typeStr := formatSessionType(sess)
		rows = append(rows, []string{sess.Name, model, typeStr, util.FormatRelativeTime(lastUsed)})
	}

	// Create and run interactive table
	fmt.Printf("Sessions (%d total)\n\n", len(sessions))
	table := ui.NewTable(headers, rows).WithSorting()
	selectedRow, err := ui.RunTable(table)
	if err != nil {
		return nil, err
	}

	// If cancelled or no selection, return nil
	if len(selectedRow) == 0 {
		return nil, nil
	}

	// Map the selected row back to the session by name (first column)
	selectedName := selectedRow[0]
	for _, sess := range sessions {
		if sess.Name == selectedName {
			return sess, nil
		}
	}

	return nil, nil
}

// showStaticTable displays sessions in a static text table (for scripts/pipes)
func showStaticTable(cmd *cobra.Command, sessions []*session.Session, store session.Store) error {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Sessions (%d total):\n", len(sessions))

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("NAME", "MODEL", "TYPE", "LAST USED")

	for _, sess := range sessions {
		model, lastUsed := extractModelAndLastUsed(sess, store)
		typeStr := formatSessionType(sess)
		_ = table.Append(sess.Name, model, typeStr, util.FormatRelativeTime(lastUsed))
	}

	_ = table.Render()
	return nil
}

// extractModelAndLastUsed reads the transcript tail once, returning both the model
// family and the best "last used" time. More efficient than separate ExtractLastModel
// and LastTranscriptTime calls, which would each open and seek the file.
func extractModelAndLastUsed(sess *session.Session, store session.Store) (string, time.Time) {
	lastUsed := sess.Metadata.LastAccessed
	model := "-"

	if sess.Metadata.TranscriptPath != "" {
		m, ts := claude.ExtractModelAndLastTime(sess.Metadata.TranscriptPath)
		if m != "" {
			model = m
		}
		if ts.After(lastUsed) {
			lastUsed = ts
		}
	}

	// Fall back to requested model from settings (error is non-critical; no settings is common)
	if model == "-" {
		settings, _ := store.LoadSettings(sess.Name) //nolint:errcheck // missing settings file is expected
		if settings != nil && settings.Model != "" {
			model = settings.Model
		}
	}

	return model, lastUsed
}

// formatSessionType formats the session type string (regular, fork, incognito)
func formatSessionType(sess *session.Session) string {
	typeStr := "session"
	if sess.Metadata.IsForkedSession {
		typeStr = fmt.Sprintf("fork of %s", sess.Metadata.ParentSession)
	}
	if sess.Metadata.IsIncognito {
		typeStr += " 👻"
	}
	return typeStr
}
