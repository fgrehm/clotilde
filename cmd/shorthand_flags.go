package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// maxPositionalArgs returns a cobra.PositionalArgs validator that allows at most
// max positional args before "--". Cobra's built-in MaximumNArgs counts args
// after "--" too, which breaks commands that pass extra flags through to claude.
func maxPositionalArgs(max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		n := len(args)
		if dash := cmd.Flags().ArgsLenAtDash(); dash >= 0 {
			n = dash
		}
		if n > max {
			return fmt.Errorf("accepts at most %d arg(s), received %d", max, n)
		}
		return nil
	}
}

// rangePositionalArgs returns a cobra.PositionalArgs validator that allows
// between min and max positional args before "--".
func rangePositionalArgs(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		n := len(args)
		if dash := cmd.Flags().ArgsLenAtDash(); dash >= 0 {
			n = dash
		}
		if n < min || n > max {
			return fmt.Errorf("accepts between %d and %d arg(s), received %d", min, max, n)
		}
		return nil
	}
}

// registerShorthandFlags adds permission mode shortcuts and the --fast composite
// preset to the given command.
func registerShorthandFlags(cmd *cobra.Command) {
	// Permission mode shortcuts
	cmd.Flags().Bool("accept-edits", false, "Shorthand for --permission-mode acceptEdits")
	cmd.Flags().Bool("yolo", false, "Shorthand for --permission-mode bypassPermissions")
	cmd.Flags().Bool("plan", false, "Shorthand for --permission-mode plan")
	cmd.Flags().Bool("dont-ask", false, "Shorthand for --permission-mode dontAsk")

	// Composite preset
	cmd.Flags().Bool("fast", false, "Use haiku model with low effort for quick tasks")

	// Effort level (pass-through to claude CLI)
	cmd.Flags().String("effort", "", "Reasoning effort level (low, medium, high, max)")
}

// resolvePermissionMode reads the four permission shorthand bools and the
// explicit --permission-mode string (if registered on cmd), validates that
// at most one is set, and returns the resolved permission mode string.
// Returns ("", nil) if none are set.
func resolvePermissionMode(cmd *cobra.Command) (string, error) {
	// Read explicit --permission-mode if this command has it
	var explicit string
	if cmd.Flags().Lookup("permission-mode") != nil {
		explicit, _ = cmd.Flags().GetString("permission-mode")
	}

	acceptEdits, _ := cmd.Flags().GetBool("accept-edits")
	yolo, _ := cmd.Flags().GetBool("yolo")
	plan, _ := cmd.Flags().GetBool("plan")
	dontAsk, _ := cmd.Flags().GetBool("dont-ask")

	count := 0
	mode := explicit
	if acceptEdits {
		count++
		mode = "acceptEdits"
	}
	if yolo {
		count++
		mode = "bypassPermissions"
	}
	if plan {
		count++
		mode = "plan"
	}
	if dontAsk {
		count++
		mode = "dontAsk"
	}

	if count > 1 {
		return "", fmt.Errorf("cannot combine multiple permission mode shortcuts (--accept-edits, --yolo, --plan, --dont-ask)")
	}
	if count == 1 && explicit != "" {
		return "", fmt.Errorf("cannot combine permission mode shortcut with --permission-mode")
	}
	return mode, nil
}

// resolveFastMode checks if --fast is set and validates conflicts with --model
// (if the command has it). Returns true if --fast was set.
//
// When true, the caller should:
//   - For session-creating commands: set model to "haiku" via cmd.Flags().Set()
//     and append ["--effort", "low"] to additionalArgs
//   - For non-creating commands: append ["--model", "haiku", "--effort", "low"]
//     to additionalArgs
func resolveFastMode(cmd *cobra.Command) (bool, error) {
	fast, _ := cmd.Flags().GetBool("fast")
	if !fast {
		return false, nil
	}
	if cmd.Flags().Lookup("model") != nil && cmd.Flags().Changed("model") {
		return false, fmt.Errorf("cannot use --fast with --model")
	}
	if cmd.Flags().Changed("effort") {
		return false, fmt.Errorf("cannot use --fast with --effort")
	}
	return true, nil
}

// collectEffortFlag appends --effort to additionalArgs if the flag is set.
// Called after resolveFastMode (which may also append --effort low).
func collectEffortFlag(cmd *cobra.Command, additionalArgs []string) []string {
	effort, _ := cmd.Flags().GetString("effort")
	if effort != "" {
		additionalArgs = append(additionalArgs, "--effort", effort)
	}
	return additionalArgs
}
