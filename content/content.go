// Package content embeds the portfolio files that visitors browse in the
// virtual shell. Edit the files next to this one; they are compiled into
// the binary, so a rebuild picks up changes.
package content

import (
	"embed"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// FS holds the visitor-facing files; .motd is the login banner
//
//go:embed .motd cv.txt contact_info.md projects
var FS embed.FS

// projectOrder is the order the front page lists projects in. A write-up
// missing from it is still listed, after these, alphabetically.
var projectOrder = []string{
	"the-handover",
	"oke-version-guard",
	"terraform-aws-chatops",
	"k8s-cluster-vagrant",
	"portfolio-terminal",
}

// Project is one write-up in projects/, as the front page lists it
type Project struct {
	File  string `json:"file"`  // "the-handover.md"
	Name  string `json:"name"`  // "the-handover"
	Link  string `json:"link"`  // the first URL line under the title: the repo or the live site
	Label string `json:"label"` // "repo" for a GitHub link, else "live"
	Gist  string `json:"gist"`  // the first paragraph that isn't the title or the link
}

// Projects reads every write-up in fsys's projects/
func Projects(fsys fs.FS) ([]Project, error) {
	files, err := fs.Glob(fsys, "projects/*.md")
	if err != nil {
		return nil, err
	}
	var list []Project
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		list = append(list, parseProject(path.Base(f), string(data)))
	}
	rank := func(p Project) int {
		if i := slices.Index(projectOrder, p.Name); i >= 0 {
			return i
		}
		return len(projectOrder)
	}
	slices.SortStableFunc(list, func(a, b Project) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.Name, b.Name)
	})
	return list, nil
}

func parseProject(file, text string) Project {
	p := Project{File: file, Name: strings.TrimSuffix(file, ".md")}
	var gist []string
	for i, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case i == 0 && strings.HasPrefix(t, "# "):
			continue
		case strings.HasPrefix(t, "https://") && !strings.Contains(t, " "):
			if p.Link == "" && len(gist) == 0 {
				p.Link = t
			}
			continue
		case t == "":
			if len(gist) > 0 {
				p.Gist = strings.Join(gist, " ")
				return label(p)
			}
			continue
		}
		gist = append(gist, t)
	}
	p.Gist = strings.Join(gist, " ")
	return label(p)
}

func label(p Project) Project {
	if p.Link != "" {
		p.Label = "live"
		if strings.HasPrefix(p.Link, "https://github.com/") {
			p.Label = "repo"
		}
	}
	return p
}
