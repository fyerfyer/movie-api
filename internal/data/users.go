package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/plugin/optimisticlock"
	"greenlight.fyerfyer.net/internal/validator"
)

type User struct {
	ID             int64                  `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time              `gorm:"not null;default:now()" json:"created_at"`
	Name           string                 `gorm:"not null" json:"name"`
	Email          string                 `gorm:"type:citext;not null;unique" json:"email"`
	HashedPassword string                 `gorm:"type:bytea;not null" json:"-"`
	Activated      bool                   `gorm:"not null" json:"activated"`
	Version        optimisticlock.Version `gorm:"version;not null;default 1" json:"-"`
	Password       password               `gorm:"-" json:"-"`
	Permission     []Permission           `gorm:"many2many:users_permissions;" json:"-"`
}

type password struct {
	plaintext *string
	hash      []byte
}

var (
	ErrDuplicateEmail = errors.New("duplicate email")
	AnonymousUser     = &User{}
)

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	ValidateEmail(v, user.Email)
	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

func (p *password) Matches(Password string) (bool, error) {
	log.Printf("Password hash: %s", p.hash)
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(Password))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func (u *User) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	u.Password.plaintext = &plaintextPassword
	u.Password.hash = hash
	u.HashedPassword = string(hash)

	return nil
}

func (u *User) Insert(user *User) error {
	if err := db.Create(&user).Error; err != nil {
		switch {
		case err.Error() == `ERROR: duplicate key value violates unique constraint "uni_users_email" (SQLSTATE 23505)`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (u *User) GetByEmail(email string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var user User
	if err := db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error; err != nil {
		return nil, err
	}
	user.Password.hash = []byte(user.HashedPassword)
	return &user, nil
}

// remember to update the Password field manually!!!
func (u *User) Update(user *User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := db.WithContext(ctx).
		Model(&User{}).
		Where("email = ?", user.Email).
		Updates(map[string]interface{}{
			"name":            user.Name,
			"email":           user.Email,
			"hashed_password": user.HashedPassword,
			"activated":       user.Activated,
			"id":              user.ID,
		}).Error

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

func (u *User) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tokenHash := sha256.Sum256([]byte(tokenPlaintext))
	var user User
	err := db.WithContext(ctx).
		Joins("INNER JOIN tokens ON users.id = tokens.user_id").
		Where("tokens.hash = ?", tokenHash[:]).
		Where("tokens.scope = ?", tokenScope).
		Where("tokens.expiry > ?", time.Now()).
		First(&user).Error

	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}
