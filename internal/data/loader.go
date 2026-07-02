package data

import (
	"encoding/json"
	"fmt"
	"os"
)

// kaomojiFile mirrors the top-level shape of kaomoji.json: {"kaomoji": [...]}.
type kaomojiFile struct {
	Kaomoji []Kaomoji `json:"kaomoji"`
}

// LoadEmoji loads emoji data from the given JSON file path. If path is
// empty, the embedded default dataset (emoji.json) is used instead.
func LoadEmoji(path string) ([]Emoji, error) {
	raw, err := readJSON(path, embeddedEmojiJSON)
	if err != nil {
		return nil, fmt.Errorf("load emoji: %w", err)
	}

	var emojis []Emoji
	if err := json.Unmarshal(raw, &emojis); err != nil {
		return nil, fmt.Errorf("load emoji: parse json: %w", err)
	}
	return emojis, nil
}

// LoadKaomoji loads kaomoji data from the given JSON file path. If path is
// empty, the embedded default dataset (kaomoji.json) is used instead.
func LoadKaomoji(path string) ([]Kaomoji, error) {
	raw, err := readJSON(path, embeddedKaomojiJSON)
	if err != nil {
		return nil, fmt.Errorf("load kaomoji: %w", err)
	}

	var file kaomojiFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("load kaomoji: parse json: %w", err)
	}
	return file.Kaomoji, nil
}

// readJSON returns the embedded bytes when path is empty, otherwise it
// reads path from disk.
func readJSON(path string, embedded []byte) ([]byte, error) {
	if path == "" {
		return embedded, nil
	}
	return os.ReadFile(path)
}
