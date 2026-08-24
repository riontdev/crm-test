package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("credenciales inválidas")

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

type UserService struct {
	pool *pgxpool.Pool
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{pool: pool}
}

func (s *UserService) Authenticate(ctx context.Context, email, password string) (*User, error) {
	var (
		u   User
		hash string
	)
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, role, password_hash FROM users WHERE email = $1`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &hash)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &u, nil
}

func (s *UserService) GetByID(ctx context.Context, id string) (*User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("id inválido")
	}
	var u User
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, role FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, fmt.Errorf("usuario no encontrado: %w", err)
	}
	return &u, nil
}

// UpdateProfile actualiza nombre y/o contraseña del propio usuario autenticado.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, name, currentPassword, newPassword *string) (*User, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, errors.New("id inválido")
	}

	var (
		u         User
		nameDB    string
		passHash  string
	)
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, name, role, password_hash FROM users WHERE id = $1`, userID,
	).Scan(&u.ID, &u.Email, &nameDB, &u.Role, &passHash)
	if err != nil {
		return nil, fmt.Errorf("usuario no encontrado: %w", err)
	}
	if name == nil && currentPassword == nil && newPassword == nil {
		return &u, nil
	}

	newName := nameDB
	if name != nil {
		newName = strings.TrimSpace(*name)
		if utf8.RuneCountInString(newName) < 2 || utf8.RuneCountInString(newName) > 80 {
			return nil, errors.New("el nombre debe tener entre 2 y 80 caracteres")
		}
	}

	newHash := passHash
	if newPassword != nil {
		if currentPassword == nil {
			return nil, errors.New("completá la contraseña actual y la nueva")
		}
		if err := bcrypt.CompareHashAndPassword([]byte(passHash), []byte(*currentPassword)); err != nil {
			return nil, errors.New("la contraseña actual es incorrecta")
		}
		if len(*newPassword) < 6 {
			return nil, errors.New("la nueva contraseña debe tener al menos 6 caracteres")
		}
		h, err := bcrypt.GenerateFromPassword([]byte(*newPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		newHash = string(h)
	} else if currentPassword != nil {
		return nil, errors.New("completá la contraseña actual y la nueva")
	}

	err = s.pool.QueryRow(ctx,
		`UPDATE users SET name = $1, password_hash = $2, updated_at = now()
		 WHERE id = $3 RETURNING id, email, name, role`,
		newName, newHash, userID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}
	return &u, nil
}

func (s *UserService) List(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, name, role, created_at, updated_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var r struct {
			ID        string
			Email     string
			Name      string
			Role      string
			CreatedAt time.Time
			UpdatedAt time.Time
		}
		if err := rows.Scan(&r.ID, &r.Email, &r.Name, &r.Role, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		out = append(out, User{ID: r.ID, Email: r.Email, Name: r.Name, Role: r.Role})
	}
	return out, rows.Err()
}

func validateRole(role string) error {
	if role != "admin" && role != "agente" {
		return errors.New("el rol debe ser 'admin' o 'agente'")
	}
	return nil
}

func (s *UserService) Create(ctx context.Context, email, name, password, role string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)

	if !emailRegex.MatchString(email) {
		return nil, errors.New("email inválido")
	}
	if utf8.RuneCountInString(name) < 2 {
		return nil, errors.New("el nombre es obligatorio")
	}
	if len(password) < 6 {
		return nil, errors.New("la contraseña debe tener al menos 6 caracteres")
	}
	if err := validateRole(role); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var u User
	err = s.pool.QueryRow(ctx,
		`INSERT INTO users (email, name, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, name, role`,
		email, name, string(hash), role,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, errors.New("el email ya está registrado")
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &u, nil
}

func (s *UserService) Update(ctx context.Context, id string, name, role, password *string) (*User, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, errors.New("id inválido")
	}

	sets := []string{}
	args := []interface{}{}
	arg := 1

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if utf8.RuneCountInString(trimmed) < 2 {
			return nil, errors.New("el nombre es obligatorio")
		}
		sets = append(sets, fmt.Sprintf("name = $%d", arg))
		args = append(args, trimmed)
		arg++
	}
	if role != nil {
		if err := validateRole(*role); err != nil {
			return nil, err
		}
		sets = append(sets, fmt.Sprintf("role = $%d", arg))
		args = append(args, *role)
		arg++
	}
	if password != nil {
		if len(*password) > 0 && len(*password) < 6 {
			return nil, errors.New("la contraseña debe tener al menos 6 caracteres")
		}
		if len(*password) > 0 {
			hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
			if err != nil {
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}
			sets = append(sets, fmt.Sprintf("password_hash = $%d", arg))
			args = append(args, string(hash))
			arg++
		}
	}

	if len(sets) == 0 {
		return s.GetByID(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	query := fmt.Sprintf(
		`UPDATE users SET %s WHERE id = %s RETURNING id, email, name, role`,
		strings.Join(sets, ", "), fmt.Sprintf("$%d", arg),
	)
	args = append(args, id)

	var u User
	err := s.pool.QueryRow(ctx, query, args...).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}
	return &u, nil
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return errors.New("id inválido")
	}

	var role string
	err = s.pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, parsedID).Scan(&role)
	if err != nil {
		return fmt.Errorf("usuario no encontrado: %w", err)
	}

	if role == "admin" {
		var adminCount int
		err = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&adminCount)
		if err != nil {
			return fmt.Errorf("failed to count admins: %w", err)
		}
		if adminCount <= 1 {
			return errors.New("no se puede eliminar el último administrador")
		}
	}

	_, err = s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, parsedID)
	return err
}
