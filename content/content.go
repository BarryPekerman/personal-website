// Package content embeds the portfolio files that visitors browse in the
// virtual shell. Edit the files next to this one; they are compiled into
// the binary, so a rebuild picks up changes.
package content

import "embed"

// FS holds the visitor-facing files; .motd is the login banner
//
//go:embed .motd cv.txt contact_info.md projects
var FS embed.FS
