package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudmanager/cloudmanager/internal/api"
	"github.com/cloudmanager/cloudmanager/internal/auth"
	"github.com/cloudmanager/cloudmanager/internal/config"
	"github.com/cloudmanager/cloudmanager/internal/db"
	"github.com/cloudmanager/cloudmanager/internal/migrate"
	"github.com/cloudmanager/cloudmanager/internal/repo"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

func main() {
	cfgPath := os.Getenv("CM_CONFIG")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := migrate.Up(ctx, cfg.DatabaseURL); err != nil {
		log.Fatal("migrate: ", err)
	}
	if cfg.DevBootstrap {
		pool0, err0 := db.Connect(ctx, cfg.DatabaseURL)
		if err0 != nil {
			log.Fatal("bootstrap connect: ", err0)
		}
		if err := repo.DevBootstrap(ctx, pool0); err != nil {
			log.Fatal("bootstrap: ", err)
		}
		pool0.Close()
	}
	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	signer := auth.NewSigner(cfg.SessionSecret)
	mw := &api.Middleware{
		Pool:   pool,
		Signer: signer,
		Dev:    cfg.DevBootstrap,
		DevKey: os.Getenv("CM_DEV_BEARER"),
	}
	srv := &api.Server{Pool: pool, Signer: signer, Cfg: cfg}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	if len(cfg.CORSOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowedHeaders:   []string{"*"},
			AllowCredentials: true,
		}))
	}
	r.Use(httprate.Limit(300, 1*time.Minute))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })

	// unauthenticated: dev session cookie
	r.Get("/api/v1/auth/dev", srv.DevLogin)

	r.Group(func(r chi.Router) {
		r.Use(mw.Auth)
		r.Get("/api/v1/auth/me", srv.Me)
		r.Get("/api/v1/pve/vms", srv.PVEVMs)
		r.Post("/api/v1/pve/connection", srv.PostPVEConnection)
		r.Post("/api/v1/pve/power", srv.PVEPower)
		r.Get("/api/v1/templates", srv.ListTemplates)
		r.Post("/api/v1/templates", srv.PostTemplate)
		r.Get("/api/v1/flavors", srv.ListFlavors)
		r.Post("/api/v1/flavors", srv.PostFlavor)
		r.Get("/api/v1/audit", srv.ListAudit)
		r.Get("/api/v1/orgs", srv.ListOrgs)
		// Terraform
		r.Get("/api/v1/tf/workspaces", srv.ListWorkspaces)
		r.Post("/api/v1/tf/workspaces", srv.CreateWorkspace)
		r.Post("/api/v1/tf/workspaces/{id}/config", srv.UploadConfig)
		r.Post("/api/v1/tf/workspaces/{id}/plan", srv.EnqueuePlan)
		r.Post("/api/v1/tf/workspaces/{id}/apply", srv.EnqueueApply)
		r.Get("/api/v1/tf/runs/{id}", srv.GetRun)
		r.Get("/api/v1/apikeys", srv.ListAPIKeys)
		r.Post("/api/v1/apikeys", srv.CreateAPIKey)
	})
	// static SPA (dev: Vite; prod: no embed in this path — set CM_WEB_ROOT)
	web := os.Getenv("CM_WEB_ROOT")
	if web == "" {
		if d, e := os.Getwd(); e == nil {
			cand := filepath.Join(d, "apps", "web", "dist")
			if st, e2 := os.Stat(cand); e2 == nil && st.IsDir() {
				web = cand
			}
		}
	}
	if _, e := os.Stat(web); e == nil {
		fs := http.FileServer(http.Dir(web))
		r.Get("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/" || (len(req.URL.Path) > 1 && !hasFile(web, req.URL.Path[1:])) {
				req.URL.Path = "/"
			}
			fs.ServeHTTP(w, req)
		}))
	} else {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`<html><body>Cloudmanager API — <a href="/api/v1/auth/dev">/api/v1/auth/dev</a> (dev) — build <code>apps/web</code> and set <code>CM_WEB_ROOT</code> for UI</body></html>`))
		})
	}
	addr := cfg.HTTPAddr
	if a := os.Getenv("CM_HTTP_ADDR"); a != "" {
		addr = a
	}
	s := &http.Server{Addr: addr, Handler: r, ReadHeaderTimeout: 20 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 2 * time.Minute}
	log.Println("listening", addr)
	go func() { log.Fatal(s.ListenAndServe()) }()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	_ = s.Shutdown(context.Background())
}

func hasFile(root, p string) bool {
	_, e := os.Stat(root + string(os.PathSeparator) + p)
	return e == nil
}
