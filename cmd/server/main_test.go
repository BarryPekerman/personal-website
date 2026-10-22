package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestNoListing(t *testing.T) {
	fsys := fstest.MapFS{
		"cv.txt":        {Data: []byte("cv")},
		"projects/a.md": {Data: []byte("a")},
	}
	h := http.StripPrefix("/api/content/", noListing(http.FileServerFS(fsys)))

	for path, want := range map[string]int{
		"/api/content/cv.txt":        http.StatusOK,
		"/api/content/projects/a.md": http.StatusOK,
		"/api/content/":              http.StatusNotFound,
		"/api/content/projects/":     http.StatusNotFound,
		"/api/content/projects":      http.StatusMovedPermanently, // to projects/, a 404
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("GET %s: %d, want %d", path, rec.Code, want)
		}
	}
}
