package delivery

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"2026_2_PinPals/onlineconf_service/domain"
	"2026_2_PinPals/onlineconf_service/repository"
	"2026_2_PinPals/pkg/httpx"
	"2026_2_PinPals/pkg/middleware"
	sdk "2026_2_PinPals/pkg/onlineconf"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	DB      *pgxpool.Pool
	mu      sync.Mutex
	catalog atomic.Pointer[repository.Catalog]
}

func (s *Server) publish(c repository.Catalog) {
	s.catalog.Store(&c)
	sdk.MarkSynced(c["frontend"].Revision)
}
func (s *Server) Refresh(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	c, err := repository.Load(ctx, tx)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.publish(c)
	return nil
}
func (s *Server) Poll(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Refresh(ctx); err != nil {
				sdk.SyncFailed()
				slog.Warn("catalog refresh failed")
			}
		}
	}
}
func (s *Server) Routes(mux *http.ServeMux, tm *middleware.TokenManager, token string) {
	const base = "/api/v1/onlineconf/"
	mux.Handle("GET "+base+"client-state", tm.OptionalMiddleware(http.HandlerFunc(s.clientState)))
	mux.HandleFunc("GET /internal/v1/onlineconf/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if token == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			httpx.WriteError(w, 403, "forbidden")
			return
		}
		scope := r.URL.Query().Get("scope")
		if scope == "frontend" || !slices.Contains(domain.Scopes, scope) {
			httpx.WriteError(w, 400, "invalid_scope")
			return
		}
		httpx.WriteJSON(w, 200, (*s.catalog.Load())[scope])
	})
	admin := http.NewServeMux()
	admin.HandleFunc("GET /me", s.handle(s.me, false))
	admin.HandleFunc("GET /history", s.handle(s.history, false))
	for _, kind := range []string{"configs", "flags"} {
		collection := "/" + kind
		admin.HandleFunc("GET "+collection, s.handle(s.list, false))
		admin.HandleFunc("GET "+collection+"/{id}", s.handle(s.get, false))
		admin.HandleFunc("POST "+collection, s.handle(s.save, true))
		admin.HandleFunc("PATCH "+collection+"/{id}", s.handle(s.save, true))
		admin.HandleFunc("DELETE "+collection+"/{id}", s.handle(s.save, true))
	}
	admin.HandleFunc("POST /flags/{id}/evaluate", s.handle(s.evaluate, false))
	admin.HandleFunc("POST /flags/{id}/rollout/{action}", s.handle(s.rollout, true))
	for _, kind := range []string{"admins", "roles", "groups"} {
		admin.HandleFunc("GET /"+kind, s.handle(s.access, false))
		admin.HandleFunc("POST /"+kind, s.handle(s.access, true))
		admin.HandleFunc("DELETE /"+kind, s.handle(s.access, true))
	}
	mux.Handle(base+"admin/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		tm.Middleware(middleware.CSRF(http.StripPrefix(base+"admin", admin))).ServeHTTP(w, r)
	}))
}
func (s *Server) clientState(w http.ResponseWriter, r *http.Request) {
	snap := (*s.catalog.Load())["frontend"]
	user, _ := middleware.UserIDFromContext(r.Context())
	tag := fmt.Sprintf("\"%d-%d\"", snap.Revision, user)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("ETag", tag)
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(304)
		return
	}
	flags := map[string]bool{}
	for key, f := range snap.Flags {
		sdk.CountDecision()
		flags[key] = f.Evaluate(user).Enabled
	}
	httpx.WriteJSON(w, 200, map[string]any{"revision": snap.Revision, "configs": snap.Configs, "flags": flags})
}

type request struct {
	*http.Request
	tx    pgx.Tx
	roles domain.Rights
	user  int
}
type operation func(request) (any, error)

func (s *Server) handle(fn operation, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if write {
			s.mu.Lock()
			defer s.mu.Unlock()
		}
		options := pgx.TxOptions{}
		if !write {
			options = pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead}
		}
		tx, err := s.DB.BeginTx(ctx, options)
		if err != nil {
			writeError(w, err)
			return
		}
		defer tx.Rollback(ctx)
		// Serialize admin writes across service instances, including ACL checks.
		if write {
			if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(780026)"); err != nil {
				writeError(w, err)
				return
			}
		}
		user, _ := middleware.UserIDFromContext(ctx)
		roles, err := repository.Rights(ctx, tx, user)
		if err != nil {
			writeError(w, err)
			return
		}
		if len(roles) == 0 {
			writeError(w, domain.ErrDenied)
			return
		}
		result, err := fn(request{r, tx, roles, user})
		var catalog repository.Catalog
		if err == nil && write {
			catalog, err = repository.Load(ctx, tx)
		}
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err != nil {
			writeError(w, err)
			return
		}
		if write {
			s.publish(catalog)
			slog.Info("admin change", "actor", user, "method", r.Method, "path", r.URL.Path)
		}
		status := 200
		if r.Method == "POST" && (r.URL.Path == "/configs" || r.URL.Path == "/flags") {
			status = 201
		}
		if r.Method == "DELETE" {
			w.WriteHeader(204)
			return
		}
		httpx.WriteJSON(w, status, result)
	}
}
func writeError(w http.ResponseWriter, err error) {
	status, code := 500, "internal_error"
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, domain.ErrInvalid), err.Error() == "sensitive_value":
		status, code = 422, "invalid_input"
	case errors.Is(err, domain.ErrDenied):
		status, code = 403, "forbidden"
	case errors.Is(err, domain.ErrConflict):
		status, code = 409, "version_or_state_conflict"
	case errors.Is(err, domain.ErrMissing):
		status, code = 404, "not_found"
	case errors.As(err, &pg):
		switch pg.Code {
		case "23505", "40001":
			status, code = 409, "conflict"
		case "23503", "23514", "22P02":
			status, code = 422, "invalid_reference"
		}
	}
	slog.Warn("admin request rejected", "status", status, "code", code)
	httpx.WriteError(w, status, code)
}
func body(r request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.ErrInvalid
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return domain.ErrInvalid
	}
	return nil
}
