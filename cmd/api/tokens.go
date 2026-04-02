package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/htet-29/greenlight/internal/data"
	"github.com/htet-29/greenlight/internal/domain"
	"github.com/htet-29/greenlight/internal/validator"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *application) createAuthenticationTokenHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	v := validator.New()

	domain.ValidateEmail(v, input.Email)
	domain.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	user, err := app.db.GetUserByEmail(ctx, input.Email)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.invalidCredentialsResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	domainUser := toDomainUserWithPassword(user)

	match, err := domainUser.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(w, r)
		return
	}

	token := domain.GenerateToken(domainUser.ID, 24*time.Hour, domain.ScopeAuthentication)

	tokenCtx, tokenCancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer tokenCancel()

	err = app.db.CreateToken(tokenCtx, data.CreateTokenParams{
		Hash:   token.Hash,
		UserID: domainUser.ID,
		Expiry: pgtype.Timestamptz{Time: token.Expiry, Valid: true},
		Scope:  token.Scope,
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"authentication_token": token}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
