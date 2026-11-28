package domain

import (
	"slices"
	"time"
)

type Plan struct {
	Name          string    `json:"name"`
	Stages        []int     `json:"stages"`
	Stage         int       `json:"stage"`
	Duration      int       `json:"duration_seconds"`
	ApproverRole  string    `json:"approver_role"`
	ApprovalState string    `json:"approval_state,omitempty"`
	Status        string    `json:"status"`
	Started       time.Time `json:"started_at"`
}

func (p Plan) Waiting(now time.Time) bool {
	return p.Status == "running" && !now.Before(p.Started.Add(time.Duration(p.Duration)*time.Second))
}
func (p Plan) Live() bool { return p.Status == "running" || p.Status == "paused" }
func (r *Resource) Rollout(action string, input *Plan, roles Rights, now time.Time) error {
	p := r.Plan
	if action == "create" {
		if p != nil && p.Live() {
			return ErrConflict
		}
		if input == nil || input.Name == "" || len(input.Stages) < 2 || input.Duration < 0 || input.Duration > 31536000 {
			return ErrInvalid
		}
		for i, n := range input.Stages {
			if n < 0 || n > 100 || (i > 0 && n <= input.Stages[i-1]) {
				return ErrInvalid
			}
		}
		input.Stage = 0
		input.Started = now
		input.Status = "running"
		r.Plan = input
		r.Percent = input.Stages[0]
		return nil
	}
	if action == "emergency" {
		r.Active = false
		if p != nil {
			p.Status = "stopped"
		}
		return nil
	}
	if p == nil {
		return ErrConflict
	}
	switch action {
	case "approve":
		if !p.Waiting(now) || p.Stage+1 >= len(p.Stages) {
			return ErrConflict
		}
		if p.ApproverRole != "" && !slices.ContainsFunc(roles, func(role Role) bool { return role.Name == p.ApproverRole }) {
			return ErrDenied
		}
		p.Stage++
		p.Started = now
		r.Percent = p.Stages[p.Stage]
		if p.Stage == len(p.Stages)-1 {
			p.Status = "complete"
		}
	case "pause", "reject":
		if p.Status != "running" {
			return ErrConflict
		}
		p.Status = "paused"
	case "resume":
		if p.Status != "paused" {
			return ErrConflict
		}
		p.Status = "running"
		p.Started = now
	case "stop":
		if !p.Live() {
			return ErrConflict
		}
		p.Status = "stopped"
	case "rollback":
		if p.Stage < 1 {
			return ErrConflict
		}
		p.Stage--
		r.Percent = p.Stages[p.Stage]
		p.Status = "paused"
		p.Started = now
	default:
		return ErrInvalid
	}
	return nil
}
