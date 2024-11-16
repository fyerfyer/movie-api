package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"greenlight.fyerfyer.net/internal/validator"
)

const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
	ScopePasswordRest   = "password-reset"
)

type Token struct {
	Plaintext string    `gorm:"-" json:"token"`
	Hash      []byte    `gorm:"primaryKey;type:bytea" json:"-"`
	UserID    int64     `gorm:"not null;constraint:OnDelete:CASCADE;foreignKey:ID;" json:"-"`
	Expiry    time.Time `gorm:"not null" json:"expiry"`
	Scope     string    `gorm:"not null" json:"-"`
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	// use a byte slice to store 16 bytes random numbers
	randomBytes := make([]byte, 16)

	// use rand.Read to fill the slice with random numbers
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil
}

func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

func (t *Token) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = t.Insert(token)
	return token, err
}

func (t *Token) Insert(token *Token) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).
		Create(&token).Error; err != nil {
		return err
	}

	return nil
}

func (t *Token) DeleteAllForUser(scope string, userID int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).
		Where("scope = ? AND user_id = ?", scope, userID).
		Delete(&Token{}).Error; err != nil {
		return err
	}

	return nil
}
