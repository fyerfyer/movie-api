package main

import (
	"errors"
	"expvar"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	// "github.com/tomasen/realip"
	"golang.org/x/time/rate"
	"greenlight.fyerfyer.net/internal/data"
	"greenlight.fyerfyer.net/internal/validator"
)

func (app *application) recoverPanic() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				ctx.Header("Connection", "close")
				app.serverErrorResponse(ctx, fmt.Errorf("%s", err))
			}
		}()

		ctx.Next()
	}
}

func (app *application) rateLimitor() gin.HandlerFunc {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}

			mu.Unlock()
		}
	}()
	return func(ctx *gin.Context) {
		if app.config.Limiter.Enable {
			ip, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
			if err != nil {
				ctx.Abort()
				app.serverErrorResponse(ctx, err)
				return
			}

			mu.Lock()
			if _, found := clients[ip]; !found {
				clients[ip] = &client{
					limiter: rate.NewLimiter(rate.Limit(app.config.Limiter.Rps), app.config.Limiter.Burst),
				}
			}

			clients[ip].lastSeen = time.Now()
			if !clients[ip].limiter.Allow() {
				mu.Unlock()
				ctx.Abort()
				app.rateLimitExceededResponse(ctx)
				return
			}

			mu.Unlock()
		}

		ctx.Next()
	}
}

func (app *application) authenticate() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Add("Vary", "Authorization")

		// retrieve the value of the authorization header from the request
		authorizationHeader := ctx.Request.Header.Get("Authorization")

		// if there's no Authorization header found, we set an anonymous user
		if authorizationHeader == "" {
			ctx.Set("user", data.AnonymousUser)
			ctx.Next() // Proceed, as anonymous user is allowed
			return
		}

		// otherwise, we expect the header to be: "Bearer <token>" format
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			ctx.Abort()
			app.invalidAuthenticationTokenResponse(ctx)
			return
		}

		token := headerParts[1]
		v := validator.New()
		if data.ValidatePasswordPlaintext(v, token); !v.Valid() {
			ctx.Abort()
			app.invalidAuthenticationTokenResponse(ctx)
			return
		}

		// retrieve the user from the token
		user, err := app.models.UserModel.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			ctx.Abort()
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(ctx)
			default:
				app.serverErrorResponse(ctx, err)
			}

			return
		}

		// add the user info to the request
		ctx.Set("user", user)

		// Do not call ctx.Next() here after authentication failure, return early with error response
		ctx.Next()
	}
}

func (app *application) requireAuthenticatedUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		user, ok := ctx.Value("user").(*data.User)
		if !ok {
			panic("missing user value in request context")
		}

		if user.IsAnonymous() {
			ctx.Abort()
			app.authenticationRequiredResponse(ctx)
			return
		}
	}
}

func (app *application) requireActivatedUser() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		app.requireAuthenticatedUser()(ctx)
		if ctx.IsAborted() {
			return
		}
		user, ok := ctx.Value("user").(*data.User)
		if !ok {
			panic("missing user value in request context")
		}

		// log.Println(user.Name)
		// if user.IsAnonymous() {
		// 	ctx.Abort()
		// 	app.authenticationRequiredResponse(ctx)
		// 	return
		// }

		if !user.Activated {
			ctx.Abort()
			app.inactiveAccountResponse(ctx)
			return
		}
	}
}

func (app *application) requirePermission(code string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		app.requireActivatedUser()(ctx)
		if ctx.IsAborted() {
			return
		}

		user, ok := ctx.Value("user").(*data.User)
		if !ok {
			panic("missing user value in request context")
		}

		permissions, err := app.models.PermissionModel.Permissions.GetAllForUser(user.ID)
		log.Println(permissions)
		if err != nil {
			ctx.Abort()
			app.serverErrorResponse(ctx, err)
			return
		}

		if !permissions.Include(code) {
			ctx.Abort()
			app.notPermittedResponse(ctx)
			return
		}

		ctx.Next()
	}
}

func (app *application) enableCORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Add("Vary", "Origin")
		ctx.Writer.Header().Add("Vary", "Access-Control-Request-Method")
		origin := ctx.Request.Header.Get("Origin")
		log.Println(origin)
		if origin != "" {
			for i := range app.config.Cors.TrustedOrigins {
				if origin == app.config.Cors.TrustedOrigins[i] {
					ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					log.Println(1)
					// the http.MethodOptions header is sent by browser as a signal for preflight request
					if ctx.Request.Method == http.MethodOptions && ctx.Request.Header.Get("Access-Control-Request-Method") != "" {

						// set the necessary preflight response for our api
						ctx.Writer.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
						ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
						log.Println(2)
						// write http.StatusOK in the header
						ctx.Writer.WriteHeader(http.StatusOK)
						log.Println(3)
						return
					}

					break
				}
			}
		}

		ctx.Next()
	}
}

func (app *application) metrics() gin.HandlerFunc {
	var (
		totalRequestReceived            = expvar.NewInt("total_requests_received")
		totalResponsesSent              = expvar.NewInt("total_responses_sent")
		totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
	)

	return func(ctx *gin.Context) {
		start := time.Now()
		totalRequestReceived.Add(1)
		ctx.Next()
		totalResponsesSent.Add(1)
		duration := time.Since(start).Microseconds()
		totalProcessingTimeMicroseconds.Add(duration)
	}
}
