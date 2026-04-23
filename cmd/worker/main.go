package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudmanager/cloudmanager/internal/config"
	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/worker"
)

func main() {
	cfgPath := os.Getenv("CM_CONFIG")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	interval := 3 * time.Second
	if s := os.Getenv("CM_WORKER_INTERVAL_SEC"); s != "" {
		if n, e := strconv.Atoi(s); e == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	tf := cfg.TerraformPath
	if tf == "" {
		tf = "terraform"
	}
	wd := cfg.Workdir
	if wd == "" {
		wd = os.TempDir()
	}
	if err := os.MkdirAll(wd, 0o750); err != nil {
		log.Fatal(err)
	}
	log.Println("cloudmanager-worker interval", interval, "terraform", tf, "workdir", wd)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if e := worker.RunOnce(ctx, pool, tf, wd); e != nil {
				log.Println("RunOnce", e)
			}
		}
	}
}
