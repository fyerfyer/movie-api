package data

import (
	"database/sql"
	"errors"
	"expvar"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"greenlight.fyerfyer.net/internal/config"
)

var (
	db                *gorm.DB
	sqlDB             *sql.DB
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflcit")
)

type MovieModels struct {
	Movies *Movie
}

type UserModels struct {
	Users *User
}

type TokenModels struct {
	Tokens *Token
}

type PermissionModels struct {
	Permissions *Permission
}

type Models struct {
	MovieModel      MovieModels
	UserModel       UserModels
	TokenModel      TokenModels
	PermissionModel PermissionModels
}

func NewModels() Models {
	return Models{
		MovieModel:      MovieModels{Movies: &Movie{}},
		UserModel:       UserModels{Users: &User{}},
		TokenModel:      TokenModels{Tokens: &Token{}},
		PermissionModel: PermissionModels{Permissions: &Permission{}},
	}
}

func migrateModels(db *gorm.DB) error {
	err := db.AutoMigrate(&Movie{}, &User{}, &Token{}, &Permission{})
	if err != nil {
		return err
	}

	if err := initPermissionTable(); err != nil {
		return err
	}
	// add gin index
	db.Exec("CREATE INDEX IF NOT EXISTS movies_title_idx ON movies USING GIN (to_tsvector('simple', title))")
	db.Exec("CREATE INDEX IF NOT EXISTS movies_genres_idx ON movies USING GIN (genres)")
	return nil
}

func InitSql() error {
	var err error
	db, err = gorm.Open(postgres.Open(config.Cfg.DB.Dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	sqlDB, err = db.DB()
	if err != nil {
		return err
	}
	err = migrateModels(db)
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	log.Println("Migration completed successfully!")

	sqlDB.SetMaxOpenConns(config.Cfg.DB.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.Cfg.DB.MaxIdleConns)
	duration, err := time.ParseDuration(config.Cfg.DB.MaxIdleTime)
	if err != nil {
		log.Fatalf("Failed to config database: %v", err)
	}
	sqlDB.SetConnMaxIdleTime(duration)

	// init expvar
	expvar.Publish("database", expvar.Func(func() any {
		return sqlDB.Stats()
	}))
	return nil
}
