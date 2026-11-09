package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/evanofslack/prulights/store"
)

const origin = "http://localhost:8080"

// call makes a request as a browser on our origin would, with a session
// cookie when token is set.
func call(t *testing.T, s *Server, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Origin", origin)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// person adds a user straight to the store and signs them in.
func person(t *testing.T, s *Server, name string, admin bool) (store.User, string) {
	t.Helper()
	ctx, now := context.Background(), s.now()
	inv, err := s.store.CreateInvite(ctx, store.Invite{IsAdmin: admin, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	u, err := s.store.Redeem(ctx, inv, store.NewHandle(), name, []byte(name+"-cred"), []byte(`{}`), now)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.store.CreateSession(ctx, u.ID, now, sessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	return u, token
}

func TestMe(t *testing.T) {
	s := newTestServer(t)
	if rec := call(t, s, "GET", "/api/v1/me", "", ""); rec.Code != http.StatusNoContent {
		t.Errorf("signed out /me = %d", rec.Code)
	}
	mom, token := person(t, s, "Mom", false)
	rec := call(t, s, "GET", "/api/v1/me", "", token)
	var me MeResponse
	json.Unmarshal(rec.Body.Bytes(), &me)
	if rec.Code != 200 || me.ID != mom.ID || me.Name != "Mom" || me.IsAdmin {
		t.Errorf("/me = %d %+v", rec.Code, me)
	}
	if rec := call(t, s, "GET", "/api/v1/me", "", "stale-token"); rec.Code != http.StatusNoContent ||
		!strings.Contains(rec.Header().Get("Set-Cookie"), "Max-Age=0") {
		t.Errorf("a bad cookie should be cleared: %d %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}

	// a day later the session slides forward
	s.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if rec := call(t, s, "GET", "/api/v1/me", "", token); !strings.Contains(rec.Header().Get("Set-Cookie"), sessionCookie+"="+token) {
		t.Errorf("session not refreshed: %q", rec.Header().Get("Set-Cookie"))
	}

	if rec := call(t, s, "POST", "/api/v1/auth/logout", "", token); rec.Code != http.StatusNoContent {
		t.Errorf("logout = %d", rec.Code)
	}
	if rec := call(t, s, "GET", "/api/v1/me", "", token); rec.Code != http.StatusNoContent {
		t.Errorf("after logout /me = %d", rec.Code)
	}
}

func TestSameOrigin(t *testing.T) {
	s := newTestServer(t)
	_, token := person(t, s, "Evan", true)
	req := httptest.NewRequest("POST", "/api/v1/admin/invites", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("other origin = %d", rec.Code)
	}
	req = httptest.NewRequest("POST", "/api/v1/admin/invites", strings.NewReader("is_admin=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("form post = %d", rec.Code)
	}
}

func TestAdminInvites(t *testing.T) {
	s := newTestServer(t)
	_, admin := person(t, s, "Evan", true)
	mom, momToken := person(t, s, "Mom", false)

	if rec := call(t, s, "POST", "/api/v1/admin/invites", `{}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/invites", `{}`, momToken); rec.Code != http.StatusForbidden {
		t.Errorf("not an admin = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/admin/invites", `{"for_user_id":999}`, admin); rec.Code != http.StatusNotFound {
		t.Errorf("re-invite for nobody = %d", rec.Code)
	}

	rec := call(t, s, "POST", "/api/v1/admin/invites", `{"for_user_id":`+itoa(mom.ID)+`}`, admin)
	var link InviteLink
	json.Unmarshal(rec.Body.Bytes(), &link)
	token, ok := strings.CutPrefix(link.URL, "http://localhost:8080/invite?t=")
	if rec.Code != http.StatusCreated || !ok {
		t.Fatalf("invite = %d %+v", rec.Code, link)
	}

	rec = call(t, s, "GET", "/api/v1/auth/invites/"+token, "", "")
	var info InviteInfo
	json.Unmarshal(rec.Body.Bytes(), &info)
	if rec.Code != 200 || info.ForUser != "Mom" || info.IsAdmin {
		t.Errorf("invite info = %d %+v", rec.Code, info)
	}
	if rec := call(t, s, "GET", "/api/v1/auth/invites/nope", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown invite = %d", rec.Code)
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func TestRegisterBegin(t *testing.T) {
	s := newTestServer(t)
	evan, _ := person(t, s, "Evan", true)
	ctx, now := context.Background(), s.now()
	inv, _ := s.store.CreateInvite(ctx, store.Invite{CreatedBy: &evan.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)})

	for body, want := range map[string]int{
		`{"invite":"` + inv + `","name":"   "}`:                             http.StatusBadRequest,
		`{"invite":"` + inv + `","name":"` + strings.Repeat("x", 41) + `"}`: http.StatusBadRequest,
		`{"invite":"` + inv + `","name":"EVAN"}`:                            http.StatusConflict,
		`{"invite":"expired","name":"Mom"}`:                                 http.StatusGone,
		`not json`:                                                          http.StatusBadRequest,
	} {
		if rec := call(t, s, "POST", "/api/v1/auth/register/begin", body, ""); rec.Code != want {
			t.Errorf("%s = %d, want %d: %s", body, rec.Code, want, rec.Body.String())
		}
	}

	rec := call(t, s, "POST", "/api/v1/auth/register/begin", `{"invite":"`+inv+`","name":"  Mom  "}`, "")
	var cer struct {
		Ceremony string `json:"ceremony"`
		Options  struct {
			RP   struct{ ID string } `json:"rp"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
			Challenge              string `json:"challenge"`
			AuthenticatorSelection struct {
				ResidentKey string `json:"residentKey"`
			} `json:"authenticatorSelection"`
		} `json:"options"`
	}
	json.Unmarshal(rec.Body.Bytes(), &cer)
	if rec.Code != 200 || cer.Ceremony == "" || cer.Options.RP.ID != "localhost" || cer.Options.User.Name != "Mom" ||
		cer.Options.Challenge == "" || cer.Options.AuthenticatorSelection.ResidentKey != "required" {
		t.Errorf("begin = %d %s", rec.Code, rec.Body.String())
	}

	// finishing with a junk response fails, and the ceremony is used up
	if rec := call(t, s, "POST", "/api/v1/auth/register/finish?ceremony="+cer.Ceremony, `{}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("junk finish = %d", rec.Code)
	}
	if rec := call(t, s, "POST", "/api/v1/auth/register/finish?ceremony="+cer.Ceremony, `{}`, ""); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "too long") {
		t.Errorf("reused ceremony = %d %s", rec.Code, rec.Body.String())
	}
	// the invite still works after a failed attempt
	if _, err := s.store.InviteByToken(ctx, inv, now); err != nil {
		t.Errorf("invite used by a failed attempt: %v", err)
	}
}

func TestLoginBegin(t *testing.T) {
	s := newTestServer(t)
	rec := call(t, s, "POST", "/api/v1/auth/login/begin", "", "")
	var cer Ceremony
	json.Unmarshal(rec.Body.Bytes(), &cer)
	if rec.Code != 200 || cer.Ceremony == "" {
		t.Errorf("login begin = %d %s", rec.Code, rec.Body.String())
	}
	// a ceremony expires
	s.now = func() time.Time { return time.Now().Add(ceremonyTTL + time.Second) }
	if rec := call(t, s, "POST", "/api/v1/auth/login/finish?ceremony="+cer.Ceremony, `{}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("expired ceremony = %d", rec.Code)
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{"  Mom ": "Mom", "Aunt   Sue": "Aunt Sue", "Zoë": "Zoë"} {
		if got, ok := cleanName(in); !ok || got != want {
			t.Errorf("cleanName(%q) = %q %v", in, got, ok)
		}
	}
	if _, ok := cleanName(strings.Repeat("é", 41)); ok {
		t.Error("41 characters is too long")
	}
}
