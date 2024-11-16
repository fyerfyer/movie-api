package main

import (
	"net/http"
	// "time"

	"github.com/gin-gonic/gin"
)

func (app *application) healthcheckHandler(c *gin.Context) {
	env := envelope{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.Env,
			"version":     app.config.Version,
		},
	}

	// time.Sleep(4 * time.Second)

	jsonData, err := formatJSONWithNewline(env)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error encoding JSON"})
		return
	}

	c.String(http.StatusOK, jsonData)
}
