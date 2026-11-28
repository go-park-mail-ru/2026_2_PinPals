package domain

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	sdk "2026_2_PinPals/pkg/onlineconf"
)

var Scopes = []string{"frontend", "auth_service", "pin_service", "user_service"}
var Permissions = strings.Fields("config.read config.create config.update config.delete flag.read flag.create flag.update flag.delete flag.evaluate rollout.create rollout.update rollout.approve rollout.pause rollout.stop rollout.emergency history.read admins.manage groups.manage roles.manage")
var Key = regexp.MustCompile("^[a-z][a-z0-9._-]{0,127}$")
var sensitive = regexp.MustCompile("(?i)(password|passwd|secret|token|private[._ -]?key|api[._ -]?key|BEGIN [A-Z ]*PRIVATE KEY)")
var ErrConflict = errors.New("version_or_state_conflict")
var ErrDenied = errors.New("forbidden")
var ErrInvalid = errors.New("invalid_input")
var ErrMissing = errors.New("not_found")

type Role struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Scopes      []string `json:"scopes"`
}
type Rights []Role

func (roles Rights) Allows(permission string, scopes []string) bool {
	for _, scope := range scopes {
		found := false
		for _, role := range roles {
			if slices.Contains(role.Permissions, permission) && slices.Contains(role.Scopes, scope) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(scopes) > 0
}

type Resource struct {
	sdk.Flag
	Kind        string          `json:"kind"`
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Scopes      []string        `json:"scopes"`
	ValueType   string          `json:"value_type"`
	Value       json.RawMessage `json:"value"`
	Visibility  string          `json:"visibility"`
	Archived    bool            `json:"archived"`
	Version     int64           `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   *int            `json:"created_by"`
	UpdatedBy   *int            `json:"updated_by"`
	Plan        *Plan           `json:"plan"`
}

func Subset[T comparable](items, allowed []T) bool {
	seen := map[T]bool{}
	for _, item := range items {
		if seen[item] || !slices.Contains(allowed, item) {
			return false
		}
		seen[item] = true
	}
	return true
}
func (r Resource) Validate() error {
	if !Key.MatchString(r.Key) || strings.TrimSpace(r.Title) == "" || len(r.Scopes) == 0 || !Subset(r.Scopes, Scopes) {
		return ErrInvalid
	}
	if r.Key == sdk.PageSizeKey || r.Key == sdk.LikesKey {
		if !slices.Contains(r.Scopes, "frontend") || !slices.Contains(r.Scopes, "pin_service") {
			return ErrInvalid
		}
	}
	if r.Kind == "flag" {
		if r.Key == sdk.PageSizeKey || r.Percent < 0 || r.Percent > 100 {
			return ErrInvalid
		}
		seen := map[int]bool{}
		for _, list := range [][]int{r.Allow, r.Deny} {
			for _, id := range list {
				if id < 1 || seen[id] {
					return ErrInvalid
				}
				seen[id] = true
			}
		}
		return nil
	}
	if r.Kind != "config" || r.Key == sdk.LikesKey || (r.Visibility != "public" && r.Visibility != "internal") {
		return ErrInvalid
	}
	if r.Visibility == "public" && !slices.Contains(r.Scopes, "frontend") {
		return ErrInvalid
	}
	if sensitive.MatchString(r.Key) || sensitive.Match(r.Value) {
		return errors.New("sensitive_value")
	}
	var value any
	if json.Unmarshal(r.Value, &value) != nil {
		return ErrInvalid
	}
	valid := false
	switch r.ValueType {
	case "null":
		valid = value == nil
	case "string":
		_, valid = value.(string)
	case "number":
		_, valid = value.(float64)
	case "integer":
		var n int64
		valid = json.Unmarshal(r.Value, &n) == nil && value != nil
	case "boolean":
		_, valid = value.(bool)
	case "array":
		_, valid = value.([]any)
	case "object":
		_, valid = value.(map[string]any)
	}
	if !valid {
		return ErrInvalid
	}
	if r.Key == sdk.PageSizeKey {
		n, ok := value.(float64)
		if !ok || r.ValueType != "integer" || r.Visibility != "public" || n < 1 || n > 100 || math.Trunc(n) != n {
			return ErrInvalid
		}
	}
	return nil
}
