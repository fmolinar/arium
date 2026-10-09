package user

import (
	"strings"
	"testing"
)

func TestRegisterRequestValidate(t *testing.T) {
	valid := RegisterRequest{Name: "Fer", Email: "fer@example.com", Password: "hunter22"}

	tests := []struct {
		name   string
		modify func(*RegisterRequest)
		ok     bool
	}{
		{"valid", func(*RegisterRequest) {}, true},
		{"missing name", func(r *RegisterRequest) { r.Name = "  " }, false},
		{"long name", func(r *RegisterRequest) { r.Name = strings.Repeat("a", 101) }, false},
		{"not an email", func(r *RegisterRequest) { r.Email = "fer" }, false},
		{"display name", func(r *RegisterRequest) { r.Email = "Fer <fer@example.com>" }, false},
		{"long email", func(r *RegisterRequest) { r.Email = strings.Repeat("a", 250) + "@example.com" }, false},
		{"short password", func(r *RegisterRequest) { r.Password = "hunter2" }, false},
		{"long password", func(r *RegisterRequest) { r.Password = strings.Repeat("a", 73) }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.modify(&req)
			req.normalize()

			if msg := req.validate(); (msg == "") != tt.ok {
				t.Fatalf("validate() = %q, want ok=%v", msg, tt.ok)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	req := RegisterRequest{Name: " Fer ", Email: " Fer@Example.COM "}
	req.normalize()

	if req.Email != "fer@example.com" || req.Name != "Fer" {
		t.Fatalf("got name %q, email %q", req.Name, req.Email)
	}

	login := LoginRequest{Email: "FER@example.com"}
	login.normalize()
	if login.Email != req.Email {
		t.Fatalf("login email %q doesn't match registered %q", login.Email, req.Email)
	}
}

func TestUpdateProfileRequestValidate(t *testing.T) {
	for name, ok := range map[string]bool{"Fernando": true, "   ": false, strings.Repeat("é", 101): false} {
		req := UpdateProfileRequest{Name: name}
		req.normalize()
		if msg := req.validate(); (msg == "") != ok {
			t.Errorf("name %q: validate() = %q, want ok=%v", name, msg, ok)
		}
	}
}
