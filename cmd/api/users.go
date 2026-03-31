package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
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

	permissionCtx, permissionCancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer permissionCancel()

	err = app.db.AddPermissionsForUser(permissionCtx, data.AddPermissionsForUserParams{
		UserID:          user.ID,
		PermissionCodes: []string{"movies:read"},
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
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

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	if domain.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	tokenHash := sha256.Sum256([]byte(input.TokenPlaintext))
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	returnedValue, err := app.db.GetUserByToken(ctx, data.GetUserByTokenParams{
		Hash:   tokenHash[:],
		Scope:  domain.ScopeActivation,
		Expiry: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}
	user := returnedValue.User
	user.Activated = true

	updateCtx, updateCancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer updateCancel()
	_, err = app.db.UpdateUser(updateCtx, data.UpdateUserParams{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Activated:    user.Activated,
		Version:      user.Version,
	})
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.editConflictResponse(w, r)
		case errors.Is(err, context.DeadlineExceeded):
			app.serverErrorResponse(w, r, errors.New("database operation time out"))
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	deleteCtx, deleteCancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer deleteCancel()

	err = app.db.DeleteAllForUser(deleteCtx, data.DeleteAllForUserParams{
		Scope:  domain.ScopeActivation,
		UserID: user.ID,
	})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"user": toDomainUser(user)}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
