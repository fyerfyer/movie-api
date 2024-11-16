package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	// "time"

	"github.com/gin-gonic/gin"
	"greenlight.fyerfyer.net/internal/data"
	"greenlight.fyerfyer.net/internal/validator"
)

func (app *application) showMovieHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(c)
		return
	}

	movie, err := app.models.MovieModel.Movies.Get(int64(id))
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{"movie": movie})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) createMovieHandler(c *gin.Context) {
	var input struct {
		Title   string       `json:"title"`
		Year    int          `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	err = app.models.MovieModel.Movies.Insert(movie)
	if err != nil {
		log.Println("Insert error:", err)
		app.serverErrorResponse(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))
	err = app.writeJSON(c, http.StatusCreated, envelope{"movie": movie})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) updateMovieHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		app.notFoundResponse(c)
		return
	}

	movie := &data.Movie{}
	movie, err = app.models.MovieModel.Movies.Get(int64(id))
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}
		return
	}

	var input struct {
		Title   *string       `json:"title"`
		Year    *int          `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}

	err = app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	v := validator.New()
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	// movie.Version += 1
	err = app.models.MovieModel.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{"movie": movie})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) deleteMovieHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFoundResponse(c)
		return
	}

	err = app.models.MovieModel.Movies.Delete(int64(id))
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{"message": "movie successfully deleted"})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) listMoviesHandler(c *gin.Context) {
	var input struct {
		Title  string
		Genres []string
		Filter data.Filters
	}

	v := validator.New()
	values := c.Request.URL.Query()

	input.Title = app.readString(values, "title", "")
	input.Genres = app.readCSV(values, "genres", []string{})
	input.Filter.Page = app.readInt(values, "page", 1, v)
	input.Filter.PageSize = app.readInt(values, "page_size", 20, v)
	input.Filter.Sort = app.readString(values, "sort", "id")
	input.Filter.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	if data.ValidateFilters(v, input.Filter); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	movies, metadata, err := app.models.MovieModel.Movies.GetAll(input.Title, input.Genres, input.Filter)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{"movies": movies, "metadata": metadata})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}
