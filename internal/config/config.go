package config

import (
	"expvar"
	"flag"
	"runtime"
	"strings"
	"time"

	"os"

	"greenlight.fyerfyer.net/internal/jsonlog"
)

type Config struct {
	Version string
	Port    int
	Env     string
	DB      struct {
		Dsn          string
		MaxOpenConns int
		MaxIdleConns int
		MaxIdleTime  string
	}

	Limiter struct {
		Rps    float64
		Burst  int
		Enable bool
	}

	Smtp struct {
		Host     string
		Port     int
		Username string
		Password string
		Sender   string
	}

	Cors struct {
		TrustedOrigins []string
	}
}

var Cfg Config
var Logger *jsonlog.Logger

const version = "1.0.0"

func InitConfig() {
	Cfg.Version = version
	// flag config
	flag.IntVar(&Cfg.Port, "port", 4000, "API server port")
	flag.StringVar(&Cfg.Env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&Cfg.DB.Dsn, "db-dsn", "", "PostgreSQL DSN")

	// read the database configure
	flag.IntVar(&Cfg.DB.MaxIdleConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&Cfg.DB.MaxOpenConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&Cfg.DB.MaxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")

	// read the rate limitor configure
	flag.Float64Var(&Cfg.Limiter.Rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&Cfg.Limiter.Burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&Cfg.Limiter.Enable, "limiter-enabled", true, "Enable rate limiter")

	// read the email configure
	flag.StringVar(&Cfg.Smtp.Host, "smtp-host", "sandbox.smtp.mailtrap.io", "SMTP host")
	flag.IntVar(&Cfg.Smtp.Port, "smtp-port", 25, "SMTP port")
	flag.StringVar(&Cfg.Smtp.Username, "smtp-username", "347b3782b87de5", "SMTP username")
	flag.StringVar(&Cfg.Smtp.Password, "smtp-password", "c4e438ea2a35d5", "SMTP password")
	flag.StringVar(&Cfg.Smtp.Sender, "smtp-sender", "Greenlight <no-reply@greenlight.fyerfyer.net>", "SMTP sender")

	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		Cfg.Cors.TrustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()
	// log.Println(Cfg.DB.Dsn)

	// config the logger
	Logger = jsonlog.New(os.Stdout, jsonlog.LevelInfo)
}

func ConfigExpvar() {
	expvar.NewString("version").Set(version)
	expvar.Publish("goroutine", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))
	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))
}
