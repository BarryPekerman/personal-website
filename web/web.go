// Package web embeds the front end: the page templates and the static files
// they load. Like content/, it is compiled into the binary.
package web

import "embed"

// FS holds templates/ (the pages, rendered with the asset version) and
// static/ (served under /static/)
//
//go:embed templates static
var FS embed.FS
