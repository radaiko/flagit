// Command flagit runs the Flagit ticket server: a public API for apps on one
// port, and the internal API plus admin dashboard on another.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"flagit/internal/api"
	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/overlay"
	"flagit/internal/service"
	"flagit/internal/version"
	"flagit/internal/webhook"
)

// shutdownGrace is how long in-flight requests get to finish on SIGINT/SIGTERM.
const shutdownGrace = 15 * time.Second

// webhookDrainGrace bounds the wait for outbound webhooks after the servers
// have stopped. A missed delivery is recoverable — Hermes polls — so this is
// deliberately shorter than the delivery retry budget.
const webhookDrainGrace = 20 * time.Second

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "flagit:", err)
		os.Exit(1)
	}
}

// config is the resolved runtime configuration.
type config struct {
	port      int
	adminPort int
	dbPath    string
	adminKey  string
	publicURL string
	dev       bool
	viteURL   string
	logLevel  string
	// commit is the revision this container was deployed from. Empty falls
	// back to whatever the linker stamped in at build time.
	commit string
}

// parseFlags reads configuration from flags, falling back to environment
// variables. Flags win, so a container can be overridden on the command line.
func parseFlags(args []string, out io.Writer) (*config, error) {
	fs := flag.NewFlagSet("flagit", flag.ContinueOnError)
	fs.SetOutput(out)

	cfg := &config{}
	fs.IntVar(&cfg.port, "port", envInt("FLAGIT_PORT", 8080), "port for the public API and web overlay")
	fs.IntVar(&cfg.adminPort, "admin-port", envInt("FLAGIT_ADMIN_PORT", 3000), "port for the internal API and admin dashboard")
	fs.StringVar(&cfg.dbPath, "db-path", env("FLAGIT_DB_PATH", "./data/flagit.db"), "path to the SQLite database")
	fs.StringVar(&cfg.adminKey, "admin-key", env("FLAGIT_ADMIN_KEY", ""), "admin API key (env FLAGIT_ADMIN_KEY); generated and printed on first start if unset")
	fs.StringVar(&cfg.publicURL, "public-url", env("FLAGIT_PUBLIC_URL", ""), "externally reachable base URL, used in webhook payloads (env FLAGIT_PUBLIC_URL)")
	fs.BoolVar(&cfg.dev, "dev", envBool("FLAGIT_DEV", false), "dev mode: proxy the frontend to the Vite dev server instead of serving embedded files")
	fs.StringVar(&cfg.viteURL, "vite-url", env("FLAGIT_VITE_URL", overlay.DefaultViteURL), "Vite dev server URL, used with -dev")
	fs.StringVar(&cfg.logLevel, "log-level", env("FLAGIT_LOG_LEVEL", "info"), "log level: debug, info, warn or error")
	// SOURCE_COMMIT is read directly, not only through the FLAGIT_COMMIT mapping
	// in docker-compose.yml: a platform that injects it into the container but
	// not into compose's interpolation environment would otherwise go unnoticed.
	fs.StringVar(&cfg.commit, "commit", env("FLAGIT_COMMIT", env("SOURCE_COMMIT", "")), "git revision this deployment was built from, shown in the admin dashboard (env FLAGIT_COMMIT, else SOURCE_COMMIT; falls back to the revision stamped in at build time)")

	fs.Usage = func() {
		fmt.Fprintln(out, "flagit — self-hosted in-app ticket system")
		fmt.Fprintln(out, "\nUsage:\n  flagit [flags]\n\nFlags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	cfg.publicURL = strings.TrimSuffix(strings.TrimSpace(cfg.publicURL), "/")
	return cfg, nil
}

// run wires everything together and blocks until ctx is cancelled. It is
// separate from main so tests can drive a real server.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	cfg, err := parseFlags(args, stdout)
	if err != nil {
		// flag.ErrHelp already printed usage; exiting cleanly is correct.
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	logger := newLogger(stderr, cfg.logLevel)

	database, err := db.InitDB(cfg.dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("closing database", "error", err)
		}
	}()
	logger.Info("database ready", "path", cfg.dbPath)

	adminKeyHash, err := resolveAdminKey(database, cfg.adminKey, stdout)
	if err != nil {
		return err
	}

	sender := webhook.NewSender(logger)
	svc := service.New(database, sender, cfg.publicURL, logger)
	// Deliveries run on the sender's tracked goroutines so shutdown can drain
	// them rather than dropping a ticket that was accepted but never announced.
	svc.Dispatch = sender.Go

	srv := api.NewServer(svc, adminKeyHash, logger)
	srv.Commit = version.Resolve(cfg.commit)

	if err := mountFrontend(srv, cfg, logger); err != nil {
		return err
	}

	public := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.port),
		Handler:           srv.PublicRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	internal := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.adminPort),
		Handler:           srv.InternalRouter(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("starting flagit", "publicPort", cfg.port, "adminPort", cfg.adminPort, "dev", cfg.dev,
		"commit", srv.Commit)
	err = serve(ctx, logger, public, internal)
	drainWebhooks(logger, sender)
	return err
}

// drainWebhooks waits for in-flight Hermes deliveries once the servers have
// stopped accepting requests. Bounded, because a wedged Hermes must not stop
// the process from exiting.
func drainWebhooks(logger *slog.Logger, sender *webhook.Sender) {
	ctx, cancel := context.WithTimeout(context.Background(), webhookDrainGrace)
	defer cancel()

	if sender.Wait(ctx) {
		return
	}
	logger.Warn("gave up waiting for in-flight webhooks",
		"after", webhookDrainGrace.String(),
		"note", "Hermes can still pick these tickets up from /internal/poll")
}

// serve runs both servers until one fails or ctx is cancelled, then shuts them
// down gracefully.
func serve(ctx context.Context, logger *slog.Logger, servers ...*http.Server) error {
	errs := make(chan error, len(servers))
	for _, s := range servers {
		go func(s *http.Server) {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("listen on %s: %w", s.Addr, err)
				return
			}
			errs <- nil
		}(s)
	}

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errs:
		if runErr != nil {
			logger.Error("server stopped", "error", runErr)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	for _, s := range servers {
		if err := s.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "addr", s.Addr, "error", err)
		}
	}
	logger.Info("flagit stopped")
	return runErr
}

// resolveAdminKey settles on the admin key to use and returns its SHA-256
// hash, in order of precedence: the flag/env value, a hash persisted from a
// previous start, or a freshly generated key.
//
// Only the hash is ever stored, the same way device tokens are handled, so a
// copy of the database does not hand over admin access. A generated key is
// therefore printed exactly once — nothing can recover it afterwards.
func resolveAdminKey(database *db.DB, provided string, stdout io.Writer) (string, error) {
	if provided = strings.TrimSpace(provided); provided != "" {
		hash := db.HashAdminKey(provided)
		// Persisted so a later restart without the env var still accepts the
		// same key.
		if err := database.SetSetting(model.SettingAdminKeyHash, hash); err != nil {
			return "", fmt.Errorf("store admin key: %w", err)
		}
		return hash, nil
	}

	stored, err := database.GetSetting(model.SettingAdminKeyHash, "")
	if err != nil {
		return "", fmt.Errorf("read admin key: %w", err)
	}
	if stored != "" {
		return stored, nil
	}

	generated, err := db.GenerateAdminKey()
	if err != nil {
		return "", fmt.Errorf("generate admin key: %w", err)
	}
	hash := db.HashAdminKey(generated)
	if err := database.SetSetting(model.SettingAdminKeyHash, hash); err != nil {
		return "", fmt.Errorf("store admin key: %w", err)
	}

	fmt.Fprintf(stdout, "\n  No FLAGIT_ADMIN_KEY set — generated one for this instance:\n\n    %s\n\n"+
		"  Save it now: only its hash is stored, so this is the only time it can\n"+
		"  be shown. Send it as the X-Admin-Key header.\n\n", generated)
	return hash, nil
}

// mountFrontend attaches the overlay and dashboard handlers, proxying to Vite
// in dev mode. A missing frontend build is a warning, not a failure: the API
// is useful on its own.
func mountFrontend(srv *api.Server, cfg *config, logger *slog.Logger) error {
	if cfg.dev {
		overlayProxy, err := overlay.DevProxy(cfg.viteURL, "")
		if err != nil {
			return err
		}
		dashboardProxy, err := overlay.DevProxy(cfg.viteURL, "/internal/admin")
		if err != nil {
			return err
		}
		srv.Overlay = overlayProxy
		srv.Dashboard = dashboardProxy
		logger.Info("dev mode: proxying frontend to Vite", "url", cfg.viteURL)
		return nil
	}

	if !overlay.Built() {
		logger.Warn("no frontend build embedded, serving the API only (run `make web` before `make build`)")
		return nil
	}

	overlayHandler, err := overlay.OverlayHandler()
	if err != nil {
		return err
	}
	dashboardHandler, err := overlay.DashboardHandler("/internal/admin")
	if err != nil {
		return err
	}
	srv.Overlay = overlayHandler
	srv.Dashboard = dashboardHandler
	return nil
}

// newLogger builds the structured logger every component shares.
func newLogger(w io.Writer, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl}))
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	raw := env(key, "")
	if raw == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
