package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/server"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/tour"
	"github.com/fgrehm/clotilde/internal/util"
)

// tourNameRe validates tour names: lowercase letters, digits, hyphens; starts/ends with alnum.
var tourNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

func newTourCmd() *cobra.Command {
	tourCmd := &cobra.Command{
		Use:   "tour",
		Short: "Interactive codebase tours via browser",
		Long:  "Browse codebase tours with syntax-highlighted code, step descriptions, and an AI chat sidebar.",
	}

	tourCmd.AddCommand(newTourListCmd())
	tourCmd.AddCommand(newTourServeCmd())
	tourCmd.AddCommand(newTourGenerateCmd())

	return tourCmd
}

func newTourListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available tours",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			toursDir := filepath.Join(dir, ".tours")

			tours, err := tour.LoadFromDir(toursDir)
			if err != nil {
				return fmt.Errorf("failed to load tours: %w", err)
			}

			if len(tours) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tours found.")
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCreate a tour file in %s/\n", toursDir)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tours (%d):\n", len(tours))
			names := make([]string, 0, len(tours))
			for name := range tours {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				t := tours[name]
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s (%d steps)\n", name, t.Title, len(t.Steps))
			}
			return nil
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().String("dir", cwd, "Directory containing .tours/ folder")

	return cmd
}

func newTourServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the tour web server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			port, _ := cmd.Flags().GetInt("port")
			model, _ := cmd.Flags().GetString("model")

			// Find or create clotilde root
			clotildeRoot, err := config.FindOrCreateClotildeRoot()
			if err != nil {
				return fmt.Errorf("failed to initialize session storage: %w", err)
			}

			// Create or load tour session
			store := session.NewFileStore(clotildeRoot)
			sanitized := util.SanitizeBranchName(filepath.Base(dir))
			if sanitized == "" {
				sanitized = "default"
			}
			sessionName := "tour-" + sanitized

			var sess *session.Session
			if store.Exists(sessionName) {
				sess, err = store.Get(sessionName)
				if err != nil {
					return fmt.Errorf("failed to load session: %w", err)
				}
				sess.UpdateLastAccessed()
				sess.Metadata.SystemPromptMode = "replace"
				if err := store.Update(sess); err != nil {
					return fmt.Errorf("failed to update session: %w", err)
				}
			} else {
				sess = session.NewSession(sessionName, util.GenerateUUID())
				sess.Metadata.SystemPromptMode = "replace"
				if err := store.Create(sess); err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
			}

			// Write system prompt to session (full replacement, not append)
			tourGuidePrompt := `You are a code tour guide. Explain code, architecture, and design decisions.

Guidelines:
- Reference file and line numbers from the code being discussed
- Start with the "why" before diving into the "how"
- Connect steps to broader patterns when relevant
- Be direct and concise
- When asked about code outside the tour, relate it back if possible`

			if err := store.SaveSystemPrompt(sessionName, tourGuidePrompt); err != nil {
				return fmt.Errorf("failed to save system prompt: %w", err)
			}

			// Save settings with output style and model
			settings := &session.Settings{
				Model:       model,
				OutputStyle: "explanatory",
			}
			if err := store.SaveSettings(sessionName, settings); err != nil {
				return fmt.Errorf("failed to save settings: %w", err)
			}

			srv := server.New(port, dir, model, sess, clotildeRoot)
			return srv.Start()
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().String("dir", cwd, "Directory containing .tours/ folder")
	cmd.Flags().Int("port", 3333, "Port to listen on")
	cmd.Flags().String("model", "haiku", "Claude model to use for chat (haiku, sonnet, opus)")

	return cmd
}

func newTourGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a tour file using Claude",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			name, _ := cmd.Flags().GetString("name")
			focus, _ := cmd.Flags().GetString("focus")
			model, _ := cmd.Flags().GetString("model")

			// Validate name: must be lowercase alnum+hyphens, max 30 chars to leave room
			// for the "tour-generate-<name>-<timestamp>" session name (≤64 chars total).
			if len(name) > 30 || !tourNameRe.MatchString(name) {
				return fmt.Errorf("invalid tour name %q: use lowercase letters, digits, and hyphens only, max 30 characters (e.g. \"overview\", \"auth-flow\")", name)
			}

			// Find or create clotilde root for named generation session
			clotildeRoot, err := config.FindOrCreateClotildeRoot()
			if err != nil {
				return fmt.Errorf("failed to initialize session storage: %w", err)
			}

			// Create unique named session for this generation attempt
			store := session.NewFileStore(clotildeRoot)
			// Use timestamp-based naming so each attempt gets its own session
			sessionName := fmt.Sprintf("tour-generate-%s-%d", name, time.Now().UnixMilli())

			// Create new session for this specific generation attempt
			sess := session.NewSession(sessionName, util.GenerateUUID())
			if err := store.Create(sess); err != nil {
				return fmt.Errorf("failed to create session: %w", err)
			}

			// Save settings
			settings := &session.Settings{Model: model}
			if err := store.SaveSettings(sessionName, settings); err != nil {
				return fmt.Errorf("failed to save settings: %w", err)
			}

			// Build prompt - Claude will crawl the repo itself using its file tools
			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("failed to resolve dir: %w", err)
			}
			prompt := tour.BuildGenerationPrompt(absDir, focus)

			// Invoke Claude
			fmt.Fprintln(os.Stderr, "Generating tour via Claude Code...")
			var output strings.Builder

			sessionDir := config.GetSessionDir(clotildeRoot, sessionName)
			args := []string{"--model", model, "--permission-mode", "bypassPermissions"}
			opts := claude.InvokeOptions{
				SessionID:      sess.Metadata.SessionID,
				SettingsFile:   filepath.Join(sessionDir, "settings.json"),
				AdditionalArgs: args,
			}

			err = claude.InvokeStreaming(cmd.Context(), opts, prompt, func(line string) {
				ev, parseErr := tour.ParseStreamEvent(line)
				if parseErr != nil {
					return
				}
				// Show progress for tool calls
				if summary := tour.ToolCallSummary(ev); summary != "" {
					fmt.Fprintf(os.Stderr, "  %s\n", summary)
				}
				// Capture final result
				if ev.Type == "result" {
					output.WriteString(ev.Result)
				}
			})
			if err != nil {
				return fmt.Errorf("claude invocation failed: %w", err)
			}

			// Extract and validate JSON
			raw := tour.ExtractJSON(output.String())
			toursDir := filepath.Join(dir, ".tours")
			if err := util.EnsureDir(toursDir); err != nil {
				return fmt.Errorf("failed to create .tours directory: %w", err)
			}

			outputPath := filepath.Join(toursDir, name+".tour")

			t, err := tour.ValidateTourJSON([]byte(raw), dir)
			if err != nil {
				// Save invalid output for debugging (best-effort)
				invalidPath := outputPath + ".invalid"
				_ = os.WriteFile(invalidPath, []byte(raw), 0o644)
				return fmt.Errorf("generated tour failed validation: %w\nRaw output saved to %s", err, invalidPath)
			}

			// Write validated tour
			formatted, _ := json.MarshalIndent(t, "", "  ")
			if err := util.WriteFile(outputPath, formatted); err != nil {
				return fmt.Errorf("failed to write tour file: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Generated %s (%d steps)\n", outputPath, len(t.Steps))
			fmt.Fprintf(os.Stderr, "  %q\n\n", t.Title)
			for i, step := range t.Steps {
				desc := step.Description
				if idx := strings.Index(desc, "\n"); idx > 0 {
					desc = desc[:idx]
				}
				fmt.Fprintf(os.Stderr, "  %2d. %-25s %s\n", i+1, fmt.Sprintf("%s:%d", step.File, step.Line), desc)
			}

			fmt.Fprintf(os.Stderr, "\nGeneration session: %s\n", sessionName)
			fmt.Fprintf(os.Stderr, "View transcript: clotilde inspect %s\n", sessionName)

			return nil
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().String("dir", cwd, "Repository directory to analyze")
	cmd.Flags().String("name", "overview", "Tour name (output: .tours/<name>.tour)")
	cmd.Flags().String("focus", "", "Focus on a specific area (e.g. 'auth flow')")
	cmd.Flags().String("model", "sonnet", "Claude model to use for generation")

	return cmd
}
