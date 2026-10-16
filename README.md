# Last.fm Popularity Rating Plugin for Navidrome

**Attention: This plugin requires Navidrome 0.61.0 or later.**

This plugin uses Last.fm artist charts to set Navidrome track star ratings, giving you a quick way to spot popular songs in your library. It takes inspiration from Plexamp's Last.fm popularity flame indicators and the [krestaino/sptnr](https://github.com/krestaino/sptnr) project that repurposes star ratings in Navidrome to show popularity on Spotify.

As krestaino notes in the project, I also utilize the 'favorite' feature instead of star ratings, so if you do utilize star ratings on tracks, this plugin may not be for you.

## Features

- Rates songs using listener counts from each artist's all-time Last.fm top tracks
- Assigns 1-5 stars based on a track's listener-count percentile among the artist's returned top tracks
- Optional Plexamp Mode gives every matching top track 1 star instead
- Tracks that are outside the Last.fm top-tracks list are set to 0 stars when they are processed
- Scans each Navidrome user's library on a configurable schedule, with progress saved between runs
- Caches Last.fm artist charts to reduce API requests
- Can leave existing ratings alone or update them, depending on configuration

## Installation

1. Download the latest `navidrome-lastfm-popularity-plugin.ndp` package from the [releases page](https://github.com/TChilderhose/navidrome-lastfm-popularity-plugin/releases).
2. Place the package in Navidrome's plugins directory. By default, this is `<Navidrome data directory>/plugins/`.
3. Start or restart Navidrome if needed, then open **Settings > Plugins** and enable **Last.fm Popularity Rating**.
4. Enter a Last.fm API key in the plugin settings. Create one at [last.fm/api/account/create](https://www.last.fm/api/account/create).
5. Adjust the plugin settings as desired. The scheduled scan starts when the plugin is enabled.

## Configuration

Configure the plugin in **Settings > Plugins > Last.fm Popularity Rating**.

| Setting | Default | Description |
| --- | --- | --- |
| Last.fm API key (`api_key`) | Required | API key used to request artist top tracks from Last.fm. |
| Schedule (`cron`) | `*/15 * * * *` | Cron schedule for library scans; the default is every 15 minutes. |
| Songs per run (`batch_size`) | `500` | Maximum number of songs processed per user in one scan. A scan may stop earlier at the run-time limit and resume later. |
| Top tracks considered (`top_tracks_limit`) | `50` | Number of all-time Last.fm artist top tracks to request. Allowed range: 1-100. |
| Cache duration (`cache_ttl_days`) | `30` days | Lifetime for newly cached artist charts. Allowed range: 1-365 days. Changing this does not change expiration times for entries already cached. |
| Overwrite existing ratings (`overwrite_existing`) | `false` | When off, songs with any existing star rating are skipped. When on, processed songs are updated, including setting a song to 0 stars if it is absent from a nonempty Last.fm chart. |
| Plexamp Mode (`plexamp_mode`) | `false` | Give every matching track in an artist's Last.fm top tracks 1 star instead of using the 1-5 percentile scale. Tracks not in the list still receive 0 stars. |
| Verbose logging (`verbose_logging`) | `false` | Logs rating results and reasons songs were skipped. |

Last.fm's artist top-tracks endpoint provides all-time data, not a selectable recent period. A shorter cache duration refreshes the same all-time chart more often; it does not produce recent-window rankings.

## Configurations Recommendations

When first populating, having a more frequent `Schedule` and smaller `Songs per run` (default of `500` should be fine) is better. It needs some time to query Last.fm and cache the results. If the `Songs per run` is too large, it can hit the run-time limit of 25 seconds and could trigger Last.fm's Too Many Requests errors if too frequent.
Once it has done a full round of populating (look in the Navidrome logs for "user {your user name}: rated 0 songs, ...) then setting a less frequent `Schedule` and increasing `Songs per run` makes sense as most ratings wont be adjusted due to cache and newer songs to your library can be snagged.

With `Plexamp Mode` set to `on`. Set the `Top tracks considered` option to a lower value like 10. If its too high the artist's library will just be a croweded mess of 1 star songs with no real tangible information.
With `Plexamp Mode` set to `off`. The default value of `Top tracks considered` at 50 seems good for most artists. If you listen to a lot of newer artists who do not have a massive catelog of music yet, lowering the value may provide better information.


## How It Works

For each Navidrome user, the plugin pages through the user's songs and checks each eligible track against the top tracks for its main artist. A track's MusicBrainz recording ID is used when available; otherwise, titles are compared after lowercasing and removing punctuation and whitespace. Featured-artist credits are stripped from the main artist name when possible.

With Plexamp Mode disabled (the default), listener counts are ranked into percentile bands roughly inspired by the [68-95-99.7 rule](https://en.wikipedia.org/wiki/68%E2%80%9395%E2%80%9399.7_rule):

| Stars | Listener-count percentile |
| --- | --- |
| 1 | Lowest 2.5% |
| 2 | Above 2.5% through 16% |
| 3 | Above 16% through 84% |
| 4 | Above 84% through 97.5% |
| 5 | Above 97.5% |

With Plexamp Mode enabled, every track found in the returned Last.fm top-tracks list gets 1 star, regardless of listener count.

A matching track below the chart's lowest listener count, or a track absent from a nonempty chart, receives 0 stars. If Last.fm has no chart for an artist, the plugin leaves the song's rating unchanged.

Scans process up to the configured batch size per user and stop at a 25-second run budget. The next position is saved so later scheduled runs can continue; reaching the end of a user's library wraps the scan back to the beginning. Artist charts are cached for the configured duration.


## Building and Testing

Go 1.25 or later is required. TinyGo is optional and produces a smaller WebAssembly module.

```sh
# Run tests
make test

# Build plugin.wasm
make build

# Build the distributable .ndp package
make package
```

The package contains `plugin.wasm` and `manifest.json`. To build without Make, use either:

```sh
# TinyGo
tinygo build -opt=2 -scheduler=none -no-debug -o plugin.wasm -target wasip1 -buildmode=c-shared .
zip navidrome-lastfm-popularity-plugin.ndp plugin.wasm manifest.json
```

```sh
# Standard Go
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
zip navidrome-lastfm-popularity-plugin.ndp plugin.wasm manifest.json
```

## Acknowledgements & Thanks

* [Deluan](https://github.com/deluan) for [Navidrome](https://github.com/navidrome/navidrome); such an incredible self-hosted music platform
* Elan Feingold and the [Plexamp](https://plexamp.com/) team for re-sparking my love for music years and years ago
* [krestaino](https://github.com/krestaino) for [sptnr](https://github.com/krestaino/sptnr) which inspired me to create this plugin
* [Tolriq](https://github.com/Tolriq) for [Symfonium](https://symfonium.app/); the closest thing to perfect a music player I've ever used


## License

This project is licensed under the GNU General Public License v3.0. See [LICENSE](LICENSE) for the full license text.
