package supervisor

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input       string
		wantErr     bool
		expected    Version
		canonicalStr string
	}{
		{
			input: "v1.2.3",
			expected: Version{
				Major: 1, Minor: 2, Patch: 3, IsPreRelease: false, RCNumber: 0,
			},
			canonicalStr: "v1.2.3",
		},
		{
			input: "1.2.3",
			expected: Version{
				Major: 1, Minor: 2, Patch: 3, IsPreRelease: false, RCNumber: 0,
			},
			canonicalStr: "v1.2.3",
		},
		{
			input: "v1.2.0-rc.1",
			expected: Version{
				Major: 1, Minor: 2, Patch: 0, IsPreRelease: true, RCNumber: 1,
			},
			canonicalStr: "v1.2.0-rc.1",
		},
		{
			input: "v1.2.0-rc.10",
			expected: Version{
				Major: 1, Minor: 2, Patch: 0, IsPreRelease: true, RCNumber: 10,
			},
			canonicalStr: "v1.2.0-rc.10",
		},
		{
			input:   "latest",
			wantErr: true,
		},
		{
			input:   "v1.2",
			wantErr: true,
		},
		{
			input:   "v1.2.3-beta.1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if v.Major != tt.expected.Major || v.Minor != tt.expected.Minor || v.Patch != tt.expected.Patch ||
				v.IsPreRelease != tt.expected.IsPreRelease || v.RCNumber != tt.expected.RCNumber {
				t.Errorf("parsed version mismatch: got %+v, want %+v", v, tt.expected)
			}
			if v.String() != tt.canonicalStr {
				t.Errorf("canonical string mismatch: got %q, want %q", v.String(), tt.canonicalStr)
			}
		})
	}
}

func TestSemVerPrecedence(t *testing.T) {
	// Verify exact SemVer 2.0.0 numeric -rc.<N> precedence:
	// v1.2.0-rc.1 < v1.2.0-rc.2 < v1.2.0-rc.10 < v1.2.0
	rc1, _ := ParseVersion("v1.2.0-rc.1")
	rc2, _ := ParseVersion("v1.2.0-rc.2")
	rc10, _ := ParseVersion("v1.2.0-rc.10")
	final, _ := ParseVersion("v1.2.0")
	patch1, _ := ParseVersion("v1.2.1")
	minor1, _ := ParseVersion("v1.3.0")
	major2, _ := ParseVersion("v2.0.0")

	if rc1.Compare(rc2) >= 0 {
		t.Errorf("expected rc1 < rc2")
	}
	if rc2.Compare(rc10) >= 0 {
		t.Errorf("expected rc2 < rc10")
	}
	if rc10.Compare(final) >= 0 {
		t.Errorf("expected rc10 < final")
	}
	if final.Compare(patch1) >= 0 {
		t.Errorf("expected final < patch1")
	}
	if patch1.Compare(minor1) >= 0 {
		t.Errorf("expected patch1 < minor1")
	}
	if minor1.Compare(major2) >= 0 {
		t.Errorf("expected minor1 < major2")
	}

	// Reflexive equality
	if final.Compare(final) != 0 {
		t.Errorf("expected final == final")
	}
	if rc2.Compare(rc2) != 0 {
		t.Errorf("expected rc2 == rc2")
	}
}

func TestQualifiesForChannel(t *testing.T) {
	currentStable, _ := ParseVersion("v1.0.0")
	currentPreview, _ := ParseVersion("v1.0.0-rc.2")

	// Candidate versions
	v100Final, _ := ParseVersion("v1.0.0")
	v101Patch, _ := ParseVersion("v1.0.1")
	v101RC1, _ := ParseVersion("v1.0.1-rc.1")
	v110Minor, _ := ParseVersion("v1.1.0")
	v110RC1, _ := ParseVersion("v1.1.0-rc.1")
	v200Major, _ := ParseVersion("v2.0.0")

	// 1. Stable Channel
	t.Run("Stable Channel", func(t *testing.T) {
		// Older or equal rejected
		if QualifiesForChannel(currentStable, currentStable, ChannelStable) {
			t.Errorf("stable should reject identical current version")
		}
		// Pre-releases rejected on stable channel
		if QualifiesForChannel(v101RC1, currentStable, ChannelStable) {
			t.Errorf("stable channel must reject pre-releases")
		}
		if QualifiesForChannel(v110RC1, currentStable, ChannelStable) {
			t.Errorf("stable channel must reject pre-releases")
		}
		// Newer releases accepted
		if !QualifiesForChannel(v101Patch, currentStable, ChannelStable) {
			t.Errorf("stable channel must accept newer patch")
		}
		if !QualifiesForChannel(v110Minor, currentStable, ChannelStable) {
			t.Errorf("stable channel must accept newer minor")
		}
		if !QualifiesForChannel(v200Major, currentStable, ChannelStable) {
			t.Errorf("stable channel must accept newer major")
		}
	})

	// 2. Security-Only Channel
	t.Run("Security-Only Channel", func(t *testing.T) {
		// Rejects pre-releases
		if QualifiesForChannel(v101RC1, currentStable, ChannelSecurityOnly) {
			t.Errorf("security-only must reject pre-releases")
		}
		// Rejects higher minor and major
		if QualifiesForChannel(v110Minor, currentStable, ChannelSecurityOnly) {
			t.Errorf("security-only must reject minor updates")
		}
		if QualifiesForChannel(v200Major, currentStable, ChannelSecurityOnly) {
			t.Errorf("security-only must reject major updates")
		}
		// Accepts newer patch
		if !QualifiesForChannel(v101Patch, currentStable, ChannelSecurityOnly) {
			t.Errorf("security-only must accept patch updates")
		}
		// When running preview v1.0.0-rc.2, graduating to final v1.0.0 is permitted
		if !QualifiesForChannel(v100Final, currentPreview, ChannelSecurityOnly) {
			t.Errorf("security-only must permit graduation from preview to final patch")
		}
	})

	// 3. Preview Channel
	t.Run("Preview Channel", func(t *testing.T) {
		// Accepts newer pre-release
		if !QualifiesForChannel(v101RC1, currentStable, ChannelPreview) {
			t.Errorf("preview channel must accept newer pre-release")
		}
		// Accepts newer stable
		if !QualifiesForChannel(v101Patch, currentStable, ChannelPreview) {
			t.Errorf("preview channel must accept newer stable")
		}
		// Rejects older or equal
		if QualifiesForChannel(currentPreview, currentStable, ChannelPreview) {
			t.Errorf("preview channel must enforce monotonic floor (cannot downgrade)")
		}
	})
}

func TestSelectLatestQualifyingTag(t *testing.T) {
	current, _ := ParseVersion("v1.0.0")
	tags := []string{
		"v0.9.0",
		"v1.0.0",
		"v1.0.1-rc.1",
		"v1.0.1-rc.2",
		"v1.0.1",
		"v1.1.0-rc.1",
		"invalid-tag",
	}

	// Stable: should select v1.0.1
	v, tag, found := SelectLatestQualifyingTag(tags, current, ChannelStable)
	if !found || tag != "v1.0.1" || v.String() != "v1.0.1" {
		t.Errorf("stable: expected v1.0.1, got %q (found: %v)", tag, found)
	}

	// Security-only: should select v1.0.1
	v, tag, found = SelectLatestQualifyingTag(tags, current, ChannelSecurityOnly)
	if !found || tag != "v1.0.1" || v.String() != "v1.0.1" {
		t.Errorf("security-only: expected v1.0.1, got %q (found: %v)", tag, found)
	}

	// Preview: should select v1.1.0-rc.1 (highest SemVer)
	v, tag, found = SelectLatestQualifyingTag(tags, current, ChannelPreview)
	if !found || tag != "v1.1.0-rc.1" || v.String() != "v1.1.0-rc.1" {
		t.Errorf("preview: expected v1.1.0-rc.1, got %q (found: %v)", tag, found)
	}
}
