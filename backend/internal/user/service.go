package user

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"github.com/fmolinar/arium/backend/internal/config"
	"github.com/fmolinar/arium/backend/internal/middleware"
)

var ErrInvalidCredentials = errors.New("invalid email or password")

type Service struct {
	repo *Repository
	cfg  config.Config
}

func NewService(repo *Repository, cfg config.Config) *Service {
	return &Service{repo: repo, cfg: cfg}
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		Role:         RoleUser,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	return s.authResponse(u)
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}

		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.authResponse(u)
}

func (s *Service) Profile(ctx context.Context, id string) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrNotFound
	}

	return s.repo.FindByID(ctx, oid)
}

func (s *Service) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, ErrNotFound
	}

	if err := s.repo.UpdateName(ctx, oid, req.Name); err != nil {
		return nil, err
	}

	return s.repo.FindByID(ctx, oid)
}

func (s *Service) authResponse(u *User) (*AuthResponse, error) {
	token, err := middleware.NewToken(s.cfg, u.ID.Hex(), u.Role)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{Token: token, User: *u}, nil
}
