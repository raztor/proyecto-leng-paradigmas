package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"proyecto-leng-paradigmas/ejemplo/internal/model"
)

// Duration permite parsear strings como "30s" desde archivos JSON.
type Duration time.Duration

// UnmarshalJSON convierte strings en time.Duration.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("duracion invalida: %w", err)
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("no se pudo parsear duracion %q: %w", raw, err)
	}
	*d = Duration(parsed)
	return nil
}

type rawConfig struct {
	Targets []rawTarget `json:"targets"`
}

type rawTarget struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	URL       string   `json:"url"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	Frequency Duration `json:"frequency"`
	Timeout   Duration `json:"timeout"`
}

// Config representa el resultado final del parseo del archivo de configuracion.
type Config struct {
	Targets []model.Target
}

// Load lee y parsea un archivo JSON con la lista de targets.
func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("no se pudo abrir config %q: %w", path, err)
	}
	defer f.Close()

	var raw rawConfig
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("configuracion JSON invalida: %w", err)
	}

	cfg := Config{Targets: make([]model.Target, 0, len(raw.Targets))}
	for _, target := range raw.Targets {
		m, err := mapTarget(target)
		if err != nil {
			return Config{}, err
		}
		cfg.Targets = append(cfg.Targets, m)
	}
	return cfg, nil
}

func mapTarget(raw rawTarget) (model.Target, error) {
	if raw.ID == "" {
		return model.Target{}, fmt.Errorf("target sin id")
	}
	if raw.Name == "" {
		raw.Name = raw.ID
	}
	if raw.Kind == "" {
		return model.Target{}, fmt.Errorf("target %q sin kind", raw.ID)
	}
	kind := model.TargetKind(strings.ToLower(raw.Kind))
	switch kind {
	case model.TargetHTTP:
		if raw.URL == "" {
			return model.Target{}, fmt.Errorf("target %q requiere url", raw.ID)
		}
	case model.TargetTCP:
		if raw.Host == "" || raw.Port == 0 {
			return model.Target{}, fmt.Errorf("target %q requiere host y port", raw.ID)
		}
	default:
		return model.Target{}, fmt.Errorf("target %q tiene kind desconocido %q", raw.ID, raw.Kind)
	}
	freq := time.Duration(raw.Frequency)
	if freq <= 0 {
		freq = 30 * time.Second
	}
	timeout := time.Duration(raw.Timeout)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return model.Target{
		ID:        raw.ID,
		Name:      raw.Name,
		Kind:      kind,
		URL:       raw.URL,
		Host:      raw.Host,
		Port:      raw.Port,
		Frequency: freq,
		Timeout:   timeout,
	}, nil
}
