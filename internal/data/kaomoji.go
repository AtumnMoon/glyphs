package data

import (
	"math/rand"
	"sort"
	"strings"
)

// KaomojiStore holds a loaded set of kaomoji and provides query helpers.
// Zero value is not usable; construct with NewKaomojiStore.
type KaomojiStore struct {
	items         []Kaomoji
	categoryIndex map[string][]Kaomoji // lowercased category -> kaomoji
}

// NewKaomojiStore builds a KaomojiStore from the given JSON file path.
// Pass "" to use the embedded default dataset.
func NewKaomojiStore(path string) (*KaomojiStore, error) {
	items, err := LoadKaomoji(path)
	if err != nil {
		return nil, err
	}
	return newKaomojiStore(items), nil
}

func newKaomojiStore(items []Kaomoji) *KaomojiStore {
	s := &KaomojiStore{
		items:         items,
		categoryIndex: make(map[string][]Kaomoji),
	}
	for _, k := range items {
		key := strings.ToLower(k.Category)
		s.categoryIndex[key] = append(s.categoryIndex[key], k)
	}
	return s
}

// All returns every kaomoji in the store, in their original order.
func (s *KaomojiStore) All() []Kaomoji {
	return s.items
}

// Count returns the number of kaomoji in the store.
func (s *KaomojiStore) Count() int {
	return len(s.items)
}

// Categories returns the sorted list of distinct categories present
// in the store.
func (s *KaomojiStore) Categories() []string {
	out := make([]string, 0, len(s.categoryIndex))
	for c := range s.categoryIndex {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// ByCategory returns all kaomoji in the given category (case-insensitive).
func (s *KaomojiStore) ByCategory(category string) []Kaomoji {
	return s.categoryIndex[strings.ToLower(category)]
}

// Search returns all kaomoji whose category contains query, or whose
// value equals/contains query (case-insensitive).
func (s *KaomojiStore) Search(query string) []Kaomoji {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []Kaomoji
	for _, k := range s.items {
		if strings.Contains(strings.ToLower(k.Category), q) || strings.Contains(k.Value, query) {
			out = append(out, k)
		}
	}
	return out
}

// Random returns a random kaomoji from the store. The second return value
// is false if the store is empty.
func (s *KaomojiStore) Random() (Kaomoji, bool) {
	if len(s.items) == 0 {
		return Kaomoji{}, false
	}
	return s.items[rand.Intn(len(s.items))], true
}

// RandomByCategory returns a random kaomoji from the given category.
// The second return value is false if the category has no entries.
func (s *KaomojiStore) RandomByCategory(category string) (Kaomoji, bool) {
	items := s.ByCategory(category)
	if len(items) == 0 {
		return Kaomoji{}, false
	}
	return items[rand.Intn(len(items))], true
}
