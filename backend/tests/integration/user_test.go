//go:build integration

package integration

import (
	"net/http"
	"testing"
)

type userDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type authResponse struct {
	Token string  `json:"token"`
	User  userDTO `json:"user"`
}

func TestHealth(t *testing.T) {
	resp := request(t, http.MethodGet, "/health", "", nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRegisterLoginProfileFlow(t *testing.T) {
	email := uniqueEmail(t)
	password := "hunter22"

	registerResp := request(t, http.MethodPost, "/api/v1/users/register", "", map[string]string{
		"name":     "Fer",
		"email":    email,
		"password": password,
	})
	if registerResp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", registerResp.StatusCode)
	}

	registered := decodeJSON[authResponse](t, registerResp)
	if registered.Token == "" {
		t.Fatal("register: expected a token")
	}
	if registered.User.Role != "user" {
		t.Fatalf("register: expected role %q, got %q", "user", registered.User.Role)
	}

	dupeResp := request(t, http.MethodPost, "/api/v1/users/register", "", map[string]string{
		"name":     "Fer",
		"email":    email,
		"password": password,
	})
	dupeResp.Body.Close()
	if dupeResp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate register: expected 409, got %d", dupeResp.StatusCode)
	}

	loginResp := request(t, http.MethodPost, "/api/v1/users/login", "", map[string]string{
		"email":    email,
		"password": password,
	})
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", loginResp.StatusCode)
	}

	loggedIn := decodeJSON[authResponse](t, loginResp)
	if loggedIn.Token == "" {
		t.Fatal("login: expected a token")
	}

	badLoginResp := request(t, http.MethodPost, "/api/v1/users/login", "", map[string]string{
		"email":    email,
		"password": "wrong-password",
	})
	badLoginResp.Body.Close()
	if badLoginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login: expected 401, got %d", badLoginResp.StatusCode)
	}

	noAuthResp := request(t, http.MethodGet, "/api/v1/users/me", "", nil)
	noAuthResp.Body.Close()
	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/me without token: expected 401, got %d", noAuthResp.StatusCode)
	}

	meResp := request(t, http.MethodGet, "/api/v1/users/me", loggedIn.Token, nil)
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("/me: expected 200, got %d", meResp.StatusCode)
	}

	profile := decodeJSON[userDTO](t, meResp)
	if profile.Email != email {
		t.Fatalf("/me: expected email %q, got %q", email, profile.Email)
	}

	updateResp := request(t, http.MethodPatch, "/api/v1/users/me", loggedIn.Token, map[string]string{
		"name": "Fernando",
	})
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /me: expected 200, got %d", updateResp.StatusCode)
	}

	updated := decodeJSON[userDTO](t, updateResp)
	if updated.Name != "Fernando" {
		t.Fatalf("PATCH /me: expected name %q, got %q", "Fernando", updated.Name)
	}
}
