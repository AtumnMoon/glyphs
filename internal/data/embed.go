package data

import _ "embed"

// Run to fetch fresh sources and parse it as tab separated txt file
//go:generate go run ../../tools/fetchdata -out sources
//go:generate go run ../../tools/gensymbols -src sources -out .

// embeddedEmojiJSON and embeddedKaomojiJSON hold the bundled default
// datasets, embedded into the binary at build time. They are used by
// LoadEmoji/LoadKaomoji (and the New* constructors) whenever no external
// path is supplied.

//go:embed emoji.json
var embeddedEmojiJSON []byte

//go:embed kaomoji.json
var embeddedKaomojiJSON []byte

//go:embed unicode.txt.gz
var unicodeGz []byte

//go:embed nerd.txt.gz
var nerdGz []byte
