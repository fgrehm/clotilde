package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
	"github.com/fgrehm/clotilde/internal/server"
	"github.com/fgrehm/clotilde/internal/tour"
	"github.com/fgrehm/clotilde/internal/util"
)

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
				fmt.Fprintln(cmd.OutOrStdout(), "No tours found.")
				fmt.Fprintf(cmd.OutOrStdout(), "\nCreate a tour file in %s/\n", toursDir)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Tours (%d):\n", len(tours))
			for name, t := range tours {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s (%d steps)\n", name, t.Title, len(t.Steps))
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

			srv := server.New(port, dir)
			return srv.Start()
		},
	}

	cwd, _ := os.Getwd()
	cmd.Flags().String("dir", cwd, "Directory containing .tours/ folder")
	cmd.Flags().Int("port", 3333, "Port to listen on")

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

			// Gather repo context
			fmt.Fprintln(os.Stderr, "Gathering repo context...")
			ctx, err := tour.GatherContext(dir, tour.ContextOptions{Focus: focus})
			if err != nil {
				return fmt.Errorf("failed to gather context: %w", err)
			}

			// Build prompt
			prompt := tour.BuildGenerationPrompt(ctx, focus)

			// Invoke Claude
			fmt.Fprintln(os.Stderr, "Generating tour via Claude Code...")
			sessionID := util.GenerateUUID()
			var output strings.Builder

			args := []string{"--model", model}
			opts := claude.InvokeOptions{
				SessionID:      sessionID,
				AdditionalArgs: args,
			}

			err = claude.InvokeStreaming(opts, prompt, func(line string) {
				// Extract result text from stream-json
				var parsed map[string]any
				if err := json.Unmarshal([]byte(line), &parsed); err != nil {
					return
				}
				if parsed["type"] == "result" {
					if result, ok := parsed["result"].(string); ok {
						output.WriteString(result)
					}
				}
			})
			if err != nil {
				return fmt.Errorf("claude invocation failed: %w", err)
			}

			// Extract and validate JSON
			raw := tour.ExtractJSON(output.String())
			toursDir := filepath.Join(dir, ".tours")
			if err := os.MkdirAll(toursDir, 0o755); err != nil {
				return fmt.Errorf("failed to create .tours directory: %w", err)
			}

			outputPath := filepath.Join(toursDir, name+".tour")

			t, err := tour.ValidateTourJSON([]byte(raw), dir)
			if err != nil {
				// Save invalid output for debugging
				invalidPath := outputPath + ".invalid"
				os.WriteFile(invalidPath, []byte(raw), 0o644)
				return fmt.Errorf("generated tour failed validation: %w\nRaw output saved to %s", err, invalidPath)
			}

			// Write validated tour
			formatted, _ := json.MarshalIndent(t, "", "  ")
			if err := os.WriteFile(outputPath, formatted, 0o644); err != nil {
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
