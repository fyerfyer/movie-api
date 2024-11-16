package data

import (
	"context"
	"time"
)

type Permission struct {
	ID    int64  `gorm:"primaryKey"`
	Code  string `gorm:"type:text;not null"`
	Users []User `gorm:"many2many:users_permissions;"`
}

type Permissions []string

func initPermissionTable() error {
	permissions := []*Permission{
		{Code: "movies:read"},
		{Code: "movies:write"},
	}
	return db.Create(permissions).Error
}

func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}
	return false
}

func (p *Permission) GetAllForUser(id int64) (Permissions, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var codes []string
	err := db.WithContext(ctx).
		Model(&Permission{}).
		Joins(`INNER JOIN users_permissions ON users_permissions.permission_id = permissions.id`).
		Joins(`INNER JOIN users ON users_permissions.user_id = users.id`).
		Where(`users.id = ?`, id).
		Select("code").
		Find(&codes).Error

	if err != nil {
		return nil, err
	}

	return Permissions(codes), nil
}

func (p *Permission) AddForUser(userID int64, codes ...string) error {
	type UserPermission struct {
		UserID       int64 `gorm:"column:user_id"`
		PermissionID int64 `gorm:"column:permission_id"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var permissionID int64
	err := db.WithContext(ctx).
		Model(&Permission{}).
		Select("id").
		Where("code in ?", codes).
		First(&permissionID).Error
	if err != nil {
		return err
	}

	userPermission := UserPermission{
		UserID:       userID,
		PermissionID: permissionID,
	}

	err = db.Table("users_permissions").
		Create(&userPermission).Error
	return err
}
