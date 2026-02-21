package cmd

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/session"
)

// sessionNameCompletion provides dynamic completion for session names
func sessionNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Find clotilde root
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get all sessions
	store := session.NewFileStore(clotildeRoot)
	sessions, err := store.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Extract session names
	var names []string
	for _, sess := range sessions {
		names = append(names, sess.Metadata.Name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// modelCompletion provides completion for Claude model names
func modelCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"haiku", "sonnet", "opus"}, cobra.ShellCompDirectiveNoFileComp
}

// profileNameCompletion provides dynamic completion for profile names
func profileNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Find clotilde root
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Load merged profiles (global + project)
	profiles, err := config.MergedProfiles(clotildeRoot)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Extract profile names
	var names []string
	for name := range profiles {
		names = append(names, name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}

// outputStyleCompletion provides completion for --output-style flag
func outputStyleCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{
		"default\tClaude completes tasks efficiently with concise responses",
		"Explanatory\tClaude explains implementation choices and patterns",
		"Learning\tClaude asks you to write code for hands-on practice",
	}, cobra.ShellCompDirectiveNoFileComp
}

// newCompletionCmd returns a custom completion command that wraps Cobra's built-in completion
// and adds support for registering aliases
func newCompletionCmd() *cobra.Command {
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate the autocompletion script for the specified shell",
		Long:  `Generate the autocompletion script for clotilde for the specified shell.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("please specify a shell (bash, zsh, fish, or powershell)")
		},
	}

	// Add shell-specific subcommands
	bashCmd := &cobra.Command{
		Use:   "bash",
		Short: "Generate the autocompletion script for bash",
		Long: `Generate the autocompletion script for clotilde for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(clotilde completion bash)

To load completions for every new session, execute once:

#### Linux:

	clotilde completion bash > /etc/bash_completion.d/clotilde

#### macOS:

	clotilde completion bash > $(brew --prefix)/etc/bash_completion.d/clotilde

You will need to start a new shell for this setup to take effect.

To enable completion for an alias (e.g., 'clo' for 'clotilde'), run:

	clotilde completion bash --register-alias clo > /etc/bash_completion.d/clotilde

Alternatively, you can add the alias registration to your shell session:

	alias clo=clotilde
	complete -o default -F __start_clotilde clo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias, _ := cmd.Flags().GetString("register-alias")
			return generateCompletionBash(cmd, alias)
		},
	}
	bashCmd.Flags().StringP("register-alias", "", "", "Register completion for an additional alias (e.g., 'clo')")

	zshCmd := &cobra.Command{
		Use:   "zsh",
		Short: "Generate the autocompletion script for zsh",
		Long: `Generate the autocompletion script for clotilde for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -Uz compinit && compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(clotilde completion zsh); compdef _clotilde clotilde

To load completions for every new session, execute once:

#### oh-my-zsh users:

	mkdir -p ~/.oh-my-zsh/completions
	clotilde completion zsh > ~/.oh-my-zsh/completions/_clotilde

#### Standard zsh users:

	mkdir -p ~/.zsh/completions
	clotilde completion zsh > ~/.zsh/completions/_clotilde
	echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
	echo 'autoload -Uz compinit && compinit' >> ~/.zshrc

#### macOS (Homebrew):

	clotilde completion zsh > $(brew --prefix)/share/zsh/site-functions/_clotilde

You will need to start a new shell for this setup to take effect.

To enable completion for an alias (e.g., 'clo' for 'clotilde'):

	alias clo=clotilde
	compdef _clotilde clo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias, _ := cmd.Flags().GetString("register-alias")
			return generateCompletionZsh(cmd, alias)
		},
	}
	zshCmd.Flags().StringP("register-alias", "", "", "Register completion for an additional alias (e.g., 'clo')")

	fishCmd := &cobra.Command{
		Use:   "fish",
		Short: "Generate the autocompletion script for fish",
		Long: `Generate the autocompletion script for clotilde for the fish shell.

To load completions in your current shell session:

	clotilde completion fish | source

To load completions for every new session, execute once:

	clotilde completion fish > ~/.config/fish/completions/clotilde.fish

You will need to start a new shell for this setup to take effect.

Note: Fish automatically handles command aliases, so completion works with any alias
you define (e.g., alias clo=clotilde). No additional configuration needed.

To register an alias in the completion output for reference:

	clotilde completion fish --register-alias clo > ~/.config/fish/completions/clotilde.fish`,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias, _ := cmd.Flags().GetString("register-alias")
			return generateCompletionFish(cmd, alias)
		},
	}
	fishCmd.Flags().StringP("register-alias", "", "", "Register completion for an additional alias (e.g., 'clo')")

	powershellCmd := &cobra.Command{
		Use:   "powershell",
		Short: "Generate the autocompletion script for powershell",
		Long: `Generate the autocompletion script for clotilde for the powershell shell.

To load completions in your current shell session:

	clotilde completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of:

	clotilde completion powershell

to your powershell profile.

To enable completion for an alias (e.g., 'clo' for 'clotilde'), add both the
completion script and alias registration to your PowerShell profile:

	clotilde completion powershell --register-alias clo

This will include a note about using Set-Alias to register the alias completion.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias, _ := cmd.Flags().GetString("register-alias")
			return generateCompletionPowershell(cmd, alias)
		},
	}
	powershellCmd.Flags().StringP("register-alias", "", "", "Register completion for an additional alias (e.g., 'clo')")

	completionCmd.AddCommand(bashCmd)
	completionCmd.AddCommand(zshCmd)
	completionCmd.AddCommand(fishCmd)
	completionCmd.AddCommand(powershellCmd)

	return completionCmd
}

// generateCompletionBash generates bash completion with optional alias support
func generateCompletionBash(_ *cobra.Command, aliasName string) error {
	var buf bytes.Buffer
	if err := rootCmd.GenBashCompletionV2(&buf, false); err != nil {
		return err
	}

	script := buf.String()

	// If alias is specified, add completion registration for it
	if aliasName != "" {
		script = appendBashAlias(script, aliasName)
	}

	fmt.Print(script)
	return nil
}

// generateCompletionZsh generates zsh completion with optional alias support
func generateCompletionZsh(_ *cobra.Command, aliasName string) error {
	var buf bytes.Buffer
	if err := rootCmd.GenZshCompletion(&buf); err != nil {
		return err
	}

	script := buf.String()

	// If alias is specified, add completion registration for it
	if aliasName != "" {
		script = appendZshAlias(script, aliasName)
	}

	fmt.Print(script)
	return nil
}

// generateCompletionFish generates fish completion with optional alias support
func generateCompletionFish(_ *cobra.Command, aliasName string) error {
	var buf bytes.Buffer
	if err := rootCmd.GenFishCompletion(&buf, false); err != nil {
		return err
	}

	script := buf.String()

	// If alias is specified, add completion registration for it
	if aliasName != "" {
		script = appendFishAlias(script, aliasName)
	}

	fmt.Print(script)
	return nil
}

// generateCompletionPowershell generates powershell completion with optional alias support
func generateCompletionPowershell(_ *cobra.Command, aliasName string) error {
	var buf bytes.Buffer
	if err := rootCmd.GenPowerShellCompletionWithDesc(&buf); err != nil {
		return err
	}

	script := buf.String()

	// If alias is specified, add completion registration for it
	if aliasName != "" {
		script = appendPowershellAlias(script, aliasName)
	}

	fmt.Print(script)
	return nil
}

// appendBashAlias adds a completion registration line for the given alias
func appendBashAlias(script, aliasName string) string {
	// Find the complete command line and duplicate it for the alias
	// Pattern: complete -o ... -F __start_clotilde clotilde
	re := regexp.MustCompile(`(complete\s+-o\s+\S+\s+-F\s+__start_clotilde\s+)clotilde`)
	match := re.FindStringSubmatch(script)
	if len(match) > 0 {
		aliasLine := fmt.Sprintf("%s%s", match[1], aliasName)
		script = strings.TrimSuffix(script, "\n")
		script += "\n" + aliasLine + "\n"
	}
	return script
}

// appendZshAlias adds a completion registration line for the given alias
func appendZshAlias(script, aliasName string) string {
	// Find the compdef line and add one for the alias
	// Pattern: compdef _clotilde clotilde
	if strings.Contains(script, "compdef _clotilde clotilde") {
		script = strings.TrimSuffix(script, "\n")
		script += fmt.Sprintf("\ncompdef _clotilde %s\n", aliasName)
	}
	return script
}

// appendFishAlias adds a completion registration comment for the given alias
func appendFishAlias(script, aliasName string) string {
	// Fish completions use function names based on command, so we document how to add an alias
	script = strings.TrimSuffix(script, "\n")
	script += fmt.Sprintf(`
# To enable completion for alias '%s', add this to your fish config:
# alias %s=clotilde
`, aliasName, aliasName)
	return script
}

// appendPowershellAlias adds a completion comment for the given alias
func appendPowershellAlias(script, aliasName string) string {
	// PowerShell completions are function-based, document how to use with alias
	script = strings.TrimSuffix(script, "\n")
	script += fmt.Sprintf(`
# To enable completion for alias '%s', add to your PowerShell profile:
# Set-Alias -Name %s -Value clotilde
`, aliasName, aliasName)
	return script
}
