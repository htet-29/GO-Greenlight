package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/htet-29/greenlight/internal/custom"
	"github.com/htet-29/greenlight/internal/data"
	"github.com/htet-29/greenlight/internal/domain"
	"github.com/htet-29/greenlight/internal/validator"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title   string         `json:"title"`
		Year    int32          `json:"year"`
		Runtime custom.Runtime `json:"runtime"`
		Genres  []string       `json:"genres"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	movie := &domain.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}

	v := validator.New()

	if domain.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	dbMovie, err := app.db.CreateMovie(context.Background(), data.CreateMovieParams{
		Title:   movie.Title,
		Year:    movie.Year,
		Runtime: int32(movie.Runtime),
		Genres:  movie.Genres,
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", dbMovie.ID))

	movie.ID = dbMovie.ID
	movie.Version = dbMovie.Version

	err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	movie, err := app.db.GetMovie(context.Background(), id)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"movie": toDomainMovie(movie)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func toDomainMovie(dbMovie data.Movie) domain.Movie {
	return domain.Movie{
		ID:        dbMovie.ID,
		Title:     dbMovie.Title,
		Year:      dbMovie.Year,
		Runtime:   custom.Runtime(dbMovie.Runtime),
		Genres:    dbMovie.Genres,
		Version:   dbMovie.Version,
		CreatedAt: dbMovie.CreatedAt.Time,
	}
}
