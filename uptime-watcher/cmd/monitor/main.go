package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"proyecto-leng-paradigmas/ejemplo/internal/api"
	"proyecto-leng-paradigmas/ejemplo/internal/check"
	"proyecto-leng-paradigmas/ejemplo/internal/config"
	"proyecto-leng-paradigmas/ejemplo/internal/db"
	"proyecto-leng-paradigmas/ejemplo/internal/scheduler"
	"proyecto-leng-paradigmas/ejemplo/internal/service"
	"proyecto-leng-paradigmas/ejemplo/internal/store"
	"proyecto-leng-paradigmas/ejemplo/internal/ui"
)

func main() {
	addr := flag.String("addr", ":8080", "Direccion y puerto para la API")
	dbPath := flag.String("db", filepath.Join("data", "monitor.db"), "Ruta al archivo SQLite")
	seedPath := flag.String("seed", "", "Archivo JSON para poblar targets si la base esta vacia")
	flag.Parse()

	mainLogger := log.New(os.Stdout, "[monitor] ", log.LstdFlags)

	sqlDB, err := db.OpenSQLite(*dbPath)
	if err != nil {
		log.Fatalf("no se pudo abrir base de datos: %v", err)
	}
	defer func(d *sql.DB) {
		if err := d.Close(); err != nil {
			mainLogger.Printf("error cerrando base de datos: %v", err)
		}
	}(sqlDB)

	repo, err := db.NewTargetRepository(sqlDB)
	if err != nil {
		log.Fatalf("no se pudo inicializar repositorio: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := maybeSeed(ctx, repo, *seedPath, mainLogger); err != nil {
		log.Fatalf("error al aplicar seed: %v", err)
	}

	st := store.New(nil)
	runner := check.NewRunner()
	sched := scheduler.New(runner, st, mainLogger)
	svc := service.NewTargetService(repo, st, sched)

	if err := svc.Bootstrap(ctx); err != nil {
		log.Fatalf("no se pudieron cargar los targets: %v", err)
	}

	sched.Start(ctx)

	apiServer := api.New(svc)
	frontend, err := ui.New(st, svc)
	if err != nil {
		log.Fatalf("no se pudo inicializar frontend: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ui/targets/create", frontend.HandleCreate)
	mux.HandleFunc("/ui/targets/update", frontend.HandleUpdate)
	mux.HandleFunc("/ui/targets/delete", frontend.HandleDelete)
	mux.Handle("/api/", apiServer.Handler())
	mux.Handle("/healthz", apiServer.Handler())
	mux.Handle("/", frontend)

	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		mainLogger.Println("recibida señal, cerrando servidor...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			mainLogger.Printf("error cerrando servidor: %v", err)
		}
	}()

	mainLogger.Printf("servidor escuchando en %s", *addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("servidor HTTP fallo: %v", err)
	}

	sched.Wait()
	mainLogger.Println("monitor finalizado")
}

func maybeSeed(ctx context.Context, repo *db.TargetRepository, seedPath string, logger *log.Logger) error {
	if seedPath == "" {
		return nil
	}
	existing, err := repo.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		logger.Printf("se omite seed: la base ya posee %d targets", len(existing))
		return nil
	}
	if _, err := os.Stat(seedPath); err != nil {
		if os.IsNotExist(err) {
			logger.Printf("seed %s no encontrado, se omite", seedPath)
			return nil
		}
		return err
	}
	cfg, err := config.Load(seedPath)
	if err != nil {
		return err
	}
	if len(cfg.Targets) == 0 {
		return nil
	}
	for _, target := range cfg.Targets {
		if err := repo.Upsert(ctx, target); err != nil {
			logger.Printf("no se pudo insertar target %s: %v", target.ID, err)
		}
	}
	return nil
}
