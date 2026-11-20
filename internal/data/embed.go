package data

import _ "embed"

// embeddedEmojiJSON and embeddedKaomojiJSON hold the bundled default
// datasets, embedded into the binary at build time. They are used by
// LoadEmoji/LoadKaomoji (and the New* constructors) whenever no external
// path is supplied.

// The other one is run at compilation to fetch fresh sources

//go:embed emoji.json
var embeddedEmojiJSON []byte

//go:embed kaomoji.json
var embeddedKaomojiJSON []byte

//go:generate go run ../../tools/fetchdata -out sources
