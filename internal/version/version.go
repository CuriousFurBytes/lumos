// Package version computes the next release tag from the latest one using a
// small, predictable set of bump rules. While lumos is pre-1.0 every bump
// produces an alpha pre-release; the Stable level promotes the current
// numbers to a final release.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Level is the kind of version bump to apply.
type Level int

// The available bump levels.
const (
	Prerelease Level = iota // bump the pre-release counter (the default)
	Patch
	Minor
	Major
	Stable // drop the pre-release, finalising the current numbers
)

// defaultPre is the identifier used when starting a fresh pre-release.
const defaultPre = "alpha.1"

var semverRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)

type semver struct {
	major, minor, patch int
	pre                 string // e.g. "alpha.1"; empty for a stable release
}

func parse(tag string) (semver, error) {
	m := semverRe.FindStringSubmatch(strings.TrimSpace(tag))
	if m == nil {
		return semver{}, fmt.Errorf("invalid semver tag %q", tag)
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	return semver{major: atoi(m[1]), minor: atoi(m[2]), patch: atoi(m[3]), pre: m[4]}, nil
}

func (v semver) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
	if v.pre != "" {
		s += "-" + v.pre
	}
	return s
}

// Next returns the next tag after latest for the given bump level. An empty
// latest is treated as v0.0.0 (no releases yet), so the first prerelease or
// patch bump yields v0.0.1-alpha.1.
func Next(latest string, level Level) (string, error) {
	var cur semver
	if strings.TrimSpace(latest) != "" {
		var err error
		if cur, err = parse(latest); err != nil {
			return "", err
		}
	}

	switch level {
	case Prerelease:
		if cur.pre != "" {
			cur.pre = bumpPre(cur.pre)
		} else {
			cur.patch++
			cur.pre = defaultPre
		}
	case Patch:
		cur.patch++
		cur.pre = defaultPre
	case Minor:
		cur.minor++
		cur.patch = 0
		cur.pre = defaultPre
	case Major:
		cur.major++
		cur.minor, cur.patch = 0, 0
		cur.pre = defaultPre
	case Stable:
		if cur.pre != "" {
			cur.pre = ""
		} else {
			cur.patch++
		}
	default:
		return "", fmt.Errorf("unknown bump level %d", level)
	}
	return cur.String(), nil
}

var trailingNum = regexp.MustCompile(`^(.*?)(\d+)$`)

// bumpPre increments the trailing number of a pre-release identifier, e.g.
// "alpha.1" -> "alpha.2" or "beta" -> "beta.1".
func bumpPre(pre string) string {
	if m := trailingNum.FindStringSubmatch(pre); m != nil {
		n, _ := strconv.Atoi(m[2])
		return m[1] + strconv.Itoa(n+1)
	}
	return pre + ".1"
}

// ParseLevel maps a level name to a Level.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "prerelease", "pre", "alpha", "":
		return Prerelease, nil
	case "patch":
		return Patch, nil
	case "minor":
		return Minor, nil
	case "major":
		return Major, nil
	case "stable", "release":
		return Stable, nil
	default:
		return Prerelease, fmt.Errorf("unknown bump level %q", s)
	}
}

// BumpFromMessage derives a bump level from a commit message, honouring an
// explicit [major], [minor], [patch] or [stable] marker. The default is a
// pre-release bump.
func BumpFromMessage(msg string) Level {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "[major]"):
		return Major
	case strings.Contains(m, "[minor]"):
		return Minor
	case strings.Contains(m, "[patch]"):
		return Patch
	case strings.Contains(m, "[stable]"), strings.Contains(m, "[release]"):
		return Stable
	default:
		return Prerelease
	}
}
