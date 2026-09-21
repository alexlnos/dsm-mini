// Package vmm — виртуальные машины Synology Virtual Machine Manager.
package vmm

import (
	"context"
	"fmt"
	"strings"
)

type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

const (
	apiGuest  = "SYNO.Virtualization.API.Guest"
	apiAction = "SYNO.Virtualization.API.Guest.Action"
	apiHost   = "SYNO.Virtualization.API.Host"
)

// Service управляет виртуальными машинами.
type Service struct{ c apiClient }

// New создаёт службу.
func New(c apiClient) *Service { return &Service{c: c} }

// Guest — виртуальная машина.
type Guest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
	VCPU    int    `json:"vcpu"`
	RAMMB   int64  `json:"ram_mb"`
	DiskMB  int64  `json:"disk_mb"`
	// Autorun — запускается ли вместе с NAS.
	Autorun bool   `json:"autorun"`
	Storage string `json:"storage,omitempty"`
	// MACs пригодятся, чтобы сопоставить машину с адресом в сети.
	MACs []string `json:"macs,omitempty"`
}

// Host — ресурсы самого NAS под виртуализацию.
type Host struct {
	Name       string `json:"name"`
	FreeRAMMB  int64  `json:"free_ram_mb"`
	TotalRAMMB int64  `json:"total_ram_mb,omitempty"`
	UsedVCPU   int    `json:"used_vcpu"`
	RunningVMs int    `json:"running_vms"`
	TotalVMs   int    `json:"total_vms"`
}

// List возвращает все машины.
func (s *Service) List(ctx context.Context) ([]Guest, error) {
	var out struct {
		Guests []struct {
			GuestID   string `json:"guest_id"`
			GuestName string `json:"guest_name"`
			Status    string `json:"status"`
			VCPUNum   int    `json:"vcpu_num"`
			VRAMSize  int64  `json:"vram_size"`
			Autorun   int    `json:"autorun"`
			Storage   string `json:"storage_name"`
			VDisks    []struct {
				Size int64 `json:"vdisk_size"`
			} `json:"vdisks"`
			VNICs []struct {
				MAC string `json:"mac"`
			} `json:"vnics"`
		} `json:"guests"`
	}
	if err := s.c.Call(ctx, apiGuest, "list", 1, nil, &out); err != nil {
		return nil, err
	}

	guests := make([]Guest, 0, len(out.Guests))
	for _, g := range out.Guests {
		var disk int64
		for _, d := range g.VDisks {
			disk += d.Size
		}
		macs := make([]string, 0, len(g.VNICs))
		for _, n := range g.VNICs {
			if n.MAC != "" {
				macs = append(macs, n.MAC)
			}
		}
		guests = append(guests, Guest{
			ID:      g.GuestID,
			Name:    g.GuestName,
			Status:  g.Status,
			Running: strings.EqualFold(g.Status, "running"),
			VCPU:    g.VCPUNum,
			RAMMB:   g.VRAMSize,
			DiskMB:  disk,
			// Ненулевое значение означает автозапуск; какое именно — неважно.
			Autorun: g.Autorun != 0,
			Storage: g.Storage,
			MACs:    macs,
		})
	}
	return guests, nil
}

// Host возвращает сводку по ресурсам виртуализации.
func (s *Service) Host(ctx context.Context) (Host, error) {
	guests, err := s.List(ctx)
	if err != nil {
		return Host{}, err
	}

	host := Host{TotalVMs: len(guests)}
	for _, g := range guests {
		if g.Running {
			host.RunningVMs++
			host.UsedVCPU += g.VCPU
		}
	}

	// Сведения о хосте не критичны: если их нет, отдаём хотя бы счётчики.
	var out struct {
		Hosts []struct {
			HostName    string `json:"host_name"`
			FreeRAMSize int64  `json:"free_ram_size"`
			RAMSize     int64  `json:"ram_size"`
		} `json:"hosts"`
	}
	if err := s.c.Call(ctx, apiHost, "list", 1, nil, &out); err == nil && len(out.Hosts) > 0 {
		host.Name = out.Hosts[0].HostName
		host.FreeRAMMB = out.Hosts[0].FreeRAMSize
		host.TotalRAMMB = out.Hosts[0].RAMSize
	}
	return host, nil
}

// PowerOn запускает машину.
func (s *Service) PowerOn(ctx context.Context, id string) error {
	return s.action(ctx, "poweron", id)
}

// Shutdown просит гостевую систему завершить работу.
//
// Это мягкая остановка: ей нужен установленный в машине гостевой агент.
// Без него машина не выключится, и это ограничение самой виртуализации.
func (s *Service) Shutdown(ctx context.Context, id string) error {
	return s.action(ctx, "shutdown", id)
}

func (s *Service) action(ctx context.Context, method, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("не указана машина")
	}
	return s.c.Call(ctx, apiAction, method, 1, map[string]any{"guest_id": id}, nil)
}
