package main

import "testing"

func TestMainArtistName(t *testing.T) {
	cases := []struct {
		artist string
		want   string
	}{
		{"Main Artist feat. Featured Artist", "Main Artist"},
		{"Main Artist ft Featured Artist", "Main Artist"},
		{"Main Artist featuring Featured Artist", "Main Artist"},
		{"Main Artist (feat. Featured Artist)", "Main Artist"},
		{"Main Artist [ft. Featured Artist]", "Main Artist"},
		{"The Feature", "The Feature"},
		{"  Main Artist  ", "Main Artist"},
		{"feat. Featured Artist", ""},
	}
	for _, c := range cases {
		if got := mainArtistName(c.artist); got != c.want {
			t.Errorf("mainArtistName(%q) = %q, want %q", c.artist, got, c.want)
		}
	}
}

func TestMainArtist(t *testing.T) {
	cases := []struct {
		name string
		song song
		want string
	}{
		{"prefers first structured artist", song{Artist: "Display Artist", Artists: []artistCredit{{Name: "Main Artist"}, {Name: "Featured Artist"}}}, "Main Artist"},
		{"parses combined structured credit", song{Artists: []artistCredit{{Name: "Main Artist feat. Featured Artist"}}}, "Main Artist"},
		{"falls back to display artist", song{Artist: "Main Artist ft. Featured Artist"}, "Main Artist"},
		{"falls back when first structured artist is empty", song{Artist: "Display Artist", Artists: []artistCredit{{Name: " "}}}, "Display Artist"},
	}
	for _, c := range cases {
		if got := mainArtist(c.song); got != c.want {
			t.Errorf("%s: mainArtist() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestFindTopTrack(t *testing.T) {
	tracks := []topTrack{
		{Name: "Under a Glass Moon", MBID: "mbid-1", Count: 100},
		{Name: "Scene Nine: Finally Free", Count: 50},
	}
	cases := []struct {
		name, title, mbid string
		want              int64
	}{
		{"exact title", "Under a Glass Moon", "", 100},
		{"case and punctuation", "scene nine finally free", "", 50},
		{"mbid wins over title", "Something else", "mbid-1", 100},
		{"remaster suffix does not match", "Under a Glass Moon (Remastered)", "", 0},
		{"empty title", "", "", 0},
	}
	for _, c := range cases {
		got, ok := findTopTrack(tracks, c.title, c.mbid)
		if ok != (c.want != 0) || got.Count != c.want {
			t.Errorf("%s: got (%d, %v), want count %d", c.name, got.Count, ok, c.want)
		}
	}
}
