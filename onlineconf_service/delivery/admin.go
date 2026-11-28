package delivery

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"time"

	"2026_2_PinPals/onlineconf_service/domain"
	"2026_2_PinPals/onlineconf_service/repository"
	sdk "2026_2_PinPals/pkg/onlineconf"
)

func (r request) kind() string {
	return strings.TrimSuffix(strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")[0], "s")
}
func (r request) item(permission string) (domain.Resource, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return domain.Resource{}, domain.ErrInvalid
	}
	item, err := repository.Resource(r.Context(), r.tx, r.kind(), id)
	if err == nil && !r.roles.Allows(permission, item.Scopes) {
		err = domain.ErrDenied
	}
	return item, err
}
func (r request) anyScope(permission string) bool {
	for _, scope := range domain.Scopes {
		if r.roles.Allows(permission, []string{scope}) {
			return true
		}
	}
	return false
}
func (s *Server) me(r request) (any, error) {
	return map[string]any{"user_id": r.user, "roles": r.roles, "permissions": domain.Permissions, "scopes": domain.Scopes}, nil
}
func (s *Server) list(r request) (any, error) {
	if !r.anyScope(r.kind() + ".read") {
		return nil, domain.ErrDenied
	}
	q := r.URL.Query()
	return repository.List(r.Context(), r.tx, r.kind(), repository.Filter{Query: q.Get("q"), Scope: q.Get("scope"), Type: q.Get("type"), Status: q.Get("status")}, r.roles)
}
func (s *Server) get(r request) (any, error) { return r.item(r.kind() + ".read") }
func (s *Server) history(r request) (any, error) {
	if !r.anyScope("history.read") {
		return nil, domain.ErrDenied
	}
	q := r.URL.Query()
	return repository.History(r.Context(), r.tx, repository.HistoryFilter{Key: q.Get("resource"), Actor: q.Get("actor"), Action: q.Get("action"), From: q.Get("from"), To: q.Get("to")}, r.roles)
}
func (s *Server) access(r request) (any, error) {
	kind := strings.TrimPrefix(r.URL.Path, "/")
	if !r.roles.Allows(kind+".manage", domain.Scopes) {
		return nil, domain.ErrDenied
	}
	if r.Method == "GET" {
		return repository.AccessList(r.Context(), r.tx, kind)
	}
	var in repository.AccessInput
	if err := body(r, &in); err != nil {
		return nil, err
	}
	return in, repository.SaveAccess(r.Context(), r.tx, kind, in, r.user, r.Method == "DELETE", r.roles)
}
func (s *Server) evaluate(r request) (any, error) {
	item, err := r.item("flag.evaluate")
	if err != nil {
		return nil, err
	}
	var in struct {
		User int `json:"user_id"`
	}
	if err = body(r, &in); err != nil {
		return nil, err
	}
	if err = repository.ActiveUser(r.Context(), r.tx, in.User); err != nil {
		return nil, err
	}
	item.Active = item.Active && !item.Archived
	sdk.CountDecision()
	return item.Evaluate(in.User), nil
}

type edit struct {
	Key         string          `json:"key"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Scopes      []string        `json:"scopes"`
	ValueType   string          `json:"value_type"`
	Value       json.RawMessage `json:"value"`
	Visibility  string          `json:"visibility"`
	Active      bool            `json:"active"`
	Percent     int             `json:"rollout_percent"`
	Allow       []int           `json:"allowlist"`
	Deny        []int           `json:"denylist"`
	Version     int64           `json:"version"`
	Comment     string          `json:"comment"`
}

func (s *Server) save(r request) (any, error) {
	kind := r.kind()
	action := map[string]string{"POST": "create", "PATCH": "update", "DELETE": "delete"}[r.Method]
	permission := kind + "." + action
	var before *domain.Resource
	item := domain.Resource{Kind: kind}
	input := edit{Active: true, Visibility: "internal"}
	if r.Method != "POST" {
		old, err := r.item(permission)
		if err != nil {
			return nil, err
		}
		if old.Archived {
			return nil, domain.ErrConflict
		}
		before = &old
		item = old
		input = edit{Key: old.Key, Title: old.Title, Description: old.Description, Scopes: slices.Clone(old.Scopes), ValueType: old.ValueType, Value: slices.Clone(old.Value),
			Visibility: old.Visibility, Active: old.Active, Percent: old.Percent, Allow: slices.Clone(old.Allow), Deny: slices.Clone(old.Deny)}
	}
	if err := body(r, &input); err != nil {
		return nil, err
	}
	if before != nil && input.Version != before.Version {
		return nil, domain.ErrConflict
	}
	if r.Method == "DELETE" {
		item.Archived = true
		action = "archive"
	} else {
		item.Key = input.Key
		item.Title = input.Title
		item.Description = input.Description
		item.Scopes = input.Scopes
		item.ValueType = input.ValueType
		item.Value = input.Value
		item.Visibility = input.Visibility
		item.Active = input.Active
		item.Percent = input.Percent
		item.Allow = input.Allow
		item.Deny = input.Deny
		if err := item.Validate(); err != nil {
			return nil, err
		}
		if item.Kind == "flag" && (item.ValueType != "" || (len(item.Value) > 0 && string(item.Value) != "null")) {
			return nil, domain.ErrInvalid
		}
		if item.Kind == "config" && (len(item.Allow)+len(item.Deny) > 0 || item.Percent != 0) {
			return nil, domain.ErrInvalid
		}
		if before != nil && item.Plan != nil && item.Plan.Live() && item.Percent != before.Percent {
			return nil, domain.ErrConflict
		}
		if before != nil && before.Active != item.Active {
			if item.Active {
				action = "enable"
			} else {
				action = "disable"
			}
		}
	}
	if !r.roles.Allows(permission, item.Scopes) {
		return nil, domain.ErrDenied
	}
	return repository.Save(r.Context(), r.tx, item, before, r.user, action, input.Comment)
}
func (s *Server) rollout(r request) (any, error) {
	action := r.PathValue("action")
	permissions := map[string]string{"create": "create", "approve": "approve", "reject": "approve", "pause": "pause", "resume": "update", "stop": "stop", "rollback": "update", "emergency": "emergency"}
	perm, ok := permissions[action]
	if !ok {
		return nil, domain.ErrMissing
	}
	old, err := r.item("rollout." + perm)
	if err != nil {
		return nil, err
	}
	var in struct {
		Version int64        `json:"version"`
		Plan    *domain.Plan `json:"plan"`
		Comment string       `json:"comment"`
	}
	if err = body(r, &in); err != nil {
		return nil, err
	}
	if old.Archived || old.Version != in.Version {
		return nil, domain.ErrConflict
	}
	item := old
	if old.Plan != nil {
		copyPlan := *old.Plan
		item.Plan = &copyPlan
	}
	if in.Plan != nil && in.Plan.ApproverRole != "" {
		var exists bool
		err = r.tx.QueryRow(r.Context(), "SELECT EXISTS(SELECT 1 FROM onlineconf.role WHERE name=$1)", in.Plan.ApproverRole).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, domain.ErrInvalid
		}
	}
	if err = item.Rollout(action, in.Plan, r.roles, time.Now()); err != nil {
		return nil, err
	}
	return repository.Save(r.Context(), r.tx, item, &old, r.user, "rollout."+action, in.Comment)
}
