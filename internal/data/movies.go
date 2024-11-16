package data

import (
	// "database/sql"
	// "errors"
	// "sync"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/plugin/optimisticlock"
	"greenlight.fyerfyer.net/internal/validator"
)

type Movie struct {
	ID        int64                  `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time              `gorm:"not null;default:now()" json:"-"`
	Title     string                 `gorm:"type:text;not null" json:"title"`
	Year      int                    `gorm:"not null" json:"year,omitempty"`
	Runtime   Runtime                `gorm:"not null" json:"runtime,omitempty"`
	Genres    pq.StringArray         `gorm:"type:text[];not null" json:"genres,omitempty"`
	Version   optimisticlock.Version `gorm:"version" json:"version"`
	// mux       sync.Mutex
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

func (m *Movie) BeforeCreate(tx *gorm.DB) (err error) {
	if len(m.Genres) > 0 {
		m.Genres = pq.StringArray(m.Genres)
	}
	return nil
}

func (m *Movie) Insert(movie *Movie) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := db.WithContext(ctx).Create(&movie).Error; err != nil {
		return err
	}

	return nil
}

func (m *Movie) Get(id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var movie Movie

	if err := db.WithContext(ctx).Where("id = ?", id).First(&movie).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &movie, nil
}

func (m *Movie) Update(movie *Movie) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := db.WithContext(ctx).
		Model(&Movie{}).
		Where("id = ?", movie.ID).
		Updates(map[string]interface{}{
			"title":   movie.Title,
			"year":    movie.Year,
			"runtime": movie.Runtime,
			"genres":  movie.Genres,
		}).Error

	return err
}

func (m *Movie) Delete(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&Movie{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m *Movie) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var movies []*Movie
	var totalRecord int64

	countQuery := db.WithContext(ctx).
		Model(&Movie{}).
		Select("count(*)").
		Where("to_tsvector('simple', title) @@ plainto_tsquery('simple', ?) OR ? = ''", title, title).
		Where("genres @> ? OR ? = '{}'", pq.Array(genres), pq.Array(genres))

	if err := countQuery.Count(&totalRecord).Error; err != nil {
		return nil, Metadata{}, err
	}

	subQuery := db.WithContext(ctx).
		Table("movies").
		Where("to_tsvector('simple', title) @@ plainto_tsquery('simple', ?) OR ? = ''", title, title).
		Where("genres @> ? OR ? = '{}'", pq.Array(genres), pq.Array(genres)).
		Order(fmt.Sprintf("%s %s, id ASC", filters.sortColumn(), filters.sortDirection())).
		Limit(filters.limit()).
		Offset(filters.offset())

	if err := subQuery.Find(&movies).Error; err != nil {
		return nil, Metadata{}, err
	}

	if len(movies) == 0 {
		return movies, Metadata{}, nil
	}

	metadata := calculateMetadata(int(totalRecord), filters.Page, filters.PageSize)
	return movies, metadata, nil
}
