package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/plugins/pdk/go/host"
	"github.com/navidrome/navidrome/plugins/pdk/go/lifecycle"
	"github.com/navidrome/navidrome/plugins/pdk/go/pdk"
	"github.com/navidrome/navidrome/plugins/pdk/go/scheduler"
)

const (
	pluginVersion = "0.1.0"

	scheduleID   = "lastfm-popularity-sync"
	pageSize     = 100 //page size for fetching songs via search3
	runBudget = 25 * time.Second //The host kills scheduler callbacks after 30s, so stop well before that.

	configAPIKey    = "api_key"
	configTopLimit  = "top_tracks_limit"
	configCacheTTL  = "cache_ttl_days"
	configCron      = "cron"
	configBatch     = "batch_size"
	configOverwrite = "overwrite_existing"
	configPlexamp   = "plexamp_mode"
	configVerbose   = "verbose_logging"

	defaultCron  = "*/15 * * * *"
	defaultBatch = 500

	defaultTopLimit     = 50
	maxTopLimit         = 100 //config should cap up, but just in case

	defaultCacheTTLDays = 30
	maxCacheTTLDays     = 365
)

type plugin struct{}

var (
	_ lifecycle.InitProvider     = (*plugin)(nil)
	_ scheduler.CallbackProvider = (*plugin)(nil)
)

func init() {
	p := &plugin{}
	lifecycle.Register(p)
	scheduler.Register(p)
}

func main() {}

func (p *plugin) OnInit() error {
	//schedule up our background task attempting to use configured cron string
	cron, ok := host.ConfigGet(configCron)
	if !ok || strings.TrimSpace(cron) == "" {
		cron = defaultCron
	}
	_, err := host.SchedulerScheduleRecurring(strings.TrimSpace(cron), "", scheduleID)
	return err
}

//background task trigger
func (p *plugin) OnCallback(req scheduler.SchedulerCallbackRequest) error {
	if req.ScheduleID != scheduleID {
		return nil
	}
	users, err := host.UsersGetUsers()
	if err != nil {
		return err
	}

	deadline := time.Now().Add(runBudget) //figure out when we need to stop before the 30 second runtime warning
	for _, u := range users {
		if time.Now().After(deadline) {
			break //deadline hit, exit
		}
		if err := syncUser(u.UserName, deadline); err != nil {
			pdk.Log(pdk.LogError, fmt.Sprintf("sync failed for %s: %v", u.UserName, err))
			if errors.Is(err, errRateLimited) {
				break //other users would hit the same limit
			}
		}
	}
	return nil
}

type artistCredit struct {
	Name string `json:"name"`
}

type song struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Artist        string         `json:"artist"`
	Artists       []artistCredit `json:"artists"`
	MusicBrainzID string         `json:"musicBrainzId"`
	UserRating    int            `json:"userRating"`
}

// rates songs until the batch or deadline is reached, resuming from a stored offset and wrapping at the end.
func syncUser(user string, deadline time.Time) (err error) {
	overwrite := configBool(configOverwrite, false)
	batch := defaultBatch
	if n, ok := host.ConfigGetInt(configBatch); ok && n > 0 {
		batch = int(n)
	}

	//figure out where we left off last for this user
	cursorKey := "cursor:" + user
	offset := 0
	if data, exists, err := host.KVStoreGet(cursorKey); err == nil && exists {
		if v, err := strconv.ParseInt(string(data), 10, 64); err == nil && v > 0 {
			offset = int(v)
		}
	}

	next, processed, rated := offset, 0, 0
	//defer saving progress until function returns
	// makes sure is saved even on error or timeout so a failing song can't stall the scan.
	defer func() {
		if serr := host.KVStoreSet(cursorKey, []byte(strconv.Itoa(next))); serr != nil && err == nil {
			err = serr
		}
		pdk.Log(pdk.LogInfo, fmt.Sprintf("user %s: rated %d songs, next offset %d", user, rated, next))
	}()

	for processed < batch {
		size := min(pageSize, batch-processed) //never go above pageSize(100) for search3
		songs, lerr := listSongs(user, next, size)
		if lerr != nil {
			return lerr
		}
		for _, s := range songs {
			if time.Now().After(deadline) {
				return nil //deadline hit, exit
			}
			//raiting unset or plugin has overwrite enabled
			if s.UserRating == 0 || overwrite {
				ok, rerr := rateSong(user, s) //lets try to rate it
				if rerr != nil {
					return rerr
				}
				if ok {
					rated++
				}
			}
			processed++
			next++
		}
		if len(songs) < size { //reached the end of the library so wrap around
			next = 0
			return nil
		}
	}
	return nil
}

// attempts
// returns (was new rating set, error)
func rateSong(user string, s song) (bool, error) {
	// extract the main artist
	if s.Artist == "" || s.Title == "" {
		verbosef("skip %q: missing artist or title", s.ID)
		return false, nil
	}
	artist := mainArtist(s)
	if artist == "" {
		verbosef("skip %q: missing main artist", s.ID)
		return false, nil
	}

	//get the top artist tracks
	tracks, err := artistTopTracks(artist, getTopLimit())
	if err != nil {
		return false, err
	}
	if len(tracks) == 0 {
		verbosef("skip %s - %s: no top tracks on Last.fm for artist", s.Artist, s.Title)
		return false, nil
	}

	//search top artist tracks for this track
	t, ok := findTopTrack(tracks, s.Title, s.MusicBrainzID)
	stars := 0
	if !ok { //not in top artist tracks
		verbosef("%s - %s: outside artist top %d tracks, stars=0, current=%d", s.Artist, s.Title, len(tracks), s.UserRating)
	} else {
		stars = starsForTrack(t.Count, topCounts(tracks), configBool(configPlexamp, false))
		verbosef("%s - %s: listeners=%d, artist tracks sampled=%d, stars=%d, current=%d", s.Artist, s.Title, t.Count, len(tracks), stars, s.UserRating)
	}

	//rating already matches
	if stars == s.UserRating {
		return false, nil
	}

	//attempt to set the rating through subsonic api
	q := url.Values{"u": {user}, "id": {s.ID}, "rating": {strconv.Itoa(stars)}}
	if _, err := host.SubsonicAPICall("setRating?" + q.Encode()); err != nil {
		return false, err
	}

	//everything passed and we set the rating
	return true, nil
}

var featureCreditMarker = regexp.MustCompile(`(?i)(^|[\s(,\[]+)(featuring|feat\.?|ft\.?)\s+`)

func mainArtist(s song) string {
	if len(s.Artists) > 0 {
		if artist := mainArtistName(s.Artists[0].Name); artist != "" {
			return artist
		}
	}
	return mainArtistName(s.Artist)
}

func mainArtistName(artist string) string {
	marker := featureCreditMarker.FindStringIndex(artist)
	if marker == nil {
		return strings.TrimSpace(artist)
	}
	return strings.TrimRight(strings.TrimSpace(artist[:marker[0]]), "([{,")
}

//gets list of songs via subsonic's search3
func listSongs(user string, offset, count int) ([]song, error) {
	q := url.Values{
		"u":           {user},
		"songCount":   {strconv.Itoa(count)},
		"songOffset":  {strconv.Itoa(offset)},
		"albumCount":  {"0"},
		"artistCount": {"0"},
	}
	body, err := host.SubsonicAPICall("search3?" + q.Encode())
	if err != nil {
		return nil, err
	}
	var out struct {
		Response struct {
			Status string `json:"status"`
			Result struct {
				Song []song `json:"song"`
			} `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil, err
	}
	if out.Response.Status != "ok" {
		return nil, fmt.Errorf("search3 returned status %q", out.Response.Status)
	}
	return out.Response.Result.Song, nil
}

// verbosef logs at info level only when verbose_logging is enabled.
func verbosef(format string, args ...any) {
	if configBool(configVerbose, false) {
		pdk.Log(pdk.LogInfo, fmt.Sprintf(format, args...))
	}
}

func getTopLimit() int {
	if n, ok := host.ConfigGetInt(configTopLimit); ok && n > 0 {
		return int(min(n, maxTopLimit))
	}
	return defaultTopLimit
}

func cacheTTLSeconds() int64 {
	days := defaultCacheTTLDays
	if n, ok := host.ConfigGetInt(configCacheTTL); ok && n > 0 {
		days = int(min(n, maxCacheTTLDays))
	}
	return int64(days * 24 * 60 * 60)
}

func configBool(key string, def bool) bool {
	v, ok := host.ConfigGet(key)
	if !ok {
		return def
	}
	return strings.EqualFold(strings.TrimSpace(v), "true")
}
