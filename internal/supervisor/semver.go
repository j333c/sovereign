package supervisor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Supported update channels
const (
	ChannelStable       = "stable"
	ChannelSecurityOnly = "security-only"
	ChannelPreview      = "preview"
)

// DefaultChannel is the recommended production default.
const DefaultChannel = ChannelStable

// semverRegex matches SemVer 2.0.0 tags with optional leading 'v' and numeric '-rc.<N>' pre-release.
var semverRegex = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:-rc\.([0-9]+))?$`)

// Version represents a parsed SemVer 2.0.0 version with optional numeric -rc pre-release identifier.
type Version struct {
	Major         int
	Minor         int
	Patch         int
	IsPreRelease  bool
	RCNumber      int
	OriginalString string
}

// ParseVersion parses a SemVer 2.0.0 version string.
func ParseVersion(s string) (Version, error) {
	trimmed := strings.TrimSpace(s)
	matches := semverRegex.FindStringSubmatch(trimmed)
	if matches == nil {
		return Version{}, fmt.Errorf("invalid SemVer 2.0.0 format: %q", s)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %w", err)
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %w", err)
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %w", err)
	}

	v := Version{
		Major:          major,
		Minor:          minor,
		Patch:          patch,
		OriginalString: trimmed,
	}

	if matches[4] != "" {
		rc, err := strconv.Atoi(matches[4])
		if err != nil {
			return Version{}, fmt.Errorf("invalid rc number: %w", err)
		}
		v.IsPreRelease = true
		v.RCNumber = rc
	}

	return v, nil
}

// String returns the normalized canonical tag representation (e.g. "v1.2.3" or "v1.2.3-rc.1").
func (v Version) String() string {
	if v.IsPreRelease {
		return fmt.Sprintf("v%d.%d.%d-rc.%d", v.Major, v.Minor, v.Patch, v.RCNumber)
	}
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns:
//
//	-1 if v < other
//	 0 if v == other
//	+1 if v > other
//
// In accordance with SemVer 2.0.0 section 11 precedence:
// 1.0.0-rc.1 < 1.0.0-rc.2 < 1.0.0-rc.10 < 1.0.0
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Major, Minor, Patch are identical.
	// Normal version has higher precedence than a pre-release version.
	if !v.IsPreRelease && other.IsPreRelease {
		return 1
	}
	if v.IsPreRelease && !other.IsPreRelease {
		return -1
	}
	if !v.IsPreRelease && !other.IsPreRelease {
		return 0
	}

	// Both are pre-release: compare numeric rc identifier
	if v.RCNumber < other.RCNumber {
		return -1
	}
	if v.RCNumber > other.RCNumber {
		return 1
	}
	return 0
}

// QualifiesForChannel checks if a candidate version qualifies as an upgrade from current on the given channel.
func QualifiesForChannel(candidate, current Version, channel string) bool {
	// Strict monotonic floor: candidate must be strictly newer than current
	if candidate.Compare(current) <= 0 {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(channel)) {
	case ChannelStable:
		// Matches latest SemVer release tag without pre-release suffixes
		return !candidate.IsPreRelease

	case ChannelSecurityOnly:
		// Matches patch release in the current major.minor series
		if candidate.IsPreRelease {
			return false
		}
		if candidate.Major != current.Major || candidate.Minor != current.Minor {
			return false
		}
		// Patch > CurrentPatch, or Patch == CurrentPatch if current was a preview release graduating to stable
		if candidate.Patch > current.Patch {
			return true
		}
		if candidate.Patch == current.Patch && current.IsPreRelease {
			return true
		}
		return false

	case ChannelPreview:
		// Matches latest pre-release or stable tag (whichever is newer by SemVer 2.0.0 precedence)
		return true

	default:
		// Default to stable behavior
		return !candidate.IsPreRelease
	}
}

// SelectLatestQualifyingTag evaluates candidate git tags against current running version and configured channel.
// Returns the newest qualifying Version, its original tag string, and true if found; otherwise false.
func SelectLatestQualifyingTag(tags []string, current Version, channel string) (Version, string, bool) {
	var bestVersion Version
	var bestTag string
	found := false

	for _, tag := range tags {
		v, err := ParseVersion(tag)
		if err != nil {
			continue
		}

		if QualifiesForChannel(v, current, channel) {
			if !found || v.Compare(bestVersion) > 0 {
				bestVersion = v
				bestTag = tag
				found = true
			}
		}
	}

	return bestVersion, bestTag, found
}
