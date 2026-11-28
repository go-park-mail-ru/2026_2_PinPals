package repository

import (
	"context"
	"errors"
	"strconv"

	"2026_2_PinPals/onlineconf_service/domain"
	"github.com/jackc/pgx/v5"
)

type Group struct {
	Name    string   `json:"name"`
	Roles   []string `json:"roles"`
	Members []int    `json:"members"`
}
type Admin struct {
	User  int      `json:"user_id"`
	Roles []string `json:"roles"`
}
type AccessInput struct {
	domain.Role
	Roles   []string `json:"roles"`
	Members []int    `json:"members"`
	User    int      `json:"user_id"`
}

const groupQuery = `SELECT jsonb_build_object('name',name,'roles',ARRAY(SELECT role FROM onlineconf.group_role WHERE group_name=name ORDER BY role),
 'members',ARRAY(SELECT user_id FROM onlineconf.member WHERE group_name=name ORDER BY user_id)) FROM onlineconf.admin_group`
const adminQuery = `SELECT jsonb_build_object('user_id',user_id,'roles',array_agg(role ORDER BY role)) FROM onlineconf.user_role`

func AccessList(ctx context.Context, tx pgx.Tx, kind string) (any, error) {
	switch kind {
	case "roles":
		return JSONRows[domain.Role](ctx, tx, "SELECT to_jsonb(r) FROM onlineconf.role r ORDER BY name")
	case "groups":
		return JSONRows[Group](ctx, tx, groupQuery+" ORDER BY name")
	case "admins":
		return JSONRows[Admin](ctx, tx, adminQuery+" GROUP BY user_id ORDER BY user_id")
	}
	return nil, domain.ErrMissing
}

// An access manager can delegate only the permissions they hold in each scope.
func canDelegate(roles domain.Rights, target domain.Role) bool {
	for _, permission := range target.Permissions {
		if !roles.Allows(permission, target.Scopes) {
			return false
		}
	}
	return true
}

func checkRoles(ctx context.Context, tx pgx.Tx, rights domain.Rights, names []string) error {
	for _, name := range names {
		var role domain.Role
		err := tx.QueryRow(ctx, "SELECT to_jsonb(r) FROM onlineconf.role r WHERE name=$1", name).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalid
		}
		if err != nil {
			return err
		}
		if !canDelegate(rights, role) {
			return domain.ErrDenied
		}
	}
	return nil
}

func SaveAccess(ctx context.Context, tx pgx.Tx, kind string, in AccessInput, actor int, remove bool, rights domain.Rights) error {
	var before, after any
	var err error
	id := in.Name
	if kind != "admins" && !domain.Key.MatchString(id) {
		return domain.ErrInvalid
	}
	switch kind {
	case "roles":
		if id == "owner" {
			return domain.ErrDenied
		}
		var old domain.Role
		err = tx.QueryRow(ctx, "SELECT to_jsonb(r) FROM onlineconf.role r WHERE name=$1", id).Scan(&old)
		if err == nil {
			before = old
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !canDelegate(rights, old) || (!remove && !canDelegate(rights, in.Role)) {
			return domain.ErrDenied
		}
		if remove {
			_, err = tx.Exec(ctx, "DELETE FROM onlineconf.role WHERE name=$1", id)
		} else {
			if len(in.Permissions) == 0 || len(in.Scopes) == 0 || !domain.Subset(in.Permissions, domain.Permissions) || !domain.Subset(in.Scopes, domain.Scopes) {
				return domain.ErrInvalid
			}
			after = in.Role
			_, err = tx.Exec(ctx, `INSERT INTO onlineconf.role VALUES($1,$2,$3) ON CONFLICT(name) DO UPDATE SET permissions=excluded.permissions,scopes=excluded.scopes`, id, in.Permissions, in.Scopes)
		}
	case "groups":
		var old Group
		err = tx.QueryRow(ctx, groupQuery+" WHERE name=$1", id).Scan(&old)
		if err == nil {
			before = old
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = checkRoles(ctx, tx, rights, append(old.Roles, in.Roles...)); err != nil {
			return err
		}
		if remove {
			_, err = tx.Exec(ctx, "DELETE FROM onlineconf.admin_group WHERE name=$1", id)
		} else {
			if _, err = tx.Exec(ctx, "INSERT INTO onlineconf.admin_group VALUES($1) ON CONFLICT DO NOTHING", id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "DELETE FROM onlineconf.group_role WHERE group_name=$1", id); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, "DELETE FROM onlineconf.member WHERE group_name=$1", id); err != nil {
				return err
			}
			for _, user := range in.Members {
				if err = ActiveUser(ctx, tx, user); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, "INSERT INTO onlineconf.member(group_name,user_id) SELECT $1,unnest($2::int[])", id, in.Members); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, "INSERT INTO onlineconf.group_role SELECT $1,unnest($2::text[])", id, in.Roles)
			after = Group{id, in.Roles, in.Members}
		}
	case "admins":
		id = strconv.Itoa(in.User)
		if err = ActiveUser(ctx, tx, in.User); err != nil {
			return err
		}
		var old Admin
		err = tx.QueryRow(ctx, adminQuery+" WHERE user_id=$1 GROUP BY user_id", in.User).Scan(&old)
		if err == nil {
			before = old
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err = checkRoles(ctx, tx, rights, append(old.Roles, in.Roles...)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "DELETE FROM onlineconf.user_role WHERE user_id=$1", in.User); err != nil {
			return err
		}
		if !remove {
			_, err = tx.Exec(ctx, "INSERT INTO onlineconf.user_role SELECT $1,unnest($2::text[])", in.User, in.Roles)
			after = Admin{in.User, in.Roles}
		}
	default:
		return domain.ErrMissing
	}
	if err != nil {
		return err
	}
	if remove && before == nil {
		return domain.ErrMissing
	}
	var owner bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM onlineconf.user_role r JOIN public."user" u USING(user_id) WHERE role='owner' AND u.deleted_at IS NULL)`).Scan(&owner); err != nil {
		return err
	}
	if !owner {
		return domain.ErrConflict
	}
	action := "assign"
	if remove {
		action = "revoke"
	}
	return Audit(ctx, tx, kind, id, id, action, domain.Scopes, actor, 1, before, after, "")
}
