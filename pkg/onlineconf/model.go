package onlineconf

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"slices"
)

const PageSizeKey = "pins.pagination.default_page_size"
const LikesKey = "pins.likes.enabled"

type Flag struct {
	ID      int64 `json:"id"`
	Active  bool  `json:"active"`
	Percent int   `json:"rollout_percent"`
	Allow   []int `json:"allowlist"`
	Deny    []int `json:"denylist"`
}
type Decision struct {
	Enabled  bool   `json:"enabled"`
	Bucket   int    `json:"bucket"`
	Percent  int    `json:"rollout_percent"`
	Included bool   `json:"included"`
	Excluded bool   `json:"excluded"`
	Reason   string `json:"reason"`
}

// The first eight SHA-256 bytes are an unsigned big-endian integer.
// Anonymous readers see only features rolled out to everyone.
func (f Flag) Evaluate(user int) Decision {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", f.ID, user)))
	d := Decision{Bucket: int(binary.BigEndian.Uint64(hash[:8]) % 100), Percent: f.Percent,
		Included: slices.Contains(f.Allow, user), Excluded: slices.Contains(f.Deny, user)}
	switch {
	case !f.Active:
		d.Reason = "global_disabled"
	case d.Excluded:
		d.Reason = "user_excluded"
	case d.Included:
		d.Enabled = true
		d.Reason = "user_included"
	case user == 0 && f.Percent < 100:
		d.Reason = "rollout_miss"
	case d.Bucket < f.Percent:
		d.Enabled = true
		d.Reason = "rollout_match"
	default:
		d.Reason = "rollout_miss"
	}
	return d
}

type Snapshot struct {
	Scope    string                     `json:"scope"`
	Revision int64                      `json:"revision"`
	Configs  map[string]json.RawMessage `json:"configs"`
	Flags    map[string]Flag            `json:"flags"`
}

func NewSnapshot(scope string, revision int64) *Snapshot {
	return &Snapshot{scope, revision, map[string]json.RawMessage{}, map[string]Flag{}}
}
func (s *Snapshot) PageSize() int {
	var n int
	if s != nil && json.Unmarshal(s.Configs[PageSizeKey], &n) == nil && n >= 1 && n <= 100 {
		return n
	}
	return 30
}
