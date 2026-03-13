package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/export"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "export <name>",
		Short:             "Export session transcript as self-contained HTML",
		Long:              `Render a Claude Code session transcript (JSONL) into a single, self-contained HTML file.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: sessionNameCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			clotildeRoot, err := config.FindClotildeRoot()
			if err != nil {
				return fmt.Errorf("no sessions found (create one with 'clotilde start <name>')")
			}

			store := session.NewFileStore(clotildeRoot)
			sess, err := store.Get(name)
			if err != nil {
				return fmt.Errorf("session '%s' not found", name)
			}

			// Collect entries from all transcripts (previous + current)
			homeDir, err := util.HomeDir()
			if err != nil {
				return fmt.Errorf("could not determine home directory: %w", err)
			}

			paths := allTranscriptPaths(sess, clotildeRoot, homeDir)
			var allEntries []json.RawMessage
			var readable int
			for _, path := range paths {
				f, err := os.Open(path)
				if err != nil {
					continue // transcript missing — skip
				}
				entries, err := export.FilterTranscript(f)
				_ = f.Close()
				if err != nil {
					continue
				}
				readable++
				allEntries = append(allEntries, entries...)
			}
			if readable == 0 {
				return fmt.Errorf("no transcript found for session '%s'", name)
			}

			html, err := export.BuildHTML(name, allEntries)
			if err != nil {
				return fmt.Errorf("building HTML: %w", err)
			}

			toStdout, _ := cmd.Flags().GetBool("stdout")
			if toStdout {
				_, err = fmt.Fprint(cmd.OutOrStdout(), html)
				return err
			}

			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath == "" {
				outputPath = name + ".html"
			}

			if err := os.WriteFile(outputPath, []byte(html), 0o644); err != nil {
				return fmt.Errorf("writing output file: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output file path (default: ./<name>.html)")
	cmd.Flags().Bool("stdout", false, "Write to stdout instead of file")

	return cmd
}
