package sidecar

import "testing"

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		file string
		want Class
	}{
		{"Alien (1979)/movie.nfo", Class{Kind: Nfo, Scope: ScopeFolder}},
		{"Dark/tvshow.nfo", Class{Kind: Nfo, Scope: ScopeShow}},
		{"Dark/Season 1/Dark S01E01.nfo", Class{Kind: Nfo, Scope: ScopeVideo, Base: "Dark S01E01"}},
		{"Alien (1979)/Alien (1979).NFO", Class{Kind: Nfo, Scope: ScopeVideo, Base: "Alien (1979)"}},
		{"poster.jpg", Class{Kind: Poster, Scope: ScopeFolder}},
		{"Folder.JPG", Class{Kind: Poster, Scope: ScopeFolder}},
		{"cover.png", Class{Kind: Poster, Scope: ScopeFolder}},
		{"fanart.jpg", Class{Kind: Backdrop, Scope: ScopeFolder}},
		{"backdrop.webp", Class{Kind: Backdrop, Scope: ScopeFolder}},
		{"background.jpeg", Class{Kind: Backdrop, Scope: ScopeFolder}},
		{"clearlogo.png", Class{Kind: Logo, Scope: ScopeFolder}},
		{"logo.png", Class{Kind: Logo, Scope: ScopeFolder}},
		{"clearart.png", Class{Kind: Clearart, Scope: ScopeFolder}},
		{"banner.jpg", Class{Kind: Banner, Scope: ScopeFolder}},
		{"landscape.jpg", Class{Kind: Thumb, Scope: ScopeFolder}},
		{"season01-poster.jpg", Class{Kind: Poster, Scope: ScopeSeason, Season: 1}},
		{"Season12-Poster.png", Class{Kind: Poster, Scope: ScopeSeason, Season: 12}},
		{"season-specials-poster.jpg", Class{Kind: Poster, Scope: ScopeSeason, Season: 0}},
		{"Alien (1979)-poster.jpg", Class{Kind: Poster, Scope: ScopeVideo, Base: "Alien (1979)"}},
		{"Alien (1979)-fanart.jpg", Class{Kind: Backdrop, Scope: ScopeVideo, Base: "Alien (1979)"}},
		{"Dark S01E01-thumb.jpg", Class{Kind: Thumb, Scope: ScopeVideo, Base: "Dark S01E01"}},
		{"Spider-Man (2002)-poster.jpg", Class{Kind: Poster, Scope: ScopeVideo, Base: "Spider-Man (2002)"}},
		{"Spider-Man (2002).jpg", Class{}},
		{"screenshot.jpg", Class{}},
		{"poster.gif", Class{}},
		{"notes.txt", Class{}},
	} {
		if got := Classify(c.file); got != c.want {
			t.Errorf("Classify(%q) = %+v, want %+v", c.file, got, c.want)
		}
	}
}
