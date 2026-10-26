package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// defaultCLIHost is the host the Laravel Cloud CLI treats as its default. Its
// tokens live in config.json; tokens for any other host live in a file named
// after that host, so a token for one deployment is never sent to another.
const defaultCLIHost = "cloud.laravel.com"

// cliHomeDir returns the home directory the Laravel Cloud CLI resolves, in
// the same order it does: $HOME, then $USERPROFILE, then the account's entry
// in the user database. os.UserHomeDir is not a substitute -- it reads only
// $USERPROFILE on Windows, where the CLI prefers $HOME (set by Git Bash), and
// only $HOME elsewhere, so the two would disagree about where the token lives.
func cliHomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return u.HomeDir, nil
}

// cliConfigPath returns where the Laravel Cloud CLI (`cloud auth`) keeps the
// API tokens for baseURL. It mirrors the CLI's ConfigRepository, which always
// uses <home>/.config/cloud -- it ignores XDG_CONFIG_HOME and has no override,
// so neither does this. An empty or default base URL maps to config.json, any
// other host to <host-with-dashes>-config.json.
func cliConfigPath(baseURL string) (string, error) {
	home, err := cliHomeDir()
	if err != nil {
		return "", err
	}

	filename := "config.json"
	if baseURL != "" {
		u, err := url.Parse(baseURL)
		if err != nil {
			return "", fmt.Errorf("parsing base URL %q: %w", baseURL, err)
		}
		if host := u.Hostname(); host != "" && host != defaultCLIHost {
			filename = strings.ReplaceAll(host, ".", "-") + "-config.json"
		}
	}

	return filepath.Join(home, ".config", "cloud", filename), nil
}

// cliToken returns the API token saved by the Laravel Cloud CLI for baseURL,
// and the file it was read from. A missing file, or one with no tokens, yields
// an empty token and no error: the CLI is a fallback, not a requirement.
//
// The CLI saves one token per organization. When there are several it asks the
// API which organization each belongs to and prompts for a choice; a provider
// cannot prompt, and guessing would apply a plan to the wrong organization, so
// more than one token is an error.
func cliToken(baseURL string) (token, path string, err error) {
	path, err = cliConfigPath(baseURL)
	if err != nil {
		return "", "", err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", path, nil
	}
	if err != nil {
		return "", path, err
	}

	var config struct {
		APITokens []string `json:"api_tokens"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return "", path, fmt.Errorf("parsing %s: %w", path, err)
	}

	switch len(config.APITokens) {
	case 0:
		return "", path, nil
	case 1:
		return config.APITokens[0], path, nil
	default:
		return "", path, fmt.Errorf(
			"%s holds %d API tokens, one per organization, and the provider cannot tell which to use",
			path, len(config.APITokens),
		)
	}
}
