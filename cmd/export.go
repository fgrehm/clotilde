package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/claude"
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

			// Resolve transcript path
			transcriptPath := sess.Metadata.TranscriptPath
			if transcriptPath == "" {
				homeDir, err := util.HomeDir()
				if err != nil {
					return fmt.Errorf("could not determine transcript path for session '%s': %w", name, err)
				}
				projectDir := claude.ProjectDir(clotildeRoot)
				claudeProjectDir := filepath.Join(homeDir, ".claude", "projects", projectDir)
				transcriptPath = filepath.Join(claudeProjectDir, sess.Metadata.SessionID+".jsonl")
			}

			toStdout, _ := cmd.Flags().GetBool("stdout")
			if toStdout {
				return export.ExportToWriter(transcriptPath, name, cmd.OutOrStdout())
			}

			outputPath, _ := cmd.Flags().GetString("output")
			if outputPath == "" {
				outputPath = name + ".html"
			}

			if err := export.Export(transcriptPath, name, outputPath); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outputPath)
			return nil
		},
	}

	cmd.Flags().StringP("output", "o", "", "Output file path (default: ./<name>.html)")
	cmd.Flags().Bool("stdout", false, "Write to stdout instead of file")

	return cmd
}
