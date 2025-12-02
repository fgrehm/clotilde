package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/fgrehm/clotilde/internal/config"
	"github.com/fgrehm/clotilde/internal/outputstyle"
	"github.com/fgrehm/clotilde/internal/session"
	"github.com/fgrehm/clotilde/internal/util"
)

// buildSessionCreateParams extracts session creation parameters from command flags.
// Used by the start command which has an --incognito flag.
func buildSessionCreateParams(cmd *cobra.Command, name string) (SessionCreateParams, error) {
	params := buildCommonParams(cmd, name)
	params.Incognito, _ = cmd.Flags().GetBool("incognito")

	// Validate output style flags
	if params.OutputStyle != "" && params.OutputStyleFile != "" {
		return SessionCreateParams{}, fmt.Errorf("cannot specify both --output-style and --output-style-file")
	}

	return params, nil
}

// buildIncognitoParams extracts session creation parameters for incognito command.
// Always sets Incognito: true since the incognito command doesn't have that flag.
func buildIncognitoParams(cmd *cobra.Command, name string) SessionCreateParams {
	params := buildCommonParams(cmd, name)
	params.Incognito = true
	return params
}

// buildCommonParams extracts common session creation parameters from command flags.
func buildCommonParams(cmd *cobra.Command, name string) SessionCreateParams {
	model, _ := cmd.Flags().GetString("model")
	systemPrompt, _ := cmd.Flags().GetString("append-system-prompt")
	systemPromptFile, _ := cmd.Flags().GetString("append-system-prompt-file")
	replaceSystemPrompt, _ := cmd.Flags().GetString("replace-system-prompt")
	replaceSystemPromptFile, _ := cmd.Flags().GetString("replace-system-prompt-file")
	permissionMode, _ := cmd.Flags().GetString("permission-mode")
	allowedTools, _ := cmd.Flags().GetStringSlice("allowed-tools")
	disallowedTools, _ := cmd.Flags().GetStringSlice("disallowed-tools")
	additionalDirs, _ := cmd.Flags().GetStringSlice("add-dir")
	outputStyle, _ := cmd.Flags().GetString("output-style")
	outputStyleFile, _ := cmd.Flags().GetString("output-style-file")

	return SessionCreateParams{
		Name:                    name,
		Model:                   model,
		SystemPrompt:            systemPrompt,
		SystemPromptFile:        systemPromptFile,
		ReplaceSystemPrompt:     replaceSystemPrompt,
		ReplaceSystemPromptFile: replaceSystemPromptFile,
		PermissionMode:          permissionMode,
		AllowedTools:            allowedTools,
		DisallowedTools:         disallowedTools,
		AdditionalDirs:          additionalDirs,
		OutputStyle:             outputStyle,
		OutputStyleFile:         outputStyleFile,
	}
}

// SessionCreateParams holds parameters for creating a new session.
type SessionCreateParams struct {
	Name                    string
	Model                   string
	SystemPrompt            string // inline content (append mode)
	SystemPromptFile        string // path to read from (append mode)
	ReplaceSystemPrompt     string // inline content (replace mode)
	ReplaceSystemPromptFile string // path to read from (replace mode)
	PermissionMode          string
	AllowedTools            []string
	DisallowedTools         []string
	AdditionalDirs          []string
	OutputStyle             string // built-in style, custom style name, or inline content
	OutputStyleFile         string // path to custom style file
	Incognito               bool
}

// SessionCreateResult holds the created session and file paths.
type SessionCreateResult struct {
	ClotildeRoot     string
	Session          *session.Session
	SettingsFile     string
	SystemPromptFile string
}

// createSession handles common session creation logic.
// Returns the session ready for claude.Start() invocation.
func createSession(params SessionCreateParams) (*SessionCreateResult, error) {
	// Validate system prompt flags
	hasAppend := params.SystemPrompt != "" || params.SystemPromptFile != ""
	hasReplace := params.ReplaceSystemPrompt != "" || params.ReplaceSystemPromptFile != ""
	if hasAppend && hasReplace {
		return nil, fmt.Errorf("cannot use both append and replace system prompt flags")
	}

	// Find clotilde root
	clotildeRoot, err := config.FindClotildeRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a clotilde project (run 'clotilde init' first)")
	}

	// Validate session name
	if err := session.ValidateName(params.Name); err != nil {
		return nil, err
	}

	// Create store
	store := session.NewFileStore(clotildeRoot)

	// Check if session already exists
	if store.Exists(params.Name) {
		return nil, fmt.Errorf("session '%s' already exists", params.Name)
	}

	// Generate UUID for the session
	sessionID := util.GenerateUUID()

	// Create session (incognito or regular)
	var sess *session.Session
	if params.Incognito {
		sess = session.NewIncognitoSession(params.Name, sessionID)
	} else {
		sess = session.NewSession(params.Name, sessionID)
	}

	// Set system prompt mode
	if hasReplace {
		sess.Metadata.SystemPromptMode = "replace"
	} else {
		sess.Metadata.SystemPromptMode = "append"
	}

	if err := store.Create(sess); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	sessionDir := config.GetSessionDir(clotildeRoot, params.Name)

	// Save settings (always create settings.json, even if empty)
	settings := &session.Settings{}
	if params.Model != "" {
		settings.Model = params.Model
	}

	// Handle output style
	var hasCustomStyle bool
	if params.OutputStyleFile != "" {
		// Create custom style from file (validates/injects frontmatter)
		if err := outputstyle.CreateCustomStyleFileFromFile(clotildeRoot, params.Name, params.OutputStyleFile); err != nil {
			return nil, fmt.Errorf("failed to create custom style: %w", err)
		}
		settings.OutputStyle = outputstyle.GetCustomStyleReference(params.Name)
		hasCustomStyle = true
	} else if params.OutputStyle != "" {
		if outputstyle.IsBuiltIn(params.OutputStyle) {
			// Use built-in style directly
			settings.OutputStyle = params.OutputStyle
			hasCustomStyle = false
		} else if outputstyle.StyleExists(clotildeRoot, params.OutputStyle) {
			// Reference existing style by name (don't create new file)
			settings.OutputStyle = params.OutputStyle
			hasCustomStyle = false
		} else {
			// Treat as custom inline content - create new session-specific style
			if err := outputstyle.CreateCustomStyleFile(clotildeRoot, params.Name, params.OutputStyle); err != nil {
				return nil, fmt.Errorf("failed to create custom style: %w", err)
			}
			settings.OutputStyle = outputstyle.GetCustomStyleReference(params.Name)
			hasCustomStyle = true
		}
	}

	// Update metadata
	sess.Metadata.HasCustomOutputStyle = hasCustomStyle

	// Build permissions if any flags provided
	if params.PermissionMode != "" || len(params.AllowedTools) > 0 || len(params.DisallowedTools) > 0 || len(params.AdditionalDirs) > 0 {
		settings.Permissions = session.Permissions{
			DefaultMode:           params.PermissionMode,
			Allow:                 params.AllowedTools,
			Deny:                  params.DisallowedTools,
			AdditionalDirectories: params.AdditionalDirs,
		}
	}

	if err := store.SaveSettings(params.Name, settings); err != nil {
		return nil, fmt.Errorf("failed to save settings: %w", err)
	}

	// Update session with the metadata changes (for output style)
	if err := store.Update(sess); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Build result
	result := &SessionCreateResult{
		ClotildeRoot: clotildeRoot,
		Session:      sess,
		SettingsFile: filepath.Join(sessionDir, "settings.json"),
	}

	// Save system prompt (handle both append and replace modes)
	promptContent := ""
	if params.SystemPrompt != "" {
		promptContent = params.SystemPrompt
	} else if params.SystemPromptFile != "" {
		content, err := os.ReadFile(params.SystemPromptFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read system prompt file: %w", err)
		}
		promptContent = string(content)
	} else if params.ReplaceSystemPrompt != "" {
		promptContent = params.ReplaceSystemPrompt
	} else if params.ReplaceSystemPromptFile != "" {
		content, err := os.ReadFile(params.ReplaceSystemPromptFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read system prompt file: %w", err)
		}
		promptContent = string(content)
	}

	if promptContent != "" {
		if err := store.SaveSystemPrompt(params.Name, promptContent); err != nil {
			return nil, fmt.Errorf("failed to save system prompt: %w", err)
		}
		result.SystemPromptFile = filepath.Join(sessionDir, "system-prompt.md")
	}

	return result, nil
}
