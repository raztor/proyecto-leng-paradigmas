package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"proyecto-leng-paradigmas/ejemplo/internal/db"
	"proyecto-leng-paradigmas/ejemplo/internal/model"
	"proyecto-leng-paradigmas/ejemplo/internal/scheduler"
	"proyecto-leng-paradigmas/ejemplo/internal/store"
)

// TargetService coordina repositorio, scheduler y store en memoria.
type TargetService struct {
	repo      *db.TargetRepository
	store     *store.Store
	scheduler *scheduler.Scheduler
}

// NewTargetService crea una nueva instancia de TargetService.
func NewTargetService(repo *db.TargetRepository, store *store.Store, sched *scheduler.Scheduler) *TargetService {
	return &TargetService{
		repo:      repo,
		store:     store,
		scheduler: sched,
	}
}

// Bootstrap carga los targets persistidos en memoria.
func (s *TargetService) Bootstrap(ctx context.Context) error {
	targets, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, target := range targets {
		s.store.UpsertTarget(target)
	}
	return nil
}

// ListTargets retorna los targets conocidos actualmente.
func (s *TargetService) ListTargets() []model.Target {
	return s.store.Targets()
}

// CreateTarget inserta un nuevo servicio a monitorear.
func (s *TargetService) CreateTarget(ctx context.Context, target model.Target) (model.Target, error) {
	if target.ID == "" {
		target.ID = uuid.NewString()
	}
	if err := validateTarget(target); err != nil {
		return model.Target{}, err
	}
	if err := s.repo.Create(ctx, target); err != nil {
		return model.Target{}, err
	}
	s.store.UpsertTarget(target)
	s.scheduler.UpsertTarget(target)
	return target, nil
}

// UpdateTarget reemplaza la configuracion de un servicio.
func (s *TargetService) UpdateTarget(ctx context.Context, target model.Target) (model.Target, error) {
	if target.ID == "" {
		return model.Target{}, errors.New("id requerido")
	}
	if err := validateTarget(target); err != nil {
		return model.Target{}, err
	}
	if err := s.repo.Update(ctx, target); err != nil {
		return model.Target{}, err
	}
	s.store.UpsertTarget(target)
	s.scheduler.UpsertTarget(target)
	return target, nil
}

// DeleteTarget elimina un servicio de la monitorizacion.
func (s *TargetService) DeleteTarget(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("id requerido")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.scheduler.RemoveTarget(id)
	s.store.RemoveTarget(id)
	return nil
}

// Trigger fuerza un chequeo inmediato.
func (s *TargetService) Trigger(id string) bool {
	return s.scheduler.Trigger(id)
}

// History obtiene el historial reciente desde memoria.
func (s *TargetService) History(id string, limit int) ([]model.CheckResult, error) {
	return s.store.History(id, limit)
}

// Status retorna el snapshot actual.
func (s *TargetService) Status() []model.TargetStatus {
	return s.store.Status()
}

func validateTarget(target model.Target) error {
	if target.Name == "" {
		return errors.New("nombre requerido")
	}
	switch target.Kind {
	case model.TargetHTTP:
		if target.URL == "" {
			return errors.New("url requerida para targets http")
		}
	case model.TargetTCP:
		if target.Host == "" || target.Port == 0 {
			return errors.New("host y port requeridos para targets tcp")
		}
	default:
		return fmt.Errorf("tipo de target desconocido: %s", target.Kind)
	}
	if target.Frequency <= 0 {
		return errors.New("frequency debe ser mayor a 0")
	}
	if target.Timeout <= 0 {
		return errors.New("timeout debe ser mayor a 0")
	}
	if target.Timeout > target.Frequency {
		return errors.New("timeout no puede ser mayor que frequency")
	}
	return nil
}

// ParseDurations ayuda a convertir strings en duraciones.
func ParseDurations(freqStr, timeoutStr string) (time.Duration, time.Duration, error) {
	freq, err := time.ParseDuration(freqStr)
	if err != nil {
		return 0, 0, fmt.Errorf("frequency invalida: %w", err)
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return 0, 0, fmt.Errorf("timeout invalido: %w", err)
	}
	return freq, timeout, nil
}
