package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setCLIHome points the user's home directory at a temp dir, so tests never
// read the real ~/.config/cloud of whoever runs them.
func setCLIHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeCLIConfig(t *testing.T, home, filename, contents string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "cloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCLIConfigPath(t *testing.T) {
	home := setCLIHome(t)

	cases := map[string]string{
		"":                                  "config.json",
		"https://cloud.laravel.com/api":     "config.json",
		"https://cloud.laravel.com":         "config.json",
		"https://staging.cloud.example/api": "staging-cloud-example-config.json",
		"http://localhost:8080/api":         "localhost-config.json",
	}
	for baseURL, want := range cases {
		got, err := cliConfigPath(baseURL)
		if err != nil {
			t.Fatalf("cliConfigPath(%q): %v", baseURL, err)
		}
		if want := filepath.Join(home, ".config", "cloud", want); got != want {
			t.Errorf("cliConfigPath(%q) = %q, want %q", baseURL, got, want)
		}
	}
}

func TestCLIHomeDir(t *testing.T) {
	t.Setenv("HOME", "/from/home")
	t.Setenv("USERPROFILE", "/from/userprofile")
	if got, _ := cliHomeDir(); got != "/from/home" {
		t.Errorf("with HOME set: got %q, want /from/home", got)
	}

	t.Setenv("HOME", "")
	if got, _ := cliHomeDir(); got != "/from/userprofile" {
		t.Errorf("with only USERPROFILE set: got %q, want /from/userprofile", got)
	}

	t.Setenv("USERPROFILE", "")
	got, err := cliHomeDir()
	if err != nil {
		t.Fatalf("with neither set: %v", err)
	}
	if got == "" {
		t.Error("with neither set: got an empty home directory")
	}
}

func TestCLIToken(t *testing.T) {
	cases := []struct {
		name      string
		filename  string
		contents  string
		baseURL   string
		wantToken string
		wantErr   string
	}{
		{name: "no file"},
		{name: "single token", filename: "config.json", contents: `{"api_tokens":["tok-1"]}`, wantToken: "tok-1"},
		{name: "no tokens", filename: "config.json", contents: `{"api_tokens":[]}`},
		{name: "other keys only", filename: "config.json", contents: `{"something":"else"}`},
		{
			name:     "several tokens",
			filename: "config.json",
			contents: `{"api_tokens":["tok-1","tok-2"]}`,
			wantErr:  "holds 2 API tokens",
		},
		{name: "invalid json", filename: "config.json", contents: `{`, wantErr: "parsing"},
		{
			name:      "host-specific file",
			filename:  "staging-cloud-example-config.json",
			contents:  `{"api_tokens":["staging-tok"]}`,
			baseURL:   "https://staging.cloud.example/api",
			wantToken: "staging-tok",
		},
		{
			// A production login must never be sent to another host.
			name:     "default file ignored for other host",
			filename: "config.json",
			contents: `{"api_tokens":["prod-tok"]}`,
			baseURL:  "https://staging.cloud.example/api",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := setCLIHome(t)
			if tc.filename != "" {
				writeCLIConfig(t, home, tc.filename, tc.contents)
			}

			token, _, err := cliToken(tc.baseURL)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tc.wantToken {
				t.Errorf("token = %q, want %q", token, tc.wantToken)
			}
		})
	}
}
