package content

import (
	"testing"
	"testing/fstest"
)

func TestProjects(t *testing.T) {
	fsys := fstest.MapFS{
		"projects/the-handover.md": {Data: []byte("# the-handover\n\nhttps://play.barrypekerman.com\n\nA CTF game:\nreal shells.\n\nMore.\n")},
		"projects/zeta.md":         {Data: []byte("# Zeta\n\nhttps://github.com/BarryPekerman/zeta\n\nNot in the order list.\n")},
		"projects/alpha.md":        {Data: []byte("# alpha\n\nNo link at all.\n")},
		"projects/notes.txt":       {Data: []byte("not a write-up")},
	}
	list, err := Projects(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := []Project{
		{File: "the-handover.md", Name: "the-handover", Link: "https://play.barrypekerman.com", Label: "live", Gist: "A CTF game: real shells."},
		{File: "alpha.md", Name: "alpha", Gist: "No link at all."},
		{File: "zeta.md", Name: "zeta", Link: "https://github.com/BarryPekerman/zeta", Label: "repo", Gist: "Not in the order list."},
	}
	if len(list) != len(want) {
		t.Fatalf("got %d projects: %+v", len(list), list)
	}
	for i := range want {
		if list[i] != want[i] {
			t.Errorf("project %d: %+v, want %+v", i, list[i], want[i])
		}
	}
}

// The real write-ups: every one listed, each with a link and a summary
func TestEmbeddedProjects(t *testing.T) {
	list, err := Projects(FS)
	if err != nil {
		t.Fatal(err)
	}
	files, _ := FS.ReadDir("projects")
	if len(list) != len(files) {
		t.Errorf("%d projects listed, %d files", len(list), len(files))
	}
	for _, p := range list {
		if p.Link == "" || p.Gist == "" {
			t.Errorf("%s: link %q, gist %q", p.File, p.Link, p.Gist)
		}
	}
	if list[0].Name != "the-handover" {
		t.Errorf("first project %s, want the-handover", list[0].Name)
	}
}
