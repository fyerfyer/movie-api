package main

import (
	"errors"
	// "log"
	"time"

	// "log"
	// "fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"greenlight.fyerfyer.net/internal/data"
	"greenlight.fyerfyer.net/internal/validator"
)

func (app *application) registerUserHandler(c *gin.Context) {
	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	user := &data.User{
		Name:      input.Name,
		Email:     input.Email,
		Activated: false,
	}

	err = user.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	v := validator.New()
	if data.ValidateUser(v, user); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	err = app.models.UserModel.Users.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			app.failedValidationResponse(c, v.Errors)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	err = app.models.PermissionModel.Permissions.AddForUser(user.ID, "movies:read")
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	token, err := app.models.TokenModel.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	app.background(func() {
		err = app.mailer.Send(user.Email, "user_welcome.tmpl", map[string]interface{}{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		})

		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(c, http.StatusCreated, envelope{"user": user})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) activateUserHandler(c *gin.Context) {
	var input struct {
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	v := validator.New()

	if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	// retrive the details of the user associated with the token
	// using the token details
	user, err := app.models.UserModel.Users.GetForToken(data.ScopeActivation, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			app.failedValidationResponse(c, v.Errors)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	user.Activated = true

	err = app.models.UserModel.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	err = app.models.TokenModel.Tokens.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{"user": user})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) updateUserPasswordHandler(c *gin.Context) {
	var input struct {
		Password       string `json:"password"`
		TokenPlaintext string `json:"token"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	v := validator.New()
	data.ValidatePasswordPlaintext(v, input.Password)
	data.ValidateTokenPlaintext(v, input.TokenPlaintext)

	if !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	// get the user
	user, err := app.models.UserModel.Users.GetForToken(data.ScopePasswordRest, input.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.failedValidationResponse(c, v.Errors)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	// update the user
	err = user.Set(input.Password)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	err = app.models.UserModel.Users.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}
	}

	// delete the reset token
	err = app.models.TokenModel.Tokens.DeleteAllForUser(data.ScopePasswordRest, user.ID)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	err = app.writeJSON(c, http.StatusOK, envelope{
		"message": "your password was successfully reset",
	})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}
