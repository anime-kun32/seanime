package scanner

import (
	"errors"
	lop "github.com/samber/lo/parallel"
	"github.com/seanime-app/seanime-server/internal/anilist"
	"github.com/seanime-app/seanime-server/internal/result"
)

type Matcher struct {
	localFiles     []*LocalFile
	mediaContainer *MediaContainer
	baseMediaCache *anilist.BaseMediaCache
	matchingCache  *MatchingCache
}

type MatcherOptions struct {
	localFiles     []*LocalFile
	mediaContainer *MediaContainer
	baseMediaCache *anilist.BaseMediaCache
}

// MatchingCache holds the previous results of the matching process.
// The key is a slice of strings representing the title variations of a local file.
// The value is the media ID of the best match.
type MatchingCache struct {
	*result.Cache[[]string, int]
}

func NewMatcher(opts *MatcherOptions) *Matcher {
	m := new(Matcher)
	m.localFiles = opts.localFiles
	m.mediaContainer = opts.mediaContainer
	m.baseMediaCache = opts.baseMediaCache
	m.matchingCache = &MatchingCache{result.NewCache[[]string, int]()}
	return m
}

// MatchLocalFilesWithMedia will match a LocalFile with a specific anilist.BaseMedia and modify the LocalFile's `mediaId`
func (m *Matcher) MatchLocalFilesWithMedia() error {

	if len(m.localFiles) == 0 {
		return errors.New("[matcher] no local files")
	}
	if len(m.mediaContainer.allMedia) == 0 {
		return errors.New("[matcher] no media fed into the matcher")
	}

	// Parallelize the matching process
	lop.ForEach(m.localFiles, func(localFile *LocalFile, index int) {
		m.FindBestCorrespondingMedia(localFile)
	})

	return nil
}

// FindBestCorrespondingMedia finds the best match for the local file
// If the best match is above a certain threshold, set the local file's mediaId to the best match's id
// If the best match is below a certain threshold, leave the local file's mediaId to 0
func (m *Matcher) FindBestCorrespondingMedia(lf *LocalFile) {
	// Check if the local file has already been matched
	if lf.MediaId != 0 {
		return
	}
	// Check if the local file has a title
	if lf.GetParsedTitle() == "" {
		return
	}

	// Create title variations
	// Check cache for title variation

	_ = lf.GetTitleVariations()

	// Compare the local file's title with all the media titles
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (m *Matcher) ValideMatches() {

}
