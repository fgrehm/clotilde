package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/server"
	"github.com/fgrehm/clotilde/internal/tour"
)

func newTourCmd() *cobra.Command {
	tourCmd := &cobra.Command{
		Use:   "tour",
		Short: "Interactive codebase tours via browser",
		Long:  "Browse codebase tours with syntax-highlighted code, step descriptions, and an AI chat sidebar.",
	}

	tourCmd.AddCommand(newTourListCmd())
	tourCmd.AddCommand(newTourServeCmd())

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
