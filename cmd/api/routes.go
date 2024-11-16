package main

import (
	"expvar"

	"github.com/gin-gonic/gin"
)

func (app *application) routes() *gin.Engine {
	r := gin.New()

	// r.Use(gin.Logger())
	// r.Use(gin.Recovery())
	r.Use(app.metrics())
	r.Use(app.recoverPanic())
	r.Use(app.enableCORS())
	r.Use(app.rateLimitor())
	r.Use(app.authenticate())
	r.GET("/v1/healthcheck", app.healthcheckHandler)
	r.POST("/v1/users", app.registerUserHandler)
	r.PUT("/v1/users/activated", app.activateUserHandler)
	r.POST("/v1/tokens/authentication", app.createAuthenticationTokenHandler)
	r.POST("/v1/tokens/password-reset", app.createPasswordResetTokenHandler)
	r.PUT("/v1/users/password", app.updateUserPasswordHandler)
	r.POST("/v1/tokens/activation", app.createActivateUserTokenHandler)

	apiv1Read := r.Group("/v1")
	apiv1Write := r.Group("/v1")
	apiv1Read.Use(app.requirePermission("movies:read"))
	{
		apiv1Read.GET("/movies", app.listMoviesHandler)
		apiv1Read.GET("/movies/:id", app.showMovieHandler)
	}

	apiv1Write.Use(app.requirePermission("movies:write"))
	{
		apiv1Write.POST("/movies", app.createMovieHandler)
		apiv1Write.PATCH("/movies/:id", app.updateMovieHandler)
		apiv1Write.DELETE("/movies/:id", app.deleteMovieHandler)
	}

	r.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	r.NoRoute(app.notFoundResponse)
	r.NoMethod(app.methodNotAllowedResponse)

	return r
}
