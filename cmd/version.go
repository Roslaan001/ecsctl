package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Version, Commit, and BuildDate are set at build time via -ldflags.
var (
	Version      = "dev"
	Commit       = "none"
	BuildDate    = "unknown"
	versionCheck bool
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ecsctl version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionCheck {
			return checkLatestVersion(cmd)
		}
		fmt.Printf("ecsctl version %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionCheck, "check", false, "Check GitHub Releases for a newer version")
	rootCmd.AddCommand(versionCmd)
}

func checkLatestVersion(cmd *cobra.Command) (runErr error) {
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/Roslaan001/ecsctl/releases/latest", nil)
	if err != nil {
		return fmt.Errorf("creating release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ecsctl-update-check")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("checking latest ecsctl release: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil && runErr == nil {
			runErr = fmt.Errorf("closing GitHub response: %w", err)
		}
	}()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("checking latest ecsctl release: GitHub returned %s", response.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return fmt.Errorf("reading latest ecsctl release: %w", err)
	}
	if release.TagName == "" {
		return fmt.Errorf("GitHub returned a release without a version tag")
	}

	out := cmd.OutOrStdout()
	if compareVersions(Version, release.TagName) >= 0 {
		_, err := fmt.Fprintf(out, "ecsctl %s is up to date (latest release: %s).\n", Version, release.TagName)
		return err
	}
	if _, err := fmt.Fprintf(out, "A newer ecsctl release is available: %s → %s.\n", Version, release.TagName); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_, err = fmt.Fprintln(out, "Install it with: irm https://roslaan001.github.io/ecsctl/install.ps1 | iex")
	} else {
		_, err = fmt.Fprintln(out, "Install it with: curl -fsSL https://roslaan001.github.io/ecsctl/install.sh | sh")
	}
	return err
}

func compareVersions(current, latest string) int {
	currentParts, currentOK := numericVersion(current)
	latestParts, latestOK := numericVersion(latest)
	if !currentOK || !latestOK {
		return strings.Compare(current, latest)
	}
	for i := range currentParts {
		if currentParts[i] < latestParts[i] {
			return -1
		}
		if currentParts[i] > latestParts[i] {
			return 1
		}
	}
	return 0
}

func numericVersion(value string) ([3]int, bool) {
	var parts [3]int
	value = strings.TrimPrefix(value, "v")
	value = strings.SplitN(value, "-", 2)[0]
	segments := strings.Split(value, ".")
	if len(segments) != len(parts) {
		return parts, false
	}
	for i, segment := range segments {
		parsed, err := strconv.Atoi(segment)
		if err != nil || parsed < 0 {
			return parts, false
		}
		parts[i] = parsed
	}
	return parts, true
}
