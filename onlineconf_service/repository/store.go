package repository

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"2026_2_PinPals/onlineconf_service/domain"
	sdk "2026_2_PinPals/pkg/onlineconf"
	"github.com/jackc/pgx/v5"
)

func JSONRows[T any](ctx context.Context, tx pgx.Tx, query string, args ...any) ([]T, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []T{}
	for rows.Next() {
		var item T
		if err := rows.Scan(&item); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}
func Rights(ctx context.Context, tx pgx.Tx, user int) (domain.Rights, error) {
	return JSONRows[domain.Role](ctx, tx, `SELECT to_jsonb(r) FROM onlineconf.effective_roles e
 JOIN onlineconf.role r ON r.name=e.role WHERE e.user_id=$1`, user)
}
func Resource(ctx context.Context, tx pgx.Tx, kind string, id int64) (domain.Resource, error) {
	var item domain.Resource
	err := tx.QueryRow(ctx, `SELECT to_jsonb(d) FROM onlineconf.details d WHERE id=$1 AND kind=$2`, id, kind).Scan(&item)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, domain.ErrMissing
	}
	return item, err
}
func ActiveUser(ctx context.Context, tx pgx.Tx, id int) error {
	var ok bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public."user" WHERE user_id=$1 AND deleted_at IS NULL)`, id).Scan(&ok)
	if err == nil && !ok {
		return domain.ErrInvalid
	}
	return err
}
func Audit(ctx context.Context, tx pgx.Tx, kind, id, key, action string, scopes []string, actor int, version int64, before, after any, comment string) error {
	_, err := tx.Exec(ctx, `INSERT INTO onlineconf.history(kind,resource_id,key,action,scopes,actor_id,version,before_json,after_json,comment)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, kind, id, key, action, scopes, actor, version, before, after, comment)
	return err
}
func Save(ctx context.Context, tx pgx.Tx, r domain.Resource, before *domain.Resource, actor int, action, comment string) (domain.Resource, error) {
	var err error
	if before == nil {
		err = tx.QueryRow(ctx, `INSERT INTO onlineconf.resource(kind,key,title,description,scopes,value_type,value,visibility,active,rollout_percent,created_by,updated_by)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11) RETURNING id`, r.Kind, r.Key, r.Title, r.Description, r.Scopes, r.ValueType, r.Value, r.Visibility, r.Active, r.Percent, actor).Scan(&r.ID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE onlineconf.resource SET key=$2,title=$3,description=$4,scopes=$5,value_type=$6,value=$7,
   visibility=$8,active=$9,rollout_percent=$10,archived=$11,updated_by=$12,updated_at=now(),version=version+1 WHERE id=$1`,
			r.ID, r.Key, r.Title, r.Description, r.Scopes, r.ValueType, r.Value, r.Visibility, r.Active, r.Percent, r.Archived, actor)
	}
	if err != nil {
		return r, err
	}
	if r.Kind == "flag" {
		if before == nil || !slices.Equal(r.Allow, before.Allow) || !slices.Equal(r.Deny, before.Deny) {
			if err = saveOverrides(ctx, tx, r, actor); err != nil {
				return r, err
			}
		}
		if p := r.Plan; p != nil {
			_, err = tx.Exec(ctx, `INSERT INTO onlineconf.plan(flag_id,name,stages,stage,duration_seconds,approver_role,status,started_at)
    VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(flag_id) DO UPDATE SET name=excluded.name, stages=excluded.stages,
    stage=excluded.stage,duration_seconds=excluded.duration_seconds,approver_role=excluded.approver_role,status=excluded.status,started_at=excluded.started_at`,
				r.ID, p.Name, p.Stages, p.Stage, p.Duration, p.ApproverRole, p.Status, p.Started)
			if err != nil {
				return r, err
			}
		}
	}
	saved, err := Resource(ctx, tx, r.Kind, r.ID)
	if err != nil {
		return saved, err
	}
	scopes := append([]string{}, r.Scopes...)
	if before != nil {
		scopes = append(scopes, before.Scopes...)
	}
	err = Audit(ctx, tx, r.Kind, strconv.FormatInt(r.ID, 10), r.Key, action, scopes, actor, saved.Version, before, saved, comment)
	return saved, err
}
func saveOverrides(ctx context.Context, tx pgx.Tx, r domain.Resource, actor int) error {
	var invalid bool
	all := append(append([]int{}, r.Allow...), r.Deny...)
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT unnest($1::int[]) EXCEPT SELECT user_id FROM public."user" WHERE deleted_at IS NULL)`, all).Scan(&invalid)
	if err != nil {
		return err
	}
	if invalid {
		return domain.ErrInvalid
	}
	_, err = tx.Exec(ctx, `DELETE FROM onlineconf.override WHERE flag_id=$1 AND NOT(user_id=ANY($2::int[]))`, r.ID, all)
	if err != nil {
		return err
	}
	for included, list := range map[bool][]int{true: r.Allow, false: r.Deny} {
		_, err = tx.Exec(ctx, `INSERT INTO onlineconf.override(flag_id,user_id,included,actor_id)
   SELECT $1,unnest($2::int[]),$3,$4 ON CONFLICT(flag_id,user_id) DO UPDATE SET included=excluded.included,actor_id=excluded.actor_id,created_at=now()
   WHERE onlineconf.override.included<>excluded.included`, r.ID, list, included, actor)
		if err != nil {
			return err
		}
	}
	return nil
}

type Catalog map[string]*sdk.Snapshot

func Load(ctx context.Context, tx pgx.Tx) (Catalog, error) {
	var revision int64
	if err := tx.QueryRow(ctx, "SELECT COALESCE(max(id),1) FROM onlineconf.history").Scan(&revision); err != nil {
		return nil, err
	}
	resources, err := JSONRows[domain.Resource](ctx, tx, "SELECT to_jsonb(d) FROM onlineconf.details d WHERE NOT archived")
	if err != nil {
		return nil, err
	}
	catalog := Catalog{}
	for _, scope := range domain.Scopes {
		catalog[scope] = sdk.NewSnapshot(scope, revision)
	}
	for _, r := range resources {
		for _, scope := range r.Scopes {
			if r.Kind == "flag" {
				catalog[scope].Flags[r.Key] = r.Flag
			} else if r.Active && (scope != "frontend" || r.Visibility == "public") {
				catalog[scope].Configs[r.Key] = r.Value
			}
		}
	}
	return catalog, nil
}

type Filter struct{ Query, Scope, Type, Status string }

func List(ctx context.Context, tx pgx.Tx, kind string, f Filter, rights domain.Rights) ([]domain.Resource, error) {
	items, err := JSONRows[domain.Resource](ctx, tx, `SELECT to_jsonb(d) FROM onlineconf.details d WHERE kind=$1
 AND ($2='' OR strpos(lower(key||' '||title),lower($2))>0) AND ($3='' OR $3=ANY(scopes))
 AND ($4='' OR value_type=$4) AND ($5='' OR $5=CASE WHEN archived THEN 'archived' WHEN active THEN 'active' ELSE 'inactive' END)
 ORDER BY key`, kind, f.Query, f.Scope, f.Type, f.Status)
	if err != nil {
		return nil, err
	}
	visible := []domain.Resource{}
	for _, r := range items {
		if rights.Allows(kind+".read", r.Scopes) {
			visible = append(visible, r)
		}
	}
	return visible, nil
}

type HistoryFilter struct{ Key, Actor, Action, From, To string }

func History(ctx context.Context, tx pgx.Tx, f HistoryFilter, roles domain.Rights) ([]json.RawMessage, error) {
	for _, date := range []string{f.From, f.To} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return nil, domain.ErrInvalid
			}
		}
	}
	scopes := []string{}
	for _, s := range domain.Scopes {
		if roles.Allows("history.read", []string{s}) {
			scopes = append(scopes, s)
		}
	}
	return JSONRows[json.RawMessage](ctx, tx, `SELECT to_jsonb(h)-'scopes' FROM onlineconf.history h WHERE scopes <@ $1::text[]
 AND ($2='' OR key=$2 OR resource_id=$2) AND ($3='' OR actor_id::text=$3) AND ($4='' OR action=$4)
 AND (NULLIF($5,'') IS NULL OR created_at>=NULLIF($5,'')::date)
 AND (NULLIF($6,'') IS NULL OR created_at<NULLIF($6,'')::date+1) ORDER BY id DESC`, scopes, f.Key, f.Actor, f.Action, f.From, f.To)
}
