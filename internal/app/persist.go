package app

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"golang.design/x/clipboard"
)

// On X11/Wayland-via-XWayland the clipboard has no owner but the process that
// copied: when it exits, the content disappears.
//
// Clipse avoids this with a long-lived process with that as example.
//
// Glyphs does the same by handing every copy to a tiny detached
// "holder" (glyphs -hold <fmt>) that owns the selection until
// something else is copied. macOS and Windows keep clipboard
// data in the OS, so they write directly.

func needsHolder() bool { return runtime.GOOS != "darwin" && runtime.GOOS != "windows" }

func formatOf(name string) (clipboard.Format, bool) {
	switch name {
	case "text":
		return clipboard.FmtText, true
	case "image":
		return clipboard.FmtImage, true
	}
	return 0, false
}

// PersistText copies text to the system clipboard so it outlives this process.
func PersistText(text string) error { return persist("text", []byte(text)) }

// PersistImage copies PNG data to the system clipboard so it outlives this process.
func PersistImage(png []byte) error { return persist("image", png) }

func persist(format string, data []byte) error {
	f, _ := formatOf(format)
	if !needsHolder() {
		if err := clipboard.Init(); err != nil {
			return fmt.Errorf("clipboard unavailable: %w", err)
		}
		clipboard.Write(f, data)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "-hold", format)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start clipboard holder: %w", err)
	}
	_ = cmd.Process.Release()

	// The holder prints one line once it owns the clipboard ("ok") or fails.
	line := make(chan string, 1)
	go func() {
		s, _ := bufio.NewReader(out).ReadString('\n')
		line <- s
	}()
	select {
	case s := <-line:
		if s == "ok\n" {
			return nil
		}
		return fmt.Errorf("clipboard holder failed: %s", errBuf.String())
	case <-time.After(3 * time.Second):
		return fmt.Errorf("clipboard holder did not respond")
	}
}

// RunHolder is the body of `glyphs -hold <text|image>`: read the payload from
// stdin, take ownership of the clipboard, and stay alive until it is replaced.
func RunHolder(format string) int {
	f, ok := formatOf(format)
	if !ok {
		fmt.Fprintln(os.Stderr, "hold: format must be text or image")
		return 1
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hold:", err)
		return 1
	}
	if err := clipboard.Init(); err != nil {
		fmt.Fprintln(os.Stderr, "clipboard unavailable:", err)
		return 1
	}
	replaced := clipboard.Write(f, data)
	fmt.Println("ok")
	os.Stdout.Close() // let the parent's read finish
	<-replaced        // closed when another program takes the clipboard
	return 0
}
