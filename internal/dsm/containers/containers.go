// Package containers — контейнеры Container Manager (Docker) на NAS.
package containers

import (
	"context"
	"fmt"
	"strings"
)

type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

const apiContainer = "SYNO.Docker.Container"

// Service управляет контейнерами.
type Service struct{ c apiClient }

// New создаёт службу.
func New(c apiClient) *Service { return &Service{c: c} }

// Container — контейнер.
type Container struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
	// CPU в процентах, Memory в байтах — как их отдаёт DSM.
	CPU    float64 `json:"cpu"`
	Memory int64   `json:"memory"`
}

// List возвращает контейнеры.
func (s *Service) List(ctx context.Context) ([]Container, error) {
	var out struct {
		Containers []struct {
			ID     string  `json:"id"`
			Name   string  `json:"name"`
			Image  string  `json:"image"`
			Status string  `json:"status"`
			CPU    float64 `json:"cpu"`
			Memory int64   `json:"memory"`
		} `json:"containers"`
		Total int `json:"total"`
	}
	err := s.c.Call(ctx, apiContainer, "list", 1, map[string]any{
		"limit": -1, "offset": 0,
	}, &out)
	if err != nil {
		return nil, err
	}

	list := make([]Container, 0, len(out.Containers))
	for _, c := range out.Containers {
		list = append(list, Container{
			ID:      c.ID,
			Name:    strings.TrimPrefix(c.Name, "/"),
			Image:   c.Image,
			Status:  c.Status,
			Running: strings.EqualFold(c.Status, "running"),
			CPU:     c.CPU,
			Memory:  c.Memory,
		})
	}
	return list, nil
}

// Start запускает контейнер.
func (s *Service) Start(ctx context.Context, name string) error {
	return s.action(ctx, "start", name)
}

// Stop останавливает контейнер.
func (s *Service) Stop(ctx context.Context, name string) error {
	return s.action(ctx, "stop", name)
}

// Restart перезапускает контейнер.
func (s *Service) Restart(ctx context.Context, name string) error {
	return s.action(ctx, "restart", name)
}

func (s *Service) action(ctx context.Context, method, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("не указан контейнер")
	}
	return s.c.Call(ctx, apiContainer, method, 1, map[string]any{"name": name}, nil)
}
