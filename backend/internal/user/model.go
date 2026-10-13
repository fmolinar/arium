package user

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

const (
	maxBodyBytes = 16 << 10

	maxNameLen  = 100
	maxEmailLen = 254
	// bcrypt only uses the first 72 bytes and Go's rejects longer passwords.
	minPasswordLen = 8
	maxPasswordLen = 72
)

type User struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string        `bson:"name"          json:"name"`
	Email        string        `bson:"email"         json:"email"`
	PasswordHash string        `bson:"password_hash" json:"-"`
	Role         string        `bson:"role"          json:"role"`
	CreatedAt    time.Time     `bson:"created_at"     json:"createdAt"`
	UpdatedAt    time.Time     `bson:"updated_at"     json:"updatedAt"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateProfileRequest struct {
	Name string `json:"name"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// normalize trims the fields and lowercases the email, so the unique index
// treats Foo@Example.com and foo@example.com as one address.
func (r *RegisterRequest) normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = normalizeEmail(r.Email)
}

// validate returns a message for the client, or "" when the request is valid.
func (r *RegisterRequest) validate() string {
	if r.Name == "" || r.Email == "" || r.Password == "" {
		return "name, email and password are required"
	}
	if msg := validateName(r.Name); msg != "" {
		return msg
	}
	if len(r.Email) > maxEmailLen {
		return "email is too long"
	}
	if addr, err := mail.ParseAddress(r.Email); err != nil || addr.Address != r.Email {
		return "email is invalid"
	}
	if len(r.Password) < minPasswordLen || len(r.Password) > maxPasswordLen {
		return "password must be 8 to 72 characters"
	}

	return ""
}

func (r *LoginRequest) normalize() {
	r.Email = normalizeEmail(r.Email)
}

func (r *UpdateProfileRequest) normalize() {
	r.Name = strings.TrimSpace(r.Name)
}

func (r *UpdateProfileRequest) validate() string {
	if r.Name == "" {
		return "name is required"
	}

	return validateName(r.Name)
}

func validateName(name string) string {
	if utf8.RuneCountInString(name) > maxNameLen {
		return "name must be at most 100 characters"
	}

	return ""
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
