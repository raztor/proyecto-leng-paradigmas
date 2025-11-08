package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"proyecto-leng-paradigmas/ejemplo/internal/model"
)

// ErrNotFound se retorna cuando un target no existe.
var ErrNotFound = errors.New("target no encontrado")

// OpenSQLite abre (o crea) el archivo SQLite y aplica pragmas basicos.
func OpenSQLite(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("no se pudo crear directorio para sqlite: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir sqlite: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("no se pudo habilitar foreign_keys: %w", err)
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// TargetRepository gestiona la persistencia de los servicios monitoreados.
type TargetRepository struct {
	db *sql.DB
}

// NewTargetRepository inicializa la tabla necesaria para almacenar targets.
func NewTargetRepository(db *sql.DB) (*TargetRepository, error) {
	repo := &TargetRepository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, err
	}
	return repo, nil
}

func (r *TargetRepository) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS targets (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		kind TEXT NOT NULL,
		url TEXT,
		host TEXT,
		port INTEGER,
		frequency_ns INTEGER NOT NULL,
		timeout_ns INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("no se pudo crear tabla targets: %w", err)
	}
	return nil
}

// List devuelve todos los targets almacenados.
func (r *TargetRepository) List(ctx context.Context) ([]model.Target, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, kind, url, host, port, frequency_ns, timeout_ns
		FROM targets
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("no se pudo listar targets: %w", err)
	}
	defer rows.Close()

	var targets []model.Target
	for rows.Next() {
		var (
			t       model.Target
			kind    string
			freqNS  int64
			timeout int64
		)
		if err := rows.Scan(&t.ID, &t.Name, &kind, &t.URL, &t.Host, &t.Port, &freqNS, &timeout); err != nil {
			return nil, fmt.Errorf("fila invalida: %w", err)
		}
		t.Kind = model.TargetKind(kind)
		t.Frequency = time.Duration(freqNS)
		t.Timeout = time.Duration(timeout)
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return targets, nil
}

// Get recupera un target especifico.
func (r *TargetRepository) Get(ctx context.Context, id string) (model.Target, error) {
	var (
		t       model.Target
		kind    string
		freqNS  int64
		timeout int64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, kind, url, host, port, frequency_ns, timeout_ns
		FROM targets
		WHERE id = ?`, id).Scan(&t.ID, &t.Name, &kind, &t.URL, &t.Host, &t.Port, &freqNS, &timeout)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Target{}, ErrNotFound
	}
	if err != nil {
		return model.Target{}, fmt.Errorf("no se pudo obtener target %q: %w", id, err)
	}
	t.Kind = model.TargetKind(kind)
	t.Frequency = time.Duration(freqNS)
	t.Timeout = time.Duration(timeout)
	return t, nil
}

// Create agrega un nuevo target.
func (r *TargetRepository) Create(ctx context.Context, target model.Target) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO targets (id, name, kind, url, host, port, frequency_ns, timeout_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, target.ID, target.Name, string(target.Kind), target.URL, target.Host, target.Port, target.Frequency.Nanoseconds(), target.Timeout.Nanoseconds())
	if err != nil {
		return fmt.Errorf("no se pudo crear target %q: %w", target.ID, err)
	}
	return nil
}

// Update modifica un target existente.
func (r *TargetRepository) Update(ctx context.Context, target model.Target) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE targets
		SET name = ?, kind = ?, url = ?, host = ?, port = ?, frequency_ns = ?, timeout_ns = ?, updated_at = datetime('now')
		WHERE id = ?
	`, target.Name, string(target.Kind), target.URL, target.Host, target.Port, target.Frequency.Nanoseconds(), target.Timeout.Nanoseconds(), target.ID)
	if err != nil {
		return fmt.Errorf("no se pudo actualizar target %q: %w", target.ID, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// Upsert crea o actualiza segun exista el registro.
func (r *TargetRepository) Upsert(ctx context.Context, target model.Target) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO targets (id, name, kind, url, host, port, frequency_ns, timeout_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			kind = excluded.kind,
			url = excluded.url,
			host = excluded.host,
			port = excluded.port,
			frequency_ns = excluded.frequency_ns,
			timeout_ns = excluded.timeout_ns,
			updated_at = datetime('now')
	`, target.ID, target.Name, string(target.Kind), target.URL, target.Host, target.Port, target.Frequency.Nanoseconds(), target.Timeout.Nanoseconds())
	if err != nil {
		return fmt.Errorf("no se pudo upsert target %q: %w", target.ID, err)
	}
	return nil
}

// Delete elimina un target.
func (r *TargetRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM targets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("no se pudo eliminar target %q: %w", id, err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}
