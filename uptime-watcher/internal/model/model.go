package model

import (
	"time"
)

// TargetKind identifica el tipo de verificación que se realizará.
type TargetKind string

const (
	TargetHTTP TargetKind = "http"
	TargetTCP  TargetKind = "tcp"
)

// Target define la configuración de un servicio a monitorear.
type Target struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Kind      TargetKind    `json:"kind"`
	URL       string        `json:"url,omitempty"`
	Host      string        `json:"host,omitempty"`
	Port      int           `json:"port,omitempty"`
	Frequency time.Duration `json:"frequency"`
	Timeout   time.Duration `json:"timeout"`
}

// CheckResult representa el resultado de un chequeo puntual.
type CheckResult struct {
	TargetID   string        `json:"target_id"`
	CheckedAt  time.Time     `json:"checked_at"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	Message    string        `json:"message"`
	StatusCode int           `json:"status_code,omitempty"`
}

// TargetStatus resume el estado actual de un Target.
type TargetStatus struct {
	Target     Target       `json:"target"`
	LastCheck  *CheckResult `json:"last_check,omitempty"`
	UptimePerc float64      `json:"uptime_perc"`
	// Failures seguidas para detectar alertas simples.
	ConsecutiveFailures int `json:"consecutive_failures"`
}
