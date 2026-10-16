package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
)

const (
	lastfmAPI     = "https://ws.audioscrobbler.com/2.0/"
	httpTimeoutMs = 10000
)

var errRateLimited = errors.New("last.fm rate limit hit")

func lastfmGet(params url.Values) ([]byte, error) {
	key, _ := host.ConfigGet(configAPIKey)
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("%s is not configured", configAPIKey)
	}
	params.Set("api_key", strings.TrimSpace(key))
	params.Set("format", "json")
	params.Set("autocorrect", "1")

	resp, err := host.HTTPSend(host.HTTPRequest{
		Method:    "GET",
		URL:       lastfmAPI + "?" + params.Encode(),
		Headers:   map[string]string{"User-Agent": "NavidromeLastFMPopularity/" + pluginVersion},
		TimeoutMs: httpTimeoutMs,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 {
		return nil, errRateLimited
	}
	return resp.Body, nil
}

type lastfmError struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

// extracts the listener count from a Last.fm track-like object.
type lastfmCount struct {
	Listeners string `json:"listeners"`
}

func (c lastfmCount) value() int64 {
	n, _ := strconv.ParseInt(c.Listeners, 10, 64)
	return n
}

// one entry of an artist's Last.fm top tracks.
type topTrack struct {
	Name  string `json:"n"`
	MBID  string `json:"m,omitempty"`
	Count int64  `json:"c"`
}

// returns the artist's top tracks with listener counts, or nil when Last.fm has none.
func artistTopTracks(artist string, limit int) ([]topTrack, error) {
	cacheKey := "toptracks:" + strconv.Itoa(limit) + ":" + strings.ToLower(artist)
	if data, exists, err := host.KVStoreGet(cacheKey); err == nil && exists {
		var cached []topTrack
		if json.Unmarshal(data, &cached) == nil {
			return cached, nil
		}
	}
	body, err := lastfmGet(url.Values{"method": {"artist.getTopTracks"}, "artist": {artist}, "limit": {strconv.Itoa(limit)}})
	if err != nil {
		return nil, err
	}
	var out struct {
		lastfmError
		TopTracks struct {
			Track json.RawMessage `json:"track"`
		} `json:"toptracks"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	if out.Error != 0 && out.Error != 6 { // 6 = artist not found
		return nil, fmt.Errorf("last.fm error %d: %s", out.Error, out.Message)
	}

	type rawTrack struct {
		lastfmCount
		Name string `json:"name"`
		MBID string `json:"mbid"`
	}
	// Last.fm returns an object instead of an array for single results.
	var raw []rawTrack
	if len(out.TopTracks.Track) > 0 {
		if err := json.Unmarshal(out.TopTracks.Track, &raw); err != nil {
			var one rawTrack
			if err := json.Unmarshal(out.TopTracks.Track, &one); err == nil {
				raw = []rawTrack{one}
			}
		}
	}
	tracks := []topTrack{}
	for _, t := range raw {
		if n := t.value(); n > 0 {
			tracks = append(tracks, topTrack{Name: t.Name, MBID: t.MBID, Count: n})
		}
	}
	if data, err := json.Marshal(tracks); err == nil {
		if err := host.KVStoreSetWithTTL(cacheKey, data, cacheTTLSeconds()); err != nil {
			pdk.Log(pdk.LogWarn, "cache write failed: "+err.Error())
		}
	}
	return tracks, nil
}

// matches a local song to the artist's top tracks by MusicBrainz ID, then by normalized title.
func findTopTrack(tracks []topTrack, title, mbid string) (topTrack, bool) {
	if mbid != "" {
		for _, t := range tracks {
			if t.MBID == mbid {
				return t, true
			}
		}
	}
	norm := normalizeTitle(title)
	if norm == "" {
		return topTrack{}, false
	}
	for _, t := range tracks {
		if normalizeTitle(t.Name) == norm {
			return t, true
		}
	}
	return topTrack{}, false
}

// lowercases and keeps only letters and digits.
func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func topCounts(tracks []topTrack) []int64 {
	counts := make([]int64, len(tracks))
	for i, t := range tracks {
		counts[i] = t.Count
	}
	return counts
}
