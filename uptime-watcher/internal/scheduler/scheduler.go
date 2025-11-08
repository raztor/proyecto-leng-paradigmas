package scheduler

import (
	"context"
	"sync"
	"time"

	"proyecto-leng-paradigmas/ejemplo/internal/check"
	"proyecto-leng-paradigmas/ejemplo/internal/model"
	"proyecto-leng-paradigmas/ejemplo/internal/store"
)

// Logger define interfaz minima para registrar eventos.
type Logger interface {
	Printf(format string, v ...any)
}

type worker struct {
	trigger chan struct{}
	cancel  context.CancelFunc
}

// Scheduler coordina la ejecucion periodica de chequeos.
type Scheduler struct {
	runner  *check.Runner
	store   *store.Store
	logger  Logger
	baseCtx context.Context

	mu      sync.RWMutex
	workers map[string]*worker
	wg      sync.WaitGroup
}

// New crea un scheduler listo para iniciar.
func New(runner *check.Runner, store *store.Store, logger Logger) *Scheduler {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Scheduler{
		runner:  runner,
		store:   store,
		logger:  logger,
		workers: make(map[string]*worker),
	}
}

// Start almacena el contexto base y lanza workers para los targets existentes.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.baseCtx = ctx
	s.mu.Unlock()

	for _, target := range s.store.Targets() {
		s.UpsertTarget(target)
	}
}

// UpsertTarget crea o reinicia el worker asociado a un target.
func (s *Scheduler) UpsertTarget(target model.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, ok := s.workers[target.ID]; ok {
		w.cancel()
		delete(s.workers, target.ID)
	}
	if s.baseCtx == nil {
		return
	}
	s.spawnWorkerLocked(target)
}

// RemoveTarget detiene el worker asociado a un target.
func (s *Scheduler) RemoveTarget(targetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if w, ok := s.workers[targetID]; ok {
		w.cancel()
		delete(s.workers, targetID)
	}
}

// Trigger fuerza la ejecucion inmediata del chequeo de un target.
func (s *Scheduler) Trigger(targetID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	w, ok := s.workers[targetID]
	if !ok {
		return false
	}
	select {
	case w.trigger <- struct{}{}:
	default:
	}
	return true
}

// Wait bloquea hasta que todas las goroutines finalicen.
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) spawnWorkerLocked(target model.Target) {
	ctx, cancel := context.WithCancel(s.baseCtx)
	trigger := make(chan struct{}, 1)
	s.workers[target.ID] = &worker{
		trigger: trigger,
		cancel:  cancel,
	}
	s.wg.Add(1)
	go s.runWorker(ctx, target, trigger)
}

func (s *Scheduler) runWorker(ctx context.Context, target model.Target, trigger <-chan struct{}) {
	defer s.wg.Done()

	s.execute(ctx, target)

	ticker := time.NewTicker(target.Frequency)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.execute(ctx, target)
		case <-trigger:
			s.execute(ctx, target)
		}
	}
}

func (s *Scheduler) execute(ctx context.Context, target model.Target) {
	checkCtx, cancel := context.WithTimeout(ctx, target.Timeout)
	defer cancel()
	result := s.runner.Run(checkCtx, target)
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now()
	}
	s.store.Update(result)
	if result.Success {
		s.logger.Printf("target %s OK (%.0fms)", target.ID, result.Duration.Seconds()*1000)
	} else {
		s.logger.Printf("target %s fallo: %s", target.ID, result.Message)
	}
}

type noopLogger struct{}

func (noopLogger) Printf(string, ...any) {}
