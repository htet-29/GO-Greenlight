package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/htet-29/greenlight/internal/data"
	"github.com/htet-29/greenlight/internal/domain"
	"github.com/htet-29/greenlight/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	pendingUser := &domain.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = pendingUser.Password.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	if domain.ValidateUser(v, pendingUser); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	user, err := app.db.CreateUser(ctx, data.CreateUserParams{
		Name:         pendingUser.Name,
		Email:        pendingUser.Email,
		PasswordHash: pendingUser.Password.Hash,
		Activated:    pendingUser.Activated,
	})
	if err != nil {
		switch {
		case strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`):
			v.AddError("email", "a use with this email address already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": toDomainUser(user)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
