package check

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"proyecto-leng-paradigmas/ejemplo/internal/model"
)

// Runner ejecuta chequeos segun el tipo del target.
type Runner struct {
	HTTPClient *http.Client
}

// NewRunner crea un Runner con clientes por defecto.
func NewRunner() *Runner {
	return &Runner{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Run ejecuta el chequeo apropiado y retorna un CheckResult.
func (r *Runner) Run(ctx context.Context, target model.Target) model.CheckResult {
	switch target.Kind {
	case model.TargetHTTP:
		return r.checkHTTP(ctx, target)
	case model.TargetTCP:
		return r.checkTCP(ctx, target)
	default:
		return model.CheckResult{
			TargetID:  target.ID,
			CheckedAt: time.Now(),
			Success:   false,
			Message:   fmt.Sprintf("tipo de target desconocido: %s", target.Kind),
		}
	}
}

func (r *Runner) checkHTTP(ctx context.Context, target model.Target) model.CheckResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		return model.CheckResult{
			TargetID:  target.ID,
			CheckedAt: time.Now(),
			Duration:  time.Since(start),
			Success:   false,
			Message:   fmt.Sprintf("no se pudo crear request: %v", err),
		}
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return model.CheckResult{
			TargetID:  target.ID,
			CheckedAt: time.Now(),
			Duration:  time.Since(start),
			Success:   false,
			Message:   fmt.Sprintf("error HTTP: %v", err),
		}
	}
	defer resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 400
	return model.CheckResult{
		TargetID:   target.ID,
		CheckedAt:  time.Now(),
		Duration:   time.Since(start),
		Success:    success,
		Message:    resp.Status,
		StatusCode: resp.StatusCode,
	}
}

func (r *Runner) checkTCP(ctx context.Context, target model.Target) model.CheckResult {
	start := time.Now()
	address := fmt.Sprintf("%s:%d", target.Host, target.Port)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return model.CheckResult{
			TargetID:  target.ID,
			CheckedAt: time.Now(),
			Duration:  time.Since(start),
			Success:   false,
			Message:   fmt.Sprintf("conexion fallida: %v", err),
		}
	}
	conn.Close()
	return model.CheckResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
		Duration:  time.Since(start),
		Success:   true,
		Message:   "tcp ok",
	}
}
