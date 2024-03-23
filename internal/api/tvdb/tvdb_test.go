package tvdb

import (
	"github.com/seanime-app/seanime/internal/api/anizip"
	"github.com/seanime-app/seanime/internal/test_utils"
	"github.com/seanime-app/seanime/internal/util"
	"testing"
)

func TestTVDB_FetchSeriesEpisodes(t *testing.T) {
	test_utils.InitTestProvider(t)

	tests := []struct {
		name          string
		anilistId     int
		episodeNumber int
	}{
		{
			name:          "Dungeon Meshi",
			anilistId:     153518,
			episodeNumber: 1,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			anizipMedia, err := anizip.FetchAniZipMedia("anilist", tt.anilistId)
			if err != nil {
				t.Fatalf("could not fetch anizip media for %s", tt.name)
			}

			tvdbId := anizipMedia.Mappings.ThetvdbID
			if tvdbId == 0 {
				t.Fatalf("could not find tvdb id for %s", tt.name)
			}

			// Create TVDB instance
			tvdb := NewTVDB(&NewTVDBOptions{
				ApiKey: "",
				Logger: util.NewLogger(),
			})

			episodes, err := tvdb.FetchSeriesEpisodes(tvdbId)
			if err != nil {
				t.Fatalf("could not fetch episodes for %s: %s", tt.name, err)
			}

			for _, episode := range episodes {

				t.Log("Episode ID:", episode.ID)
				t.Log("\t Number:", episode.Number)
				t.Log("\t Episode Number:", episode.Number)
				t.Log("\t Image:", episode.Image)
				t.Log("\t AiredAt:", episode.AiredAt)
				t.Log("")

			}

		})

	}

}

func TestTVDB_FetchSeasons(t *testing.T) {
	test_utils.InitTestProvider(t)

	tests := []struct {
		name      string
		anilistId int
	}{
		{
			name:      "Dungeon Meshi",
			anilistId: 153518,
		},
		{
			name:      "Boku no Kokoro no Yabai Yatsu 2nd Season",
			anilistId: 166216,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			anizipMedia, err := anizip.FetchAniZipMedia("anilist", tt.anilistId)
			if err != nil {
				t.Fatalf("could not fetch anizip media for %s", tt.name)
			}

			tvdbId := anizipMedia.Mappings.ThetvdbID
			if tvdbId == 0 {
				t.Fatalf("could not find tvdb id for %s", tt.name)
			}

			// Create TVDB instance
			tvdb := NewTVDB(&NewTVDBOptions{
				ApiKey: "",
				Logger: util.NewLogger(),
			})

			// Get token
			_, err = tvdb.getTokenWithTries()
			if err != nil {
				t.Fatalf("could not get token: %s", err)
			}

			// Fetch seasons
			seasons, err := tvdb.fetchSeasons(tvdbId)
			if err != nil {
				t.Fatalf("could not fetch metadata for %s: %s", tt.name, err)
			}

			for _, season := range seasons {

				t.Log("Season ID:", season.ID)
				t.Log("\t Name:", season.Type.Name)
				t.Log("\t Number:", season.Number)
				t.Log("\t Number:", season.Type.Type)
				t.Log("\t LastUpdated:", season.LastUpdated)
				t.Log("")

			}

		})

	}

}

func TestTVDB_fetchEpisodes(t *testing.T) {
	test_utils.InitTestProvider(t)

	tests := []struct {
		name          string
		anilistId     int
		episodeNumber int
	}{
		{
			name:          "Dungeon Meshi",
			anilistId:     153518,
			episodeNumber: 1,
		},
		{
			name:      "Boku no Kokoro no Yabai Yatsu 2nd Season",
			anilistId: 166216,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			anizipMedia, err := anizip.FetchAniZipMedia("anilist", tt.anilistId)
			if err != nil {
				t.Fatalf("could not fetch anizip media for %s", tt.name)
			}

			tvdbId := anizipMedia.Mappings.ThetvdbID
			if tvdbId == 0 {
				t.Fatalf("could not find tvdb id for %s", tt.name)
			}

			// Create TVDB instance
			tvdb := NewTVDB(&NewTVDBOptions{
				ApiKey: "",
				Logger: util.NewLogger(),
			})

			// Get token
			_, err = tvdb.getTokenWithTries()
			if err != nil {
				t.Fatalf("could not get token: %s", err)
			}

			// Fetch seasons
			seasons, err := tvdb.fetchSeasons(tvdbId)
			if err != nil {
				t.Fatalf("could not fetch metadata for %s: %s", tt.name, err)
			}

			// Fetch episodes
			res, err := tvdb.fetchEpisodes(seasons)
			if err != nil {
				t.Fatalf("could not fetch episode metadata for %s: %s", tt.name, err)
			}

			for _, episode := range res {

				t.Log("Episode ID:", episode.ID)
				t.Log("\t Number:", episode.Number)
				t.Log("\t Episode Number:", episode.Number)
				t.Log("\t Image:", episode.Image)
				t.Log("\t Name:", episode.Name)
				t.Log("\t Season Number:", episode.SeasonNumber)
				t.Log("\t Season Name:", episode.SeasonName)

				t.Log("")

			}

		})

	}

}
