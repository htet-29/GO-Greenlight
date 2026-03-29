package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/htet-29/greenlight/internal/data"
	"github.com/htet-29/greenlight/internal/domain"
	"github.com/htet-29/greenlight/internal/validator"
	"github.com/jackc/pgx/v5/pgtype"
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

	token := domain.GenerateToken(user.ID, 3*24*time.Hour, domain.ScopeActivation)
	tokenCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	err = app.db.CreateToken(tokenCtx, data.CreateTokenParams{
		Hash:   token.Hash,
		UserID: token.UserID,
		Expiry: pgtype.Timestamptz{Time: token.Expiry, Valid: true},
		Scope:  token.Scope,
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		err := app.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			app.logger.Error(err.Error())
		}
	})

	err = app.writeJSON(w, http.StatusCreated, envelope{"user": toDomainUser(user)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
