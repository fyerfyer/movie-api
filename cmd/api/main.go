package main

import (
	// "expvar"
	"sync"

	"greenlight.fyerfyer.net/internal/config"
	"greenlight.fyerfyer.net/internal/data"
	"greenlight.fyerfyer.net/internal/jsonlog"
	"greenlight.fyerfyer.net/internal/mailer"
	// "gorm.io/driver/postgres"
	// "gorm.io/gorm"
	// "gorm.io/gorm/logger"
	// "github.com/gin-gonic/gin"
)

type application struct {
	// version string
	config config.Config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	config.InitConfig()
	config.ConfigExpvar()

	app := &application{
		config: config.Cfg,
		logger: config.Logger,
		models: data.NewModels(),
		mailer: mailer.New(config.Cfg.Smtp.Host,
			config.Cfg.Smtp.Port,
			config.Cfg.Smtp.Username,
			config.Cfg.Smtp.Password,
			config.Cfg.Smtp.Sender),
	}

	err := data.InitSql()
	if err != nil {
		app.logger.PrintFatal(err, nil)
	}

	app.logger.PrintInfo("database connection pool established", nil)

	if err := app.serve(); err != nil {
		app.logger.PrintFatal(err, nil)
	}
}
