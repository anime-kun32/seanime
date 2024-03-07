package handlers

// HandlePlaybackSyncCurrentProgress will update the current progress of the media player.
// This route returns the media ID of the currently playing media, so the client can refetch the media data.
//
//	POST /v1/playback-manager/sync-current-progress
func HandlePlaybackSyncCurrentProgress(c *RouteCtx) error {

	err := c.App.PlaybackManager.SyncCurrentProgress()
	if err != nil {
		return c.RespondWithError(err)
	}

	mId, _ := c.App.PlaybackManager.GetCurrentMediaID()

	return c.RespondWithData(mId)
}

// HandlePlaybackPlayNextEpisode will play the next episode of the currently playing media.
//
//	POST /v1/playback-manager/play-next
func HandlePlaybackPlayNextEpisode(c *RouteCtx) error {

	err := c.App.PlaybackManager.PlayNextEpisode()
	if err != nil {
		return c.RespondWithError(err)
	}

	return c.RespondWithData(true)
}
