package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/evanofslack/prulights/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	sessionCookie = "prulights_session"
	// sessionTTL is how long a browser stays signed in without visiting; each
	// visit (at most daily) pushes it out again.
	sessionTTL     = 365 * 24 * time.Hour
	sessionRefresh = 24 * time.Hour
	// InviteTTL is how long an invite link works.
	InviteTTL = 7 * 24 * time.Hour
	// ceremonyTTL is how long a passkey prompt can take.
	ceremonyTTL = 5 * time.Minute
	// authRateLimit is sign-in attempts per client per minute.
	authRateLimit = 30
	maxNameLen    = 40
)

// InviteURL is the link for an invite token.
func InviteURL(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/invite?t=" + token
}

func newWebAuthn(rpID string, origins []string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: "prulights",
		RPOrigins:     origins,
	})
}

// mountAccount adds sign-in and the signed-in api. Every request learns who's
// signed in; writes must come from our own pages, as json.
func (s *Server) mountAccount(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(s.withUser)
		r.Use(s.sameOrigin)
		r.Get("/me", s.getMe)

		r.Route("/auth", func(r chi.Router) {
			r.Use(httprate.Limit(authRateLimit, time.Minute, httprate.WithKeyFuncs(s.clientKey),
				httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
					writeError(w, http.StatusTooManyRequests, "too many sign-in attempts, wait a minute")
				})))
			r.Get("/invites/{token}", s.getInvite)
			r.Post("/register/begin", s.registerBegin)
			r.Post("/register/finish", s.registerFinish)
			r.Post("/login/begin", s.loginBegin)
			r.Post("/login/finish", s.loginFinish)
			r.Post("/logout", s.logout)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(s.adminOnly)
			r.Post("/invites", s.createInvite)
		})
	})
}

// --- who's signed in

type userKey struct{}

// userFrom is the signed-in user, or nil.
func userFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(userKey{}).(*store.User)
	return u
}

// withUser reads the session cookie. A valid session puts its user in the
// request's context, and a day after it was last seen, slides its expiry.
func (s *Server) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		now := s.now()
		sess, err := s.store.SessionByToken(r.Context(), c.Value, now)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Error("session", "error", err)
			}
			s.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}
		if now.Sub(sess.LastSeenAt) > sessionRefresh {
			if err := s.store.ExtendSession(r.Context(), c.Value, now, sessionTTL); err == nil {
				s.setSessionCookie(w, c.Value, now)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, &sess.User)))
	})
}

func (s *Server) requireUser(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, "sign in first")
			return
		}
		h(w, r)
	}
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r.Context())
		switch {
		case u == nil:
			writeError(w, http.StatusUnauthorized, "sign in first")
		case !u.IsAdmin:
			writeError(w, http.StatusForbidden, "admins only")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// sameOrigin guards writes against other sites: they must carry an Origin we
// serve passkeys on, and a json body when they have one. A form post from
// elsewhere fails both.
func (s *Server) sameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !slices.Contains(s.cfg.RPOrigins, r.Header.Get("Origin")) {
			writeError(w, http.StatusForbidden, "cross-origin request refused")
			return
		}
		if r.ContentLength != 0 && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, http.StatusUnsupportedMediaType, "send json")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, now time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  now.Add(sessionTTL),
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: strings.HasPrefix(s.cfg.BaseURL, "https://"), SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request, u store.User) {
	now := s.now()
	token, err := s.store.CreateSession(r.Context(), u.ID, now, sessionTTL)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.setSessionCookie(w, token, now)
	writeJSON(w, http.StatusOK, meOf(u))
}

func meOf(u store.User) MeResponse {
	return MeResponse{ID: u.ID, Name: u.Name, IsAdmin: u.IsAdmin}
}

// getMe says who's signed in: 204 when nobody is (so pages can ask without
// an error showing up in the browser console), else the user.
func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	u := userFrom(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, meOf(*u))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), c.Value); err != nil {
			s.internalError(w, err)
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// --- passkeys

// waUser is a user as the webauthn library sees them.
type waUser struct {
	u     store.User
	creds []webauthn.Credential
}

func (w *waUser) WebAuthnID() []byte                         { return w.u.Handle }
func (w *waUser) WebAuthnName() string                       { return w.u.Name }
func (w *waUser) WebAuthnDisplayName() string                { return w.u.Name }
func (w *waUser) WebAuthnCredentials() []webauthn.Credential { return w.creds }

func (s *Server) loadUser(ctx context.Context, u store.User) (*waUser, error) {
	raw, err := s.store.Credentials(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	wu := &waUser{u: u}
	for _, b := range raw {
		var c webauthn.Credential
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		wu.creds = append(wu.creds, c)
	}
	return wu, nil
}

// ceremony is a passkey prompt in progress, between begin and finish.
type ceremony struct {
	session webauthn.SessionData
	invite  string // registration: the invite being used
	name    string // registration of a new person
	handle  []byte
	forUser *int64 // registration for a re-invite
	expires time.Time
}

// ceremonies are kept in memory: they last a few minutes, and a restart only
// means trying the prompt again.
type ceremonies struct {
	mu sync.Mutex
	m  map[string]ceremony
}

func (c *ceremonies) put(cer ceremony) string {
	id, _ := store.NewToken()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]ceremony{}
	}
	now := cer.expires.Add(-ceremonyTTL)
	for k, v := range c.m {
		if !now.Before(v.expires) {
			delete(c.m, k) // expired
		}
	}
	c.m[id] = cer
	return id
}

func (c *ceremonies) take(id string, now time.Time) (ceremony, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cer, ok := c.m[id]
	delete(c.m, id)
	return cer, ok && now.Before(cer.expires)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad json")
		return false
	}
	return true
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.InviteByToken(r.Context(), chi.URLParam(r, "token"), s.now())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "this invite link has expired or was already used")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	info := InviteInfo{IsAdmin: inv.IsAdmin, ExpiresAt: inv.ExpiresAt}
	if inv.ForUser != nil {
		u, err := s.store.UserByID(r.Context(), *inv.ForUser)
		if err != nil {
			s.internalError(w, err)
			return
		}
		info.ForUser = u.Name
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, info)
}

// cleanName trims a display name and checks its length.
func cleanName(name string) (string, bool) {
	name = strings.Join(strings.Fields(name), " ")
	n := utf8.RuneCountInString(name)
	return name, n >= 1 && n <= maxNameLen
}

func (s *Server) registerBegin(w http.ResponseWriter, r *http.Request) {
	var req RegisterBegin
	if !decode(w, r, &req) {
		return
	}
	ctx, now := r.Context(), s.now()
	inv, err := s.store.InviteByToken(ctx, req.Invite, now)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusGone, "this invite link has expired or was already used")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}

	cer := ceremony{invite: req.Invite, expires: now.Add(ceremonyTTL)}
	var user *waUser
	if inv.ForUser != nil {
		u, err := s.store.UserByID(ctx, *inv.ForUser)
		if err == nil {
			user, err = s.loadUser(ctx, u)
		}
		if err != nil {
			s.internalError(w, err)
			return
		}
		cer.forUser = inv.ForUser
	} else {
		name, ok := cleanName(req.Name)
		if !ok {
			writeError(w, http.StatusBadRequest, "pick a name up to 40 characters")
			return
		}
		if _, err := s.store.UserByName(ctx, name); err == nil {
			writeError(w, http.StatusConflict, "someone already has that name")
			return
		} else if !errors.Is(err, store.ErrNotFound) {
			s.internalError(w, err)
			return
		}
		cer.name, cer.handle = name, store.NewHandle()
		user = &waUser{u: store.User{Handle: cer.handle, Name: name}}
	}

	creation, session, err := s.wa.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(user.creds).CredentialDescriptors()),
	)
	if err != nil {
		s.internalError(w, err)
		return
	}
	cer.session = *session
	writeJSON(w, http.StatusOK, Ceremony{Ceremony: s.ceremonies.put(cer), Options: creation.Response})
}

func (s *Server) registerFinish(w http.ResponseWriter, r *http.Request) {
	ctx, now := r.Context(), s.now()
	cer, ok := s.ceremonies.take(r.URL.Query().Get("ceremony"), now)
	if !ok {
		writeError(w, http.StatusBadRequest, "that took too long, try again")
		return
	}
	user := &waUser{u: store.User{Handle: cer.handle, Name: cer.name}}
	if cer.forUser != nil {
		u, err := s.store.UserByID(ctx, *cer.forUser)
		if err == nil {
			user, err = s.loadUser(ctx, u)
		}
		if err != nil {
			s.internalError(w, err)
			return
		}
	}
	cred, err := s.wa.FinishRegistration(user, cer.session, r)
	if err != nil {
		slog.Info("passkey registration failed", "error", err)
		writeError(w, http.StatusBadRequest, "the passkey couldn't be saved, try again")
		return
	}
	data, err := json.Marshal(cred)
	if err != nil {
		s.internalError(w, err)
		return
	}
	u, err := s.store.Redeem(ctx, cer.invite, cer.handle, cer.name, cred.ID, data, now)
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusGone, "this invite link has expired or was already used")
	case errors.Is(err, store.ErrNameTaken):
		writeError(w, http.StatusConflict, "someone already has that name")
	case err != nil:
		s.internalError(w, err)
	default:
		slog.Info("passkey added", "user", u.ID, "name", u.Name, "reinvite", cer.forUser != nil)
		s.signIn(w, r, u)
	}
}

func (s *Server) loginBegin(w http.ResponseWriter, r *http.Request) {
	assertion, session, err := s.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		s.internalError(w, err)
		return
	}
	id := s.ceremonies.put(ceremony{session: *session, expires: s.now().Add(ceremonyTTL)})
	writeJSON(w, http.StatusOK, Ceremony{Ceremony: id, Options: assertion.Response})
}

func (s *Server) loginFinish(w http.ResponseWriter, r *http.Request) {
	ctx, now := r.Context(), s.now()
	cer, ok := s.ceremonies.take(r.URL.Query().Get("ceremony"), now)
	if !ok {
		writeError(w, http.StatusBadRequest, "that took too long, try again")
		return
	}
	found, cred, err := s.wa.FinishPasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		u, err := s.store.UserByHandle(ctx, handle)
		if err != nil {
			return nil, err
		}
		return s.loadUser(ctx, u)
	}, cer.session, r)
	if err != nil {
		slog.Info("passkey sign-in failed", "error", err)
		writeError(w, http.StatusUnauthorized, "that passkey isn't one we know")
		return
	}
	data, err := json.Marshal(cred)
	if err == nil {
		err = s.store.UpdateCredential(ctx, cred.ID, data, now)
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.signIn(w, r, found.(*waUser).u)
}

// --- admin

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	var req CreateInvite
	if !decode(w, r, &req) {
		return
	}
	if req.ForUserID != nil {
		if _, err := s.store.UserByID(r.Context(), *req.ForUserID); err != nil {
			writeError(w, http.StatusNotFound, "no such person")
			return
		}
	}
	now := s.now()
	inv := store.Invite{CreatedBy: &userFrom(r.Context()).ID, ForUser: req.ForUserID, IsAdmin: req.IsAdmin,
		CreatedAt: now, ExpiresAt: now.Add(InviteTTL)}
	token, err := s.store.CreateInvite(r.Context(), inv)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, InviteLink{URL: InviteURL(s.cfg.BaseURL, token), ExpiresAt: inv.ExpiresAt})
}
