package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run executes the CLI against a throw-away config dir.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runWithConfig(t, "", args...)
}

// runWithConfig is run with a config.toml (when body is not empty).
func runWithConfig(t *testing.T, body string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GLYPHS_CONFIG_DIR", dir)
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errb bytes.Buffer
	code = Execute(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestPause(t *testing.T) {
	for _, ok := range [][]string{{"pause", "5m"}, {"pause", "off"}, {"pause", "resume"}, {"pause", "90"}} {
		if code, _, se := run(t, ok...); code != 0 {
			t.Errorf("%v: code %d, stderr %q", ok, code, se)
		}
	}
	for _, bad := range []string{"abc", "-5m", "0"} {
		// "-5m" must be treated as a value, not a flag.
		if code, _, _ := run(t, "pause", "--", bad); code != exitFail {
			t.Errorf("pause %q: code %d, want %d", bad, code, exitFail)
		}
	}
	if code, _, _ := run(t, "pause"); code != exitUsage {
		t.Errorf("pause without argument: code %d, want %d", code, exitUsage)
	}
}

func TestVersion(t *testing.T) {
	for _, flag := range []string{"--version", "-v"} {
		code, out, _ := run(t, flag)
		if code != 0 || !strings.Contains(out, version) {
			t.Errorf("%s: code %d, output %q", flag, code, out)
		}
	}
}

func TestUnknownCommandAndFlag(t *testing.T) {
	if code, _, se := run(t, "frobnicate"); code != exitUsage || !strings.Contains(se, "frobnicate") {
		t.Errorf("unknown command: code %d, stderr %q", code, se)
	}
	if code, _, _ := run(t, "--nope"); code != exitUsage {
		t.Errorf("unknown flag: code %d", code)
	}
	if code, _, _ := run(t, "--tab", "bogus"); code != exitFail {
		t.Errorf("unknown tab: code %d", code)
	}
}

func TestClearFlagsAreExclusive(t *testing.T) {
	if code, _, _ := run(t, "clear", "--all", "--text"); code != exitUsage {
		t.Errorf("clear --all --text: code %d, want %d", code, exitUsage)
	}
	// (package app prints "removed N entries" straight to os.Stdout)
	if code, _, se := run(t, "clear", "--images"); code != 0 {
		t.Errorf("clear --images: code %d, err %q", code, se)
	}
}

func TestHelpListsCommandsButNotInternalOnes(t *testing.T) {
	_, out, _ := run(t, "--help")
	for _, want := range []string{"listen", "kill", "pause", "add", "copy", "paste", "clear", "clean", "output", "--tab", "--keep"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
	for _, hidden := range []string{"hold", "  man "} {
		if strings.Contains(out, hidden) {
			t.Errorf("help should not list internal command %q", hidden)
		}
	}
}

func TestListenHelpExplainsForeground(t *testing.T) {
	_, out, _ := run(t, "listen", "--help")
	for _, want := range []string{"--foreground", "systemd", "78"} {
		if !strings.Contains(out, want) {
			t.Errorf("listen --help is missing %q", want)
		}
	}
}

func TestManPages(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "man")
	if code, _, se := run(t, "man", dir); code != 0 {
		t.Fatalf("man: code %d, stderr %q", code, se)
	}
	for _, f := range []string{"glyphs.1", "glyphs-listen.1", "glyphs-pause.1"} {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil || len(b) == 0 {
			t.Errorf("%s was not generated: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "glyphs-hold.1")); err == nil {
		t.Error("the internal hold command must not get a man page")
	}
}

func TestCompletionScripts(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		code, out, _ := run(t, "completion", sh)
		if code != 0 || len(out) < 100 {
			t.Errorf("completion %s: code %d, %d bytes", sh, code, len(out))
		}
	}
}

// The safety property: a config that cannot be read must stop everything that
// records clipboard data, because the defaults it would fall back to have no
// excluded_patterns.
func TestRecordingCommandsRefuseAnInvalidConfig(t *testing.T) {
	broken := "excluded_patterns = [\"^sk-\"\nmax_history = 5\n" // missing ]
	for _, args := range [][]string{{"listen"}, {"listen", "--foreground"}, {"add", "hunter2"}} {
		code, _, se := runWithConfig(t, broken, args...)
		if code != exitConfig {
			t.Errorf("%v: code %d, want %d", args, code, exitConfig)
		}
		for _, want := range []string{"config.toml:", "refusing to run"} {
			if !strings.Contains(se, want) {
				t.Errorf("%v: stderr should contain %q, got:\n%s", args, want, se)
			}
		}
	}
}

func TestListenRefusesAnInvalidExcludePattern(t *testing.T) {
	// Valid TOML, invalid regexp: caught before anything is spawned.
	for _, args := range [][]string{{"listen"}, {"listen", "--foreground"}, {"add", "x"}} {
		if code, _, _ := runWithConfig(t, "excluded_patterns = [\"(unclosed\"]\n", args...); code != exitConfig {
			t.Errorf("%v: code %d, want %d", args, code, exitConfig)
		}
	}
}

func TestReadOnlyCommandsOnlyWarnAboutAnInvalidConfig(t *testing.T) {
	code, _, se := runWithConfig(t, "max_history = \n", "output")
	if code != 0 {
		t.Errorf("output must still work, code %d", code)
	}
	if !strings.Contains(se, "warning:") || !strings.Contains(se, "config.toml:1") {
		t.Errorf("want a warning that locates the problem, got %q", se)
	}
}

func TestUnknownOptionsAreReportedByTheCLI(t *testing.T) {
	code, _, se := runWithConfig(t, "maxHistory = 3\n", "output")
	if code != 0 || !strings.Contains(se, `did you mean "max_history"`) {
		t.Errorf("code %d, stderr %q", code, se)
	}
}

func TestAddRecordsWithAValidConfig(t *testing.T) {
	if code, _, se := runWithConfig(t, "excluded_patterns = [\"^sk-\"]\n", "add", "hello"); code != 0 {
		t.Errorf("add: code %d, stderr %q", code, se)
	}
}
