package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/outputstyle"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

var inspectCmd = &cobra.Command{
	Use:     "inspect <name>",
	Aliases: []string{"show", "info"},
	Short:   "Show detailed information about a session",
	Long: `Display detailed information about a session including metadata,
files present, settings, context sources, and Claude Code data status.`,
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: sessionNameCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Find clotilde root
		clotildeRoot, err := config.FindClotildeRoot()
		if err != nil {
			return fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
		}

		// Create store
		store := session.NewFileStore(clotildeRoot)

		// Load session
		sess, err := store.Get(name)
		if err != nil {
			return fmt.Errorf("session '%s' not found", name)
		}

		sessionDir := config.GetSessionDir(clotildeRoot, name)

		// Print metadata
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", sess.Name)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "UUID: %s\n", sess.Metadata.SessionID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", sess.Metadata.Created.Format(time.RFC3339))
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last Accessed: %s\n", sess.Metadata.LastAccessed.Format(time.RFC3339))

		// Try to extract last model from transcript
		if sess.Metadata.TranscriptPath != "" {
			if lastModel := claude.ExtractLastModel(sess.Metadata.TranscriptPath); lastModel != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Last Model Used: %s\n", lastModel)
			}
		}

		if sess.Metadata.IsForkedSession {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Forked from: %s\n", sess.Metadata.ParentSession)
		}

		// Show previous session IDs (from /clear operations, and defensively from /compact)
		if len(sess.Metadata.PreviousSessionIDs) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Previous UUIDs: %d\n", len(sess.Metadata.PreviousSessionIDs))
			for i, prevID := range sess.Metadata.PreviousSessionIDs {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, prevID)
			}
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout())

		// Show files present
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Files:")
		files := []string{"metadata.json", "settings.json", "system-prompt.md"}
		for _, file := range files {
			path := filepath.Join(sessionDir, file)
			if util.FileExists(path) {
				info, err := os.Stat(path)
				if err == nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s (%s)\n", file, util.FormatSize(info.Size()))
				}
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", file)
			}
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout())

		// Show settings summary
		settings, err := store.LoadSettings(name)
		if err == nil && settings != nil {
			hasSettings := settings.Model != "" ||
				settings.OutputStyle != "" ||
				len(settings.Permissions.Allow) > 0 ||
				len(settings.Permissions.Deny) > 0

			if hasSettings {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Settings:")
				if settings.Model != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Model: %s\n", settings.Model)
				}
				if settings.OutputStyle != "" {
					if outputstyle.IsBuiltIn(settings.OutputStyle) {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Output Style: %s (built-in)\n", settings.OutputStyle)
					} else {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Output Style: %s (custom)\n", settings.OutputStyle)
					}
				}
				if len(settings.Permissions.Allow) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Allowed tools: %d\n", len(settings.Permissions.Allow))
				}
				if len(settings.Permissions.Deny) > 0 {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Denied tools: %d\n", len(settings.Permissions.Deny))
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
		}

		// Show system prompt
		systemPromptPath := filepath.Join(sessionDir, "system-prompt.md")
		if util.FileExists(systemPromptPath) {
			content, err := os.ReadFile(systemPromptPath)
			if err == nil && len(content) > 0 {
				mode := sess.Metadata.GetSystemPromptMode()
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "System Prompt:")
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Mode: %s\n", mode)
				excerpt := util.TruncateText(string(content), 200)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Content: %s\n", excerpt)
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
		}

		// Show context sources
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Context:")

		globalContext := filepath.Join(clotildeRoot, config.GlobalContextFile)
		if util.FileExists(globalContext) {
			content, err := os.ReadFile(globalContext)
			if err == nil && len(content) > 0 {
				lines, _ := util.CountLines(globalContext)
				excerpt := util.TruncateText(string(content), 200)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  (%d lines): %s\n", lines, excerpt)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  empty")
			}
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  not set")
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout())

		// Show Claude Code data status
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Claude Code Data:")

		// Use stored transcript path if available, otherwise compute it
		transcriptPath := sess.Metadata.TranscriptPath
		if transcriptPath == "" {
			// Fall back to computing the path
			homeDir, err := util.HomeDir()
			if err == nil {
				projectDir := claude.ProjectDir(clotildeRoot)
				claudeProjectDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
				transcriptPath = filepath.Join(claudeProjectDir, sess.Metadata.SessionID+".jsonl")
			}
		}

		if transcriptPath != "" && util.FileExists(transcriptPath) {
			info, err := os.Stat(transcriptPath)
			if err == nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  Transcript: %s\n", util.FormatSize(info.Size()))
			}
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  Transcript: not found")
		}

		return nil
	},
}
