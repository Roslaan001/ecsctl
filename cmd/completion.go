package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:       "completion <bash|zsh|fish|powershell>",
	Short:     "Install shell autocompletion",
	Long:      "Install ecsctl autocompletion for the selected shell.",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE:      installCompletion,
}

func installCompletion(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}

	shell := strings.ToLower(args[0])
	var path string
	var generate func(io.Writer) error
	switch shell {
	case "bash":
		path = filepath.Join(home, ".local", "share", "bash-completion", "completions", "ecsctl")
		generate = func(writer io.Writer) error { return rootCmd.GenBashCompletionV2(writer, true) }
	case "zsh":
		path = filepath.Join(home, ".zsh", "completion", "_ecsctl")
		generate = rootCmd.GenZshCompletion
	case "fish":
		path = filepath.Join(home, ".config", "fish", "completions", "ecsctl.fish")
		generate = func(writer io.Writer) error { return rootCmd.GenFishCompletion(writer, true) }
	case "powershell":
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("find user config directory: %w", err)
		}
		path = filepath.Join(configDir, "powershell", "completions", "ecsctl.ps1")
		generate = rootCmd.GenPowerShellCompletion
	default:
		return fmt.Errorf("unsupported shell %q (choose bash, zsh, fish, or powershell)", args[0])
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion directory: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create completion file: %w", err)
	}
	if err := generate(file); err != nil {
		file.Close()
		return fmt.Errorf("generate %s completion: %w", shell, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("save completion file: %w", err)
	}

	if shell == "zsh" {
		if err := enableZshCompletion(home); err != nil {
			return err
		}
	}
	if shell == "powershell" {
		if err := enablePowerShellCompletion(home, path); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Installed %s completion at %s. Restart your shell to enable it.\n", shell, path)
	return err
}

func enableZshCompletion(home string) error {
	rcPath := filepath.Join(home, ".zshrc")
	const block = "\n# ecsctl shell completion\nfpath=(\"$HOME/.zsh/completion\" $fpath)\nautoload -Uz compinit && compinit\n"
	data, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", rcPath, err)
	}
	if strings.Contains(string(data), "# ecsctl shell completion") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return fmt.Errorf("create zsh config directory: %w", err)
	}
	if err := os.WriteFile(rcPath, append(data, []byte(block)...), 0o644); err != nil {
		return fmt.Errorf("update %s: %w", rcPath, err)
	}
	return nil
}

func enablePowerShellCompletion(home, completionPath string) error {
	profileDir := filepath.Join(home, "Documents", "PowerShell")
	if runtime.GOOS != "windows" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("find user config directory: %w", err)
		}
		profileDir = filepath.Join(configDir, "powershell")
	}
	profile := filepath.Join(profileDir, "Microsoft.PowerShell_profile.ps1")
	quotedPath := strings.ReplaceAll(completionPath, "'", "''")
	block := fmt.Sprintf("\n# ecsctl shell completion\n. '%s'\n", quotedPath)
	data, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", profile, err)
	}
	if strings.Contains(string(data), "# ecsctl shell completion") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		return fmt.Errorf("create PowerShell profile directory: %w", err)
	}
	if err := os.WriteFile(profile, append(data, []byte(block)...), 0o644); err != nil {
		return fmt.Errorf("update %s: %w", profile, err)
	}
	return nil
}
