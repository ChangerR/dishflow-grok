package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/changerr/dishflow-grok/internal/app"
	"github.com/changerr/dishflow-grok/internal/config"
	"github.com/changerr/dishflow-grok/internal/httpx"
	"github.com/changerr/dishflow-grok/internal/migrate"
	"github.com/changerr/dishflow-grok/internal/persist"
	"github.com/changerr/dishflow-grok/internal/worker"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: dishflow <serve|worker|migrate|healthcheck>\n")
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	cmd := os.Args[1]
	switch cmd {
	case "migrate":
		if err := migrate.Up(cfg.MySQLDSN); err != nil {
			log.Fatal(err)
		}
		db, err := persist.OpenMySQL(cfg.MySQLDSN)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		a := app.New(cfg, db, persist.OpenRedis(cfg), nil)
		if err := a.BootstrapPlatform(context.Background()); err != nil {
			log.Fatal(err)
		}
		log.Println("migrate ok")
	case "serve":
		runServe(cfg)
	case "worker":
		runWorker(cfg)
	case "healthcheck":
		runHealth(cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %s\n", cmd)
		os.Exit(2)
	}
}

func runServe(cfg config.Config) {
	db, err := persist.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rdb := persist.OpenRedis(cfg)
	defer rdb.Close()
	application := app.New(cfg, db, rdb, nil)
	if err := application.BootstrapPlatform(context.Background()); err != nil {
		log.Fatal(err)
	}
	srv := httpx.NewServer(application)
	httpSrv := &http.Server{Addr: cfg.HTTPAddr, Handler: srv}
	go func() {
		log.Printf("listen %s", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	wait()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func runWorker(cfg config.Config) {
	db, err := persist.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rdb := persist.OpenRedis(cfg)
	defer rdb.Close()
	application := app.New(cfg, db, rdb, nil)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Println("worker started")
	worker.Run(ctx, application)
}

func runHealth(cfg config.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := persist.OpenMySQL(cfg.MySQLDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rdb := persist.OpenRedis(cfg)
	defer rdb.Close()
	if err := persist.PingRedis(ctx, rdb); err != nil {
		log.Fatal(err)
	}
	fmt.Println("ok")
}

func wait() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
