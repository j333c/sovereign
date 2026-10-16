package transcode

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

var (
	ErrUnknownMobileProfile   = errors.New("unknown mobile transcode profile")
	ErrInvalidSourcePath      = errors.New("source path is invalid")
	ErrIdenticalPaths         = errors.New("target path cannot be identical to source path")
	ErrUnsupportedVideoFormat = errors.New("unsupported video format")
	ErrSourceOutsideDataDir   = errors.New("source path is outside allowed data directory")
	ErrSourceNotFound         = errors.New("source file does not exist or is inaccessible")
	ErrSourceIsDir            = errors.New("source path is a directory, not a video file")
)

const (
	ProfileMobile720p  = "mobile-720p"
	ProfileMobile1080p = "mobile-1080p"
)

// MobileProfileConfig defines ffmpeg and file naming parameters for a mobile direct-play profile.
type MobileProfileConfig struct {
	Name            string `json:"name"`
	MaxResolution   string `json:"max_resolution"`
	ScaleFilter     string `json:"scale_filter"`       // Software / QSV / NVENC scaling filter
	VAAPIScaleFilter string `json:"vaapi_scale_filter"` // Hardware VAAPI surface scaling filter
	VideoBitrate    string `json:"video_bitrate"`
	MaxRate         string `json:"max_rate"`
	BufSize         string `json:"buf_size"`
	AudioCodec      string `json:"audio_codec"`
	AudioBitrate    string `json:"audio_bitrate"`
	AudioChannels   int    `json:"audio_channels"`
	FastStart       bool   `json:"fast_start"`
	Container       string `json:"container"`
	Suffix          string `json:"suffix"`
}

// mobileProfiles defines internal standardized profiles for mobile offline viewing.
var mobileProfiles = map[string]MobileProfileConfig{
	ProfileMobile720p: {
		Name:             ProfileMobile720p,
		MaxResolution:    "720p",
		ScaleFilter:      "scale=-2:'min(720,ih)'",
		VAAPIScaleFilter: "scale_vaapi=w=-2:h=720",
		VideoBitrate:     "2500k",
		MaxRate:          "3000k",
		BufSize:          "5000k",
		AudioCodec:       "aac",
		AudioBitrate:     "128k",
		AudioChannels:    2,
		FastStart:        true,
		Container:        "mp4",
		Suffix:           " - 720p.mp4",
	},
	ProfileMobile1080p: {
		Name:             ProfileMobile1080p,
		MaxResolution:    "1080p",
		ScaleFilter:      "scale=-2:'min(1080,ih)'",
		VAAPIScaleFilter: "scale_vaapi=w=-2:h=1080",
		VideoBitrate:     "4500k",
		MaxRate:          "5500k",
		BufSize:          "9000k",
		AudioCodec:       "aac",
		AudioBitrate:     "192k",
		AudioChannels:    2,
		FastStart:        true,
		Container:        "mp4",
		Suffix:           " - 1080p.mp4",
	},
}

// IsMobileProfile returns true if the specified profile name is a supported mobile profile.
func IsMobileProfile(profile string) bool {
	norm := strings.ToLower(strings.TrimSpace(profile))
	_, ok := mobileProfiles[norm]
	return ok
}

// GetMobileProfile returns the MobileProfileConfig for the specified profile name.
func GetMobileProfile(profile string) (MobileProfileConfig, bool) {
	norm := strings.ToLower(strings.TrimSpace(profile))
	cfg, ok := mobileProfiles[norm]
	return cfg, ok
}

// ListMobileProfiles returns all configured mobile profiles.
func ListMobileProfiles() []MobileProfileConfig {
	res := make([]MobileProfileConfig, 0, len(mobileProfiles))
	for _, p := range mobileProfiles {
		res = append(res, p)
	}
	return res
}

// ResolveMultiVersionPath computes the target output path for a mobile profile alongside
// the master video file according to Jellyfin multi-version naming conventions.
//
// Examples:
//
//	"/data/media/movies/Inception (2010)/Inception (2010).mkv" -> ".../Inception (2010) - 720p.mp4"
//	"/data/media/movies/Inception (2010)/Inception (2010) - 1080p.mkv" -> ".../Inception (2010) - 720p.mp4"
//	"/data/media/tv/Show/Season 01/Show - S01E01.mkv" -> ".../Show - S01E01 - 1080p.mp4"
func ResolveMultiVersionPath(sourcePath, profile string) (string, error) {
	cleanSource := filepath.Clean(strings.TrimSpace(sourcePath))
	if cleanSource == "" || cleanSource == "." || cleanSource == "/" {
		return "", ErrInvalidSourcePath
	}

	mobCfg, ok := GetMobileProfile(profile)
	if !ok {
		return "", fmt.Errorf("%w: %q (must be %s or %s)", ErrUnknownMobileProfile, profile, ProfileMobile720p, ProfileMobile1080p)
	}

	dir := filepath.Dir(cleanSource)
	ext := filepath.Ext(cleanSource)
	base := strings.TrimSuffix(filepath.Base(cleanSource), ext)

	// Clean any pre-existing quality/version tags to prevent repetitive names like "... - 1080p - 720p.mp4"
	cleanBase := cleanVersionSuffixes(base)

	targetPath := filepath.Join(dir, cleanBase+mobCfg.Suffix)
	if targetPath == cleanSource {
		return "", ErrIdenticalPaths
	}

	return targetPath, nil
}

// cleanVersionSuffixes iteratively removes stacked and case-insensitive quality tags from a base name.
func cleanVersionSuffixes(base string) string {
	knownSuffixes := []string{
		" - 720p",
		" - 1080p",
		" - 2160p",
		" - 4k",
		" - direct-play",
		".direct-play",
		" - original",
		" - master",
	}

	res := strings.TrimSpace(base)
	changed := true
	for changed {
		changed = false
		for _, s := range knownSuffixes {
			if len(res) > len(s) && strings.EqualFold(res[len(res)-len(s):], s) {
				res = strings.TrimSpace(res[:len(res)-len(s)])
				changed = true
				break
			}
		}
	}
	return res
}
