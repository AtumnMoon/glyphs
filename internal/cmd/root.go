// Package cmd is glyphs' command line: a Cobra command tree whose commands
// are thin wrappers over package app.
package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AtumnMoon/glyphs/internal/app"
	"github.com/AtumnMoon/glyphs/internal/config"
)

var version = "v0.1.0"

// Exit codes. 78 is EX_CONFIG from sysexits.h: systemd units can list it in
// RestartPreventExitStatus= so a broken config does not cause a restart loop.
const (
	exitOK     = 0
	exitFail   = 1
	exitUsage  = 2
	exitConfig = app.ExitConfig
)

// exitError carries a process exit code out of a command. The message, when
// there is one, is printed by Execute.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// fromCode adapts package app's int results (which print their own errors).
func fromCode(code int) error {
	if code == 0 {
		return nil
	}
	return &exitError{code: code}
}

// Main runs the CLI against the real process arguments and returns the exit code.
func Main() int { return Execute(os.Args[1:], os.Stdout, os.Stderr) }

// Execute runs the CLI with explicit arguments and streams (used by tests).
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return exitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.msg != "" {
			fmt.Fprintln(stderr, "error:", ee.msg)
		}
		return ee.code
	}
	// Anything else is a usage error raised by Cobra (unknown flag, bad args).
	fmt.Fprintln(stderr, "error:", err)
	fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", root.CommandPath())
	return exitUsage
}

func newRootCmd() *cobra.Command {
	var tab string
	var keep bool

	root := &cobra.Command{
		Use:   "glyphs",
		Short: "Kaomoji, emoji and clipboard history in your terminal",
		Long: `glyphs is a terminal UI with a clipboard-history manager, an emoji picker
and a kaomoji picker, plus a background listener that records clipboard
changes.

Run it with no command to open the UI. Start the listener with
"glyphs listen" (or run "glyphs listen --foreground" from a supervisor such
as systemd, see contrib/glyphs-listener.service).`,
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validTab(tab); err != nil {
				return err
			}
			cfg, _, err := loadConfig(cmd, false)
			if err != nil {
				return err
			}
			if keep {
				cfg.QuitAfterCopy = false
			}
			return fromCode(app.LaunchTUI(cfg, tab))
		},
	}
	root.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	root.Flags().StringVar(&tab, "tab", "", "open on a tab: clipboard, kaomoji, emoji or logs (default: config default_tab)")
	root.Flags().BoolVar(&keep, "keep", false, "keep the UI open after copying an item")
	_ = root.RegisterFlagCompletionFunc("tab", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return config.TabKeys, cobra.ShellCompDirectiveNoFileComp
	})

	root.AddCommand(
		newListenCmd(), newKillCmd(), newPauseCmd(),
		newAddCmd(), newCopyCmd(), newPasteCmd(),
		newClearCmd(), newCleanCmd(), newOutputCmd(),
		newHoldCmd(), newManCmd(),
	)
	return root
}

func validTab(tab string) error {
	if tab == "" {
		return nil
	}
	for _, k := range config.TabKeys {
		if strings.EqualFold(tab, k) {
			return nil
		}
	}
	return &exitError{code: exitFail, msg: fmt.Sprintf("unknown tab %q (use %s)", tab, strings.Join(config.TabKeys, ", "))}
}

// loadConfig reads the user's config. A config that fails to parse is
// replaced by the defaults, and the parse error is returned as cfgErr so the
// caller can decide what it means:
//
//   - strict commands (those that record clipboard data, where the defaults
//     would silently drop the user's excluded_patterns) refuse to run;
//   - the rest print a warning and carry on.
func loadConfig(cmd *cobra.Command, strict bool) (cfg config.Config, cfgErr error, err error) {
	cfg, cfgErr = config.Load()
	if cfgErr != nil {
		if strict {
			msg := cfgErr.Error()
			var ce *config.Error
			if errors.As(cfgErr, &ce) && ce.Context != "" {
				msg += "\n\n" + ce.Context
			}
			return cfg, cfgErr, &exitError{code: exitConfig, msg: msg + "\nrefusing to run with an invalid config; fix it and try again"}
		}
		if cmd.Name() != "glyphs" { // the TUI shows problems itself (Logs tab)
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", cfgErr)
		}
	}
	if cmd.Name() != "glyphs" {
		for _, w := range cfg.Warnings {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
		}
	}
	return cfg, cfgErr, nil
}

// cfgRun builds a RunE that loads the config first.
func cfgRun(strict bool, fn func(cmd *cobra.Command, cfg config.Config, args []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, _, err := loadConfig(cmd, strict)
		if err != nil {
			return err
		}
		return fn(cmd, cfg, args)
	}
}
