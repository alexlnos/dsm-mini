// Package vmm covers virtual machines in Synology Virtual Machine Manager.
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

// Service manages virtual machines.
type Service struct{ c apiClient }

// New creates the service.
func New(c apiClient) *Service { return &Service{c: c} }

// Guest is a virtual machine.
type Guest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
	VCPU    int    `json:"vcpu"`
	RAMMB   int64  `json:"ram_mb"`
	DiskMB  int64  `json:"disk_mb"`
	// Autorun tells whether it starts together with the NAS.
	Autorun bool   `json:"autorun"`
	Storage string `json:"storage,omitempty"`
	// MACs help match a machine to an address on the network.
	MACs []string `json:"macs,omitempty"`
}

// Host is the NAS resources available for virtualisation.
type Host struct {
	Name       string `json:"name"`
	FreeRAMMB  int64  `json:"free_ram_mb"`
	TotalRAMMB int64  `json:"total_ram_mb,omitempty"`
	UsedVCPU   int    `json:"used_vcpu"`
	RunningVMs int    `json:"running_vms"`
	TotalVMs   int    `json:"total_vms"`
}

// List returns every machine.
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
			// A non-zero value means autostart; which one exactly does not matter.
			Autorun: g.Autorun != 0,
			Storage: g.Storage,
			MACs:    macs,
		})
	}
	return guests, nil
}

// Host returns the summary of virtualisation resources.
func (s *Service) Host(ctx context.Context) (Host, error) {
	guests, err := s.List(ctx)
	if err != nil {
		return Host{}, err
	}
	res, err := s.Resources(ctx)
	if err != nil {
		// The summary is useful even without the host memory figures.
		res = Resources{}
	}
	return s.HostFor(guests, res), nil
}

// Resources is what only the NAS itself knows: the host's free memory.
type Resources struct {
	Name       string `json:"name"`
	FreeRAMMB  int64  `json:"free_ram_mb"`
	TotalRAMMB int64  `json:"total_ram_mb"`
}

// Resources asks the NAS about the virtualisation host.
//
// The call is slow (about half a second) and only the free memory changes in
// it — which is why callers keep it in a cache.
func (s *Service) Resources(ctx context.Context) (Resources, error) {
	var out struct {
		Hosts []struct {
			HostName    string `json:"host_name"`
			FreeRAMSize int64  `json:"free_ram_size"`
			RAMSize     int64  `json:"ram_size"`
		} `json:"hosts"`
	}
	if err := s.c.Call(ctx, apiHost, "list", 1, nil, &out); err != nil {
		return Resources{}, err
	}
	if len(out.Hosts) == 0 {
		return Resources{}, nil
	}
	return Resources{
		Name:       out.Hosts[0].HostName,
		FreeRAMMB:  out.Hosts[0].FreeRAMSize,
		TotalRAMMB: out.Hosts[0].RAMSize,
	}, nil
}

// HostFor builds the summary from an already fetched machine list and host details.
//
// The list and the resources are passed in ready so they are not requested
// twice: the handler takes both from the cache.
func (s *Service) HostFor(guests []Guest, res Resources) Host {
	host := Host{
		TotalVMs:   len(guests),
		Name:       res.Name,
		FreeRAMMB:  res.FreeRAMMB,
		TotalRAMMB: res.TotalRAMMB,
	}
	for _, g := range guests {
		if g.Running {
			host.RunningVMs++
			host.UsedVCPU += g.VCPU
		}
	}

	return host
}

// PowerOn starts a machine.
func (s *Service) PowerOn(ctx context.Context, id string) error {
	return s.action(ctx, "poweron", id)
}

// Shutdown asks the guest system to power off.
//
// This is a soft stop: it needs the guest agent installed in the machine.
// Without it nothing happens — a limit of virtualisation itself, not ours.
func (s *Service) Shutdown(ctx context.Context, id string) error {
	return s.action(ctx, "shutdown", id)
}

func (s *Service) action(ctx context.Context, method, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("no machine given")
	}
	return s.c.Call(ctx, apiAction, method, 1, map[string]any{"guest_id": id}, nil)
}
