package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"greenlight.fyerfyer.net/internal/data"
	"greenlight.fyerfyer.net/internal/validator"
)

func (app *application) createAuthenticationTokenHandler(c *gin.Context) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	v := validator.New()

	data.ValidateEmail(v, input.Email)
	data.ValidatePasswordPlaintext(v, input.Password)

	if !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	user, err := app.models.UserModel.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.invalidCredentialsResponse(c)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	// check if the password is correct
	match, err := user.Password.Matches(input.Password)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	if !match {
		app.invalidCredentialsResponse(c)
		return
	}

	token, err := app.models.TokenModel.Tokens.New(user.ID, 24*time.Hour, data.ScopeAuthentication)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	err = app.writeJSON(c, http.StatusCreated, envelope{"authentication_token": token})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) createPasswordResetTokenHandler(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.badRequestResponse(c, err)
		return
	}

	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	user, err := app.models.UserModel.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(c, v.Errors)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	// check if the user is activated
	if !user.Activated {
		v.AddError("email", "user account must be activated")
		app.failedValidationResponse(c, v.Errors)
		return
	}

	// generate token for resetting password
	token, err := app.models.TokenModel.Tokens.New(user.ID, 45*time.Minute, data.ScopePasswordRest)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	// email the user
	app.background(func() {
		data := map[string]interface{}{
			"passwordResetToken": token.Plaintext,
		}

		err = app.mailer.Send(user.Email, "token_password_reset.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(c, http.StatusAccepted, envelope{
		"message": "an email will be sent to you containing password reset instructions",
	})
	if err != nil {
		app.serverErrorResponse(c, err)
	}
}

func (app *application) createActivateUserTokenHandler(c *gin.Context) {
	var input struct {
		Email string `json:"email"`
	}

	err := app.readJSON(c, &input)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	v := validator.New()
	if data.ValidateEmail(v, input.Email); !v.Valid() {
		app.failedValidationResponse(c, v.Errors)
		return
	}

	user, err := app.models.UserModel.Users.GetByEmail(input.Email)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("email", "no matching email address found")
			app.failedValidationResponse(c, v.Errors)
		default:
			app.serverErrorResponse(c, err)
		}

		return
	}

	if user.Activated {
		v.AddError("email", "user has already been activated")
		app.failedValidationResponse(c, v.Errors)
		return
	}

	token, err := app.models.TokenModel.Tokens.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		app.serverErrorResponse(c, err)
		return
	}

	app.background(func() {
		data := map[string]interface{}{
			"activationToken": token.Plaintext,
		}

		err = app.mailer.Send(user.Email, "token_activation.tmpl", data)
		if err != nil {
			app.logger.PrintError(err, nil)
		}
	})

	err = app.writeJSON(c, http.StatusAccepted, envelope{
		"message": "an email will be sent to you containing activation instructions",
	})

	if err != nil {
		app.serverErrorResponse(c, err)
	}
}
