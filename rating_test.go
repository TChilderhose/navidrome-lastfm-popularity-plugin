package main

import "testing"

func TestStarsFor(t *testing.T) {
	top := []int64{2000, 4000, 6000, 8000, 10000, 12000, 14000, 16000, 18000, 20000}
	cases := []struct {
		name  string
		count int64
		list  []int64
		want  int
	}{
		{"no count", 0, top, 0},
		{"no artist data", 5000, nil, 0},
		{"top track", 20000, top, 5},
		{"above all", 30000, top, 5},
		{"top 30 percent", 16000, top, 3},
		{"upper middle", 14000, top, 3},
		{"median", 10000, top, 3},
		{"lower middle", 6000, top, 3},
		{"least popular listed", 2000, top, 1},
		{"outside the list", 1500, top, 0},
		{"single listed track", 5000, []int64{5000}, 3},
	}
	for _, c := range cases {
		if got := starsForTrack(c.count, c.list, false); got != c.want {
			t.Errorf("%s: starsFor(%d, ...) = %d, want %d", c.name, c.count, got, c.want)
		}
	}
}

func TestStarsForPercentileBands(t *testing.T) {
	counts := make([]int64, 100)
	for i := range counts {
		counts[i] = int64(i + 1)
	}
	distribution := make([]int, 6)
	for _, count := range counts {
		distribution[starsForTrack(count, counts, false)]++
	}
	want := []int{0, 3, 13, 68, 13, 3}
	for stars := 1; stars <= 5; stars++ {
		if distribution[stars] != want[stars] {
			t.Errorf("%d-star count = %d, want %d", stars, distribution[stars], want[stars])
		}
	}
}

func TestStarsForTrackPlexampMode(t *testing.T) {
	counts := []int64{10, 20, 30, 40, 50}
	cases := []struct {
		name    string
		count   int64
		plexamp bool
		want    int
	}{
		{"default mode preserves percentile rating", 50, false, 5},
		{"Plexamp mode marks listed track with one star", 50, true, 1},
		{"Plexamp mode leaves out-of-list track unrated", 5, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := starsForTrack(c.count, counts, c.plexamp); got != c.want {
				t.Errorf("starsForTrack(%d, ..., %t) = %d, want %d", c.count, c.plexamp, got, c.want)
			}
		})
	}
}
