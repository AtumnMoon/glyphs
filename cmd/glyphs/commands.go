package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/AtumnMoon/glyphs/internal/app"
	"github.com/AtumnMoon/glyphs/internal/clipboard"
	"github.com/AtumnMoon/glyphs/internal/config"
)

func newListenCmd() *cobra.Command {
	var foreground bool
	c := &cobra.Command{
		Use:   "listen",
		Short: "Record clipboard changes into the history",
		Long: `Start the clipboard listener, replacing any listener that is already running.

By default the listener detaches and runs in the background. With
--foreground it runs in the current process until interrupted: use that for
debugging, and for any supervisor (systemd, runit, ...) that tracks the
process it starts. A detached listener would be seen as having exited, and
systemd would kill it along with the rest of the service's control group.

The listener refuses to start when the config file is invalid: falling back to
the defaults would drop your excluded_patterns and record what you excluded.
It then exits with status 78 (EX_CONFIG).`,
		Args: cobra.NoArgs,
		RunE: cfgRun(true, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			if foreground {
				_, _ = app.Kill(cfg)
				return fromErr(app.RunListener(cfg))
			}
			if err := app.LaunchListener(cfg); err != nil {
				return fromErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "glyphs listener started (log: %s)\n", cfg.LogFilePath)
			return nil
		}),
	}
	c.Flags().BoolVarP(&foreground, "foreground", "f", false, "run in the current process instead of detaching")
	return c
}

func newKillCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill",
		Short: "Stop the background listener",
		Args:  cobra.NoArgs,
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			stopped, err := app.Kill(cfg)
			if err != nil {
				return &exitError{code: exitFail, msg: err.Error()}
			}
			if stopped {
				fmt.Fprintln(cmd.OutOrStdout(), "listener stopped")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "no listener is running")
			}
			return nil
		}),
	}
}

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "pause <duration|off>",
		Short:   "Pause recording for a while, or resume it",
		Example: "  glyphs pause 5m\n  glyphs pause off",
		Long:    "Pause recording for a duration such as 30s, 5m or 1h. \"off\" (or \"resume\") resumes it.",
		Args:    cobra.ExactArgs(1),
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, args []string) error {
			return handlePause(cmd, cfg, args[0])
		}),
	}
}

func handlePause(cmd *cobra.Command, cfg config.Config, arg string) error {
	out, errw := cmd.OutOrStdout(), cmd.ErrOrStderr()
	if a := strings.ToLower(arg); a == "off" || a == "resume" {
		if err := app.Resume(cfg); err != nil {
			return fromErr(err)
		}
		fmt.Fprintln(out, "recording resumed")
		return nil
	}
	d, err := app.ParseDuration(arg)
	if err != nil {
		return &exitError{code: exitFail, msg: fmt.Sprintf("invalid duration %q: %v\nExample: glyphs pause 5m   (or \"glyphs pause off\" to resume)", arg, err)}
	}
	if !app.IsListenerRunning(cfg) {
		fmt.Fprintln(errw, "note: no listener is running, so there is nothing to pause yet (start one with: glyphs listen)")
	}
	if err := app.Pause(cfg, d); err != nil {
		return fromErr(err)
	}
	fmt.Fprintf(out, "recording paused for %s\n", d)
	return nil
}

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [text...]",
		Short: "Add text to the history without copying it",
		Long:  "Add the arguments (or piped stdin) to the history without touching the system clipboard.\nLike the listener, it refuses to run with an invalid config.",
		RunE: cfgRun(true, func(cmd *cobra.Command, cfg config.Config, args []string) error {
			added, err := app.Add(cfg, args)
			if err == nil && !added {
				fmt.Fprintln(cmd.ErrOrStderr(), "nothing added (empty or excluded)")
			}
			return fromErr(err)
		}),
	}
}

func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy [text...]",
		Short: "Copy text to the system clipboard",
		Long:  "Copy the arguments (or piped stdin) to the system clipboard. A running listener records it.",
		RunE: cfgRun(false, func(_ *cobra.Command, cfg config.Config, args []string) error {
			return fromErr(app.Copy(cfg, args))
		}),
	}
}

func newPasteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "paste",
		Short: "Print the current clipboard text",
		Args:  cobra.NoArgs,
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			text, err := app.Paste(cfg)
			if err != nil {
				return fromErr(err)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return fromErr(err)
		}),
	}
}

func newClearCmd() *cobra.Command {
	var all, images, text bool
	c := &cobra.Command{
		Use:   "clear",
		Short: "Remove entries from the history",
		Long:  "Remove every history entry except the pinned ones. --all, --images and --text widen or narrow that, and include pinned entries.",
		Args:  cobra.NoArgs,
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			scope := clipboard.ClearUnpinned
			switch {
			case all:
				scope = clipboard.ClearAll
			case images:
				scope = clipboard.ClearImages
			case text:
				scope = clipboard.ClearText
			}
			n, err := app.Clear(cfg, scope)
			if err != nil {
				return fromErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d entries\n", n)
			return nil
		}),
	}
	c.Flags().BoolVar(&all, "all", false, "remove the entire history, pinned entries included")
	c.Flags().BoolVar(&images, "images", false, "remove all images, pinned included")
	c.Flags().BoolVar(&text, "text", false, "remove all text entries, pinned included")
	c.MarkFlagsMutuallyExclusive("all", "images", "text")
	return c
}

func newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Sanitise text entries and remove orphaned images",
		Args:  cobra.NoArgs,
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			e, f, err := app.Clean(cfg)
			if err != nil {
				return fromErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d entries, %d orphaned files\n", e, f)
			return nil
		}),
	}
}

func newOutputCmd() *cobra.Command {
	var unescaped bool
	c := &cobra.Command{
		Use:   "output",
		Short: "Print every text entry, newest first",
		Long:  "Print every text entry, newest first, one per line with newlines escaped. --unescaped prints entries verbatim instead.",
		Args:  cobra.NoArgs,
		RunE: cfgRun(false, func(cmd *cobra.Command, cfg config.Config, _ []string) error {
			mode := "raw"
			if unescaped {
				mode = "unescaped"
			}
			return fromErr(app.OutputAll(cmd.OutOrStdout(), cfg, mode))
		}),
	}
	c.Flags().BoolVar(&unescaped, "unescaped", false, "print entries verbatim instead of one escaped line each")
	return c
}

// newHoldCmd is internal: copies hand their payload to a "holder" process that
// owns the X11/Wayland selection until something else is copied.
func newHoldCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hold <text|image>",
		Short:  "Internal: own the clipboard with stdin data until replaced",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return fromErr(app.RunHolder(args[0]))
		},
	}
}

// newManCmd writes the man pages; the release build runs it.
func newManCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "man <dir>",
		Short:  "Generate man pages into a directory",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(args[0], 0o755); err != nil {
				return &exitError{code: exitFail, msg: err.Error()}
			}
			hdr := &doc.GenManHeader{Title: "GLYPHS", Section: "1", Source: "glyphs " + version}
			if err := doc.GenManTree(cmd.Root(), hdr, args[0]); err != nil {
				return &exitError{code: exitFail, msg: err.Error()}
			}
			return nil
		},
	}
}
