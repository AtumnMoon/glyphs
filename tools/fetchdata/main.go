// Command fetchdata downloads the Unicode and Nerd Fonts source files that
// the symbol-data generator reads. Run it via `go generate ./internal/data`.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type source struct {
	name, url string
	validate  func([]byte) error
}

// Floating URLs; pin them (see notes) for reproducible builds.
var sources = []source{
	{"Blocks.txt", "https://www.unicode.org/Public/UNIDATA/Blocks.txt", validateBlocks},
	{"UnicodeData.txt", "https://www.unicode.org/Public/UNIDATA/UnicodeData.txt", validateUnicodeData},
	{"glyphnames.json", "https://raw.githubusercontent.com/ryanoasis/nerd-fonts/refs/heads/master/glyphnames.json", validateGlyphnames},
}

func main() {
	out := flag.String("out", "internal/data/sources", "output directory")
	force := flag.Bool("force", false, "re-download even if the file exists")
	update := flag.Bool("update-lock", false, "accept changed upstream content and rewrite sources.lock (implies -force)")
	flag.Parse()
	if *update {
		*force = true
	}
	if err := run(*out, *force, *update); err != nil {
		fmt.Fprintln(os.Stderr, "fetchdata:", err)
		os.Exit(1)
	}
}

func run(dir string, force, update bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "sources.lock")
	lock := readLock(lockPath)
	client := &http.Client{Timeout: 2 * time.Minute}
	sums := make(map[string]string, len(sources))

	for _, s := range sources {
		dst := filepath.Join(dir, s.name)
		data, downloaded, err := load(client, s.url, dst, force)
		if err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
		if err := s.validate(data); err != nil {
			return fmt.Errorf("%s: invalid content: %w", s.name, err)
		}
		h := sha256.Sum256(data)
		sum := hex.EncodeToString(h[:])
		if want, ok := lock[s.name]; ok && want != sum && !update {
			return fmt.Errorf("%s: sha256 %.12s != sources.lock %.12s; upstream changed or file is corrupt, rerun with -update-lock to accept", s.name, sum, want)
		}
		if downloaded {
			if err := writeAtomic(dst, data); err != nil {
				return fmt.Errorf("%s: %w", s.name, err)
			}
		}
		sums[s.name] = sum
		state := "cached"
		if downloaded {
			state = "fetched"
		}
		fmt.Printf("%-8s %-16s %.12s\n", state, s.name, sum)
	}
	if update || len(lock) == 0 {
		return writeLock(lockPath, sums)
	}
	return nil
}

// load returns the file from disk unless force is set or it is missing.
func load(c *http.Client, url, dst string, force bool) (data []byte, downloaded bool, err error) {
	if !force {
		if b, err := os.ReadFile(dst); err == nil {
			return b, false, nil
		}
	}
	resp, err := c.Get(url)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	return data, true, err
}

func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // no-op after a successful rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func readLock(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			m[f[1]] = f[0]
		}
	}
	return m
}

func writeLock(path string, sums map[string]string) error {
	var sb strings.Builder
	for _, s := range sources { // stable order
		fmt.Fprintf(&sb, "%s  %s\n", sums[s.name], s.name)
	}
	return writeAtomic(path, []byte(sb.String()))
}

func validateBlocks(b []byte) error {
	if !bytes.Contains(b, []byte("0000..007F; Basic Latin")) {
		return fmt.Errorf("missing Basic Latin block")
	}
	return nil
}

func validateUnicodeData(b []byte) error {
	if !bytes.HasPrefix(b, []byte("0000;<control>;")) {
		return fmt.Errorf("unexpected first line")
	}
	if n := bytes.Count(b, []byte("\n")); n < 30000 {
		return fmt.Errorf("only %d lines, truncated?", n)
	}
	return nil
}

func validateGlyphnames(b []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if len(m) < 1000 {
		return fmt.Errorf("only %d entries, truncated?", len(m))
	}
	return nil
}
