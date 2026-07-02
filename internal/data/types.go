package data

import "encoding/json"

// Emoji represents a single emoji entry from emoji.json.
// Some entries (e.g. flag/regional-indicator letters) only carry
// Hexcode, Label, and Unicode, therefore Group/Order/Tags may be absent.
type Emoji struct {
	Group    *int         `json:"group,omitempty"`
	Hexcode  string       `json:"hexcode"`
	Label    string       `json:"label"`
	Order    *int         `json:"order,omitempty"`
	Tags     []string     `json:"tags,omitempty"`
	Unicode  string       `json:"unicode"`
	Emoticon stringOrList `json:"emoticon,omitempty"`
	Skins    []Emoji      `json:"skins,omitempty"`
}

// stringOrList unmarshals a JSON field that may be either a single string
// or an array of strings (as emoji.json's "emoticon" field is) into a
// normalized []string.
type stringOrList []string

func (s *stringOrList) UnmarshalJSON(b []byte) error {
	var single string
	if err := json.Unmarshal(b, &single); err == nil {
		if single == "" {
			*s = nil
		} else {
			*s = []string{single}
		}
		return nil
	}
	var list []string
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	*s = list
	return nil
}

// GroupName returns the human-readable Unicode CLDR group name for the
// emoji's Group field, or "" if the emoji has no group.
func (e Emoji) GroupName() string {
	if e.Group == nil {
		return ""
	}
	return groupNames[*e.Group]
}

// HasSkinTones reports whether this emoji has skin-tone variants.
func (e Emoji) HasSkinTones() bool {
	return len(e.Skins) > 0
}

// Kaomoji represents a single kaomoji entry from kaomoji.json.
type Kaomoji struct {
	Category string `json:"category"`
	Value    string `json:"value"`
}

// groupNames maps the numeric "group" field found in emoji.json to the
// standard Unicode CLDR emoji group names.
var groupNames = map[int]string{
	0: "Smileys & Emotion",
	1: "People & Body",
	2: "Component",
	3: "Animals & Nature",
	4: "Food & Drink",
	5: "Travel & Places",
	6: "Activities",
	7: "Objects",
	8: "Symbols",
	9: "Flags",
}
