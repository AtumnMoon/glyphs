package data

import (
	"math/rand"
	"sort"
	"strings"
)

// EmojiStore holds a loaded set of emoji and provides query helpers.
// Zero value is not usable; construct with NewEmojiStore.
type EmojiStore struct {
	items     []Emoji
	byHexcode map[string]Emoji
	tagIndex  map[string][]Emoji // lowercased tag -> emojis
}

// NewEmojiStore builds an EmojiStore from the given JSON file path.
// Pass "" to use the embedded default dataset.
func NewEmojiStore(path string) (*EmojiStore, error) {
	items, err := LoadEmoji(path)
	if err != nil {
		return nil, err
	}
	return newEmojiStore(items), nil
}

func newEmojiStore(items []Emoji) *EmojiStore {
	s := &EmojiStore{
		items:     items,
		byHexcode: make(map[string]Emoji, len(items)),
		tagIndex:  make(map[string][]Emoji),
	}
	for _, e := range items {
		s.byHexcode[e.Hexcode] = e
		for _, t := range e.Tags {
			key := strings.ToLower(t)
			s.tagIndex[key] = append(s.tagIndex[key], e)
		}
	}
	return s
}

// All returns every emoji in the store, in their original order.
func (s *EmojiStore) All() []Emoji {
	return s.items
}

// Count returns the number of emoji in the store.
func (s *EmojiStore) Count() int {
	return len(s.items)
}

// ByHexcode returns the emoji with the given hexcode.
func (s *EmojiStore) ByHexcode(hexcode string) (Emoji, bool) {
	e, ok := s.byHexcode[hexcode]
	return e, ok
}

// ByGroup returns all emoji belonging to the given numeric group.
func (s *EmojiStore) ByGroup(group int) []Emoji {
	var out []Emoji
	for _, e := range s.items {
		if e.Group != nil && *e.Group == group {
			out = append(out, e)
		}
	}
	return out
}

// ByGroupName returns all emoji whose group name matches name
// (case-insensitive), e.g. "Animals & Nature".
func (s *EmojiStore) ByGroupName(name string) []Emoji {
	var out []Emoji
	for _, e := range s.items {
		if strings.EqualFold(e.GroupName(), name) {
			out = append(out, e)
		}
	}
	return out
}

// Groups returns the sorted list of distinct numeric group IDs present
// in the store.
func (s *EmojiStore) Groups() []int {
	seen := make(map[int]bool)
	for _, e := range s.items {
		if e.Group != nil {
			seen[*e.Group] = true
		}
	}
	out := make([]int, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Ints(out)
	return out
}

// GroupNames returns the sorted list of distinct group names present
// in the store.
func (s *EmojiStore) GroupNames() []string {
	seen := make(map[string]bool)
	for _, g := range s.Groups() {
		seen[groupNames[g]] = true
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Tags returns the sorted list of distinct tags present across all emoji.
func (s *EmojiStore) Tags() []string {
	out := make([]string, 0, len(s.tagIndex))
	for t := range s.tagIndex {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// ByTag returns all emoji tagged with the given tag (case-insensitive).
func (s *EmojiStore) ByTag(tag string) []Emoji {
	return s.tagIndex[strings.ToLower(tag)]
}

// WithSkinTones returns all emoji that have skin-tone variants.
func (s *EmojiStore) WithSkinTones() []Emoji {
	var out []Emoji
	for _, e := range s.items {
		if e.HasSkinTones() {
			out = append(out, e)
		}
	}
	return out
}

// Search returns all emoji whose label or tags contain query
// (case-insensitive substring match).
func (s *EmojiStore) Search(query string) []Emoji {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []Emoji
	for _, e := range s.items {
		if strings.Contains(strings.ToLower(e.Label), q) {
			out = append(out, e)
			continue
		}
		for _, t := range e.Tags {
			if strings.Contains(strings.ToLower(t), q) {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// Random returns a random emoji from the store. The second return value
// is false if the store is empty.
func (s *EmojiStore) Random() (Emoji, bool) {
	if len(s.items) == 0 {
		return Emoji{}, false
	}
	return s.items[rand.Intn(len(s.items))], true
}
