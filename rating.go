package main

// rates a track by its percentile among the artist's top tracks using normal-distribution bands.
// Tracks below the lowest listed count are outside the list and get 0 stars.
// Returns 0 when there is no artist data.
func starsForTrack(count int64, artistCounts []int64, plexampMode bool) int {
	if len(artistCounts) == 0 {
		return 0
	}

	//determing how many are below and the lowest in the list
	below, lowest := 0, artistCounts[0]
	for _, c := range artistCounts {
		lowest = min(lowest, c)
		if c < count {
			below++
		}
	}

	//lower than the lowest in the top songs, so outside the list
	if count < lowest {
		return 0
	}

	//in the top tracks, but in plexamp mode so just return 1 star
	if plexampMode {
		return 1
	}

	//calculate where in top tracks this lands
	percentile := 0.5
	if len(artistCounts) > 1 {
		percentile = float64(below) / float64(len(artistCounts)-1)
	}

	switch { //rough 68-95-99.7 rule
	case percentile <= 0.025:
		return 1
	case percentile <= 0.16:
		return 2
	case percentile <= 0.84:
		return 3
	case percentile <= 0.975:
		return 4
	default:
		return 5
	}
}
