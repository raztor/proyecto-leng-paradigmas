package store

import (
	"errors"
	"sort"
	"sync"

	"proyecto-leng-paradigmas/ejemplo/internal/model"
)

const historyLimit = 100

// Store mantiene en memoria los resultados de los chequeos.
type Store struct {
	mu       sync.RWMutex
	targets  map[string]model.Target
	last     map[string]model.CheckResult
	history  map[string][]model.CheckResult
	failures map[string]int
}

// New crea un store pre-cargado con los targets configurados.
func New(targets []model.Target) *Store {
	tmap := make(map[string]model.Target, len(targets))
	for _, t := range targets {
		tmap[t.ID] = t
	}
	return &Store{
		targets:  tmap,
		last:     make(map[string]model.CheckResult),
		history:  make(map[string][]model.CheckResult),
		failures: make(map[string]int),
	}
}

// Targets devuelve la lista de servicios registrados.
func (s *Store) Targets() []model.Target {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Target, 0, len(s.targets))
	for _, t := range s.targets {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// UpsertTarget agrega o actualiza la definicion de un target.
func (s *Store) UpsertTarget(target model.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.targets == nil {
		s.targets = make(map[string]model.Target)
	}
	if s.history == nil {
		s.history = make(map[string][]model.CheckResult)
	}
	if s.failures == nil {
		s.failures = make(map[string]int)
	}
	s.targets[target.ID] = target
	if _, ok := s.history[target.ID]; !ok {
		s.history[target.ID] = nil
	}
}

// RemoveTarget elimina toda traza de un target.
func (s *Store) RemoveTarget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.targets, id)
	delete(s.last, id)
	delete(s.history, id)
	delete(s.failures, id)
}

// Update almacena un nuevo resultado y actualiza estadisticas basicas.
func (s *Store) Update(result model.CheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.last[result.TargetID] = result
	h := append([]model.CheckResult{result}, s.history[result.TargetID]...)
	if len(h) > historyLimit {
		h = h[:historyLimit]
	}
	s.history[result.TargetID] = h

	if result.Success {
		s.failures[result.TargetID] = 0
	} else {
		s.failures[result.TargetID]++
	}
}

// Status devuelve el estado actual de todos los targets.
func (s *Store) Status() []model.TargetStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]model.TargetStatus, 0, len(s.targets))
	for id, target := range s.targets {
		last := s.last[id]
		history := s.history[id]
		uptime := calculateUptime(history)
		status := model.TargetStatus{
			Target:              target,
			UptimePerc:          uptime,
			ConsecutiveFailures: s.failures[id],
		}
		if !last.CheckedAt.IsZero() {
			// creamos una copia para evitar data races
			copy := last
			status.LastCheck = &copy
		}
		results = append(results, status)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Target.ID < results[j].Target.ID
	})
	return results
}

// History entrega los ultimos chequeos del target.
func (s *Store) History(targetID string, limit int) ([]model.CheckResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.targets[targetID]; !ok {
		return nil, errors.New("target no encontrado")
	}

	h := s.history[targetID]
	if limit > 0 && limit < len(h) {
		h = h[:limit]
	}

	out := make([]model.CheckResult, len(h))
	copy(out, h)
	return out, nil
}

func calculateUptime(history []model.CheckResult) float64 {
	if len(history) == 0 {
		return 0
	}
	successes := 0
	for _, res := range history {
		if res.Success {
			successes++
		}
	}
	return float64(successes) / float64(len(history)) * 100
}
