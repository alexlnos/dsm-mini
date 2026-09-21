// Package system — состояние NAS: загрузка, сведения о модели, пакеты и журнал.
package system

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

const (
	apiUtilization = "SYNO.Core.System.Utilization"
	apiSystem      = "SYNO.Core.System"
	apiPackage     = "SYNO.Core.Package"
	apiSyslog      = "SYNO.Core.SyslogClient.Log"
	apiFileStation = "SYNO.FileStation.Info"
)

// Service читает состояние NAS.
type Service struct{ c apiClient }

// New создаёт службу.
func New(c apiClient) *Service { return &Service{c: c} }

// Info — что за устройство и как давно работает.
type Info struct {
	Hostname  string `json:"hostname"`
	Model     string `json:"model"`
	Firmware  string `json:"firmware"`
	CPUVendor string `json:"cpu_vendor,omitempty"`
	CPUSeries string `json:"cpu_series,omitempty"`
	CPUCores  int    `json:"cpu_cores"`
	RAMMB     int64  `json:"ram_mb"`
	// UptimeSeconds — сколько NAS работает без перезагрузки.
	UptimeSeconds int64 `json:"uptime_seconds"`
}

// Usage — текущая загрузка.
type Usage struct {
	CPUPercent    int   `json:"cpu_percent"`
	MemoryPercent int   `json:"memory_percent"`
	MemoryTotalMB int64 `json:"memory_total_mb"`
	NetworkRx     int64 `json:"network_rx"`
	NetworkTx     int64 `json:"network_tx"`
}

// Package — установленный пакет и его состояние.
type Package struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

// LogEntry — запись системного журнала.
type LogEntry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
	Who     string    `json:"who,omitempty"`
}

// Info возвращает сведения об устройстве.
func (s *Service) Info(ctx context.Context) (Info, error) {
	var out struct {
		Hostname    string `json:"hostname"`
		Model       string `json:"model"`
		FirmwareVer string `json:"firmware_ver"`
		CPUVendor   string `json:"cpu_vendor"`
		CPUSeries   string `json:"cpu_series"`
		CPUCores    string `json:"cpu_cores"`
		RAMSize     int64  `json:"ram_size"`
		UpTime      string `json:"up_time"`
	}
	if err := s.c.Call(ctx, apiSystem, "info", 1, nil, &out); err != nil {
		return Info{}, err
	}
	cores, _ := strconv.Atoi(strings.TrimSpace(out.CPUCores))

	hostname := out.Hostname
	if hostname == "" {
		// SYNO.Core.System имя хоста не возвращает; у File Station оно есть.
		var fs struct {
			Hostname string `json:"hostname"`
		}
		if err := s.c.Call(ctx, apiFileStation, "get", 2, nil, &fs); err == nil {
			hostname = fs.Hostname
		}
	}

	return Info{
		Hostname:      hostname,
		Model:         out.Model,
		Firmware:      strings.TrimSpace(strings.TrimPrefix(out.FirmwareVer, "DSM")),
		CPUVendor:     out.CPUVendor,
		CPUSeries:     out.CPUSeries,
		CPUCores:      cores,
		RAMMB:         out.RAMSize,
		UptimeSeconds: parseUptime(out.UpTime),
	}, nil
}

// parseUptime разбирает строку вида "226:28:23" — часы, минуты, секунды.
func parseUptime(s string) int64 {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) != 3 {
		return 0
	}
	var total int64
	for i, mult := range []int64{3600, 60, 1} {
		v, err := strconv.ParseInt(parts[i], 10, 64)
		if err != nil {
			return 0
		}
		total += v * mult
	}
	return total
}

// Usage возвращает текущую загрузку процессора, памяти и сети.
func (s *Service) Usage(ctx context.Context) (Usage, error) {
	var out struct {
		CPU struct {
			// Загрузка приходит долями процента: 150 значит 1,5 %.
			UserLoad   int `json:"user_load"`
			SystemLoad int `json:"system_load"`
			OtherLoad  int `json:"other_load"`
		} `json:"cpu"`
		Memory struct {
			RealUsage int   `json:"real_usage"`
			TotalReal int64 `json:"total_real"`
		} `json:"memory"`
		Network []struct {
			Device string `json:"device"`
			Rx     int64  `json:"rx"`
			Tx     int64  `json:"tx"`
		} `json:"network"`
	}
	err := s.c.Call(ctx, apiUtilization, "get", 1, map[string]any{
		"resource": []string{"cpu", "memory", "network"},
	}, &out)
	if err != nil {
		return Usage{}, err
	}

	usage := Usage{
		CPUPercent:    out.CPU.UserLoad + out.CPU.SystemLoad + out.CPU.OtherLoad,
		MemoryPercent: out.Memory.RealUsage,
		MemoryTotalMB: out.Memory.TotalReal / 1024,
	}
	for _, n := range out.Network {
		if n.Device == "total" {
			usage.NetworkRx, usage.NetworkTx = n.Rx, n.Tx
			break
		}
	}
	return usage, nil
}

// Packages возвращает установленные пакеты с их состоянием.
func (s *Service) Packages(ctx context.Context) ([]Package, error) {
	var out struct {
		Packages []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Version    string `json:"version"`
			Additional struct {
				Status string `json:"status"`
			} `json:"additional"`
		} `json:"packages"`
	}
	err := s.c.Call(ctx, apiPackage, "list", 2, map[string]any{
		"additional": []string{"status"},
	}, &out)
	if err != nil {
		return nil, err
	}

	packages := make([]Package, 0, len(out.Packages))
	for _, p := range out.Packages {
		packages = append(packages, Package{
			ID:      p.ID,
			Name:    p.Name,
			Version: p.Version,
			Status:  p.Additional.Status,
			Running: p.Additional.Status == "running",
		})
	}
	return packages, nil
}

// PackageState сообщает, установлен ли пакет и работает ли он.
func (s *Service) PackageState(ctx context.Context, id string) (Package, bool) {
	packages, err := s.Packages(ctx)
	if err != nil {
		return Package{}, false
	}
	for _, p := range packages {
		if strings.EqualFold(p.ID, id) {
			return p, true
		}
	}
	return Package{}, false
}

// Log возвращает последние записи системного журнала.
//
// Фильтр по уровню на стороне NAS не работает: параметр принимается, но ответ
// не меняется. Поэтому отбираем нужное здесь, взяв запас записей.
func (s *Service) Log(ctx context.Context, limit int, onlyProblems bool) ([]LogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	fetch := limit
	if onlyProblems {
		// Ошибки редки: чтобы набрать нужное число, берём с запасом.
		fetch = limit * 10
		if fetch > 1000 {
			fetch = 1000
		}
	}

	var out struct {
		Items []struct {
			Time    string `json:"time"`
			Level   string `json:"level"`
			LogType string `json:"logtype"`
			Descr   string `json:"descr"`
			Who     string `json:"who"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := s.c.Call(ctx, apiSyslog, "list", 1, map[string]any{
		"limit": fetch, "offset": 0,
	}, &out); err != nil {
		return nil, err
	}

	entries := make([]LogEntry, 0, limit)
	for _, it := range out.Items {
		level := strings.ToLower(it.Level)
		if onlyProblems && level != "error" && level != "warn" {
			continue
		}
		entries = append(entries, LogEntry{
			Time:    parseLogTime(it.Time),
			Level:   level,
			Type:    it.LogType,
			Message: it.Descr,
			Who:     it.Who,
		})
		if len(entries) == limit {
			break
		}
	}
	return entries, nil
}

// parseLogTime разбирает время журнала: "2026/09/21 06:36:05" в местной зоне NAS.
func parseLogTime(s string) time.Time {
	t, err := time.ParseInLocation("2006/01/02 15:04:05", strings.TrimSpace(s), time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
