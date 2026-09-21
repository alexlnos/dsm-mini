// Package storage — диски, пулы и тома NAS.
package storage

import (
	"context"
	"strconv"
	"strings"
)

type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
}

const apiStorage = "SYNO.Storage.CGI.Storage"

// Service читает состояние хранилища.
type Service struct{ c apiClient }

// New создаёт службу.
func New(c apiClient) *Service { return &Service{c: c} }

// Disk — физический диск.
type Disk struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Vendor string `json:"vendor,omitempty"`
	Size   int64  `json:"size"`
	// Temp — температура в градусах Цельсия.
	Temp int `json:"temp"`
	// Status и Smart приходят словами: normal, warning, critical.
	Status string `json:"status"`
	Smart  string `json:"smart"`
	// Slot — номер отсека; у дисков M.2 нумерация своя.
	Slot int `json:"slot"`
	// IsSSD отличает M.2 и SSD от обычных дисков.
	IsSSD bool `json:"is_ssd"`
	// Role — для чего диск используется: "pool", "cache" или "free".
	//
	// Здесь код, а не готовая фраза: подпись переводится на стороне
	// интерфейса, иначе англичанин увидел бы русское «пул reuse_1».
	Role string `json:"role,omitempty"`
	// Pool — имя пула, если диск в пуле.
	Pool string `json:"pool,omitempty"`
	// Healthy сведён из состояния диска и SMART.
	Healthy bool `json:"healthy"`
}

// Pool — группа дисков.
type Pool struct {
	ID     string   `json:"id"`
	RAID   string   `json:"raid,omitempty"`
	Status string   `json:"status"`
	Total  int64    `json:"total"`
	Used   int64    `json:"used"`
	Disks  []string `json:"disks"`
}

// Volume — том, на котором лежат общие папки.
type Volume struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	FSType string `json:"fs_type,omitempty"`
	Status string `json:"status"`
	Total  int64  `json:"total"`
	Used   int64  `json:"used"`
}

// Overview — всё хранилище одним ответом.
type Overview struct {
	Disks   []Disk   `json:"disks"`
	Pools   []Pool   `json:"pools"`
	Volumes []Volume `json:"volumes"`
	// Healthy — всё ли в порядке: хватает одного сбойного диска, чтобы стало false.
	Healthy bool `json:"healthy"`
}

// Load читает состояние хранилища целиком.
//
// DSM отдаёт диски, пулы и тома одним вызовом, поэтому дробить его не на что.
func (s *Service) Load(ctx context.Context) (Overview, error) {
	var out struct {
		Disks []struct {
			ID         string `json:"id"`
			Model      string `json:"model"`
			Vendor     string `json:"vendor"`
			SizeTotal  string `json:"size_total"`
			Temp       int    `json:"temp"`
			Status     string `json:"status"`
			Smart      string `json:"smart_status"`
			SlotID     int    `json:"slot_id"`
			IsSSD      bool   `json:"isSsd"`
			TrayStatus string `json:"tray_status"`
			UsedBy     string `json:"used_by"`
		} `json:"disks"`
		Pools []struct {
			ID       string   `json:"id"`
			RAIDType string   `json:"raidType"`
			Status   string   `json:"status"`
			Disks    []string `json:"disks"`
			Size     struct {
				Total string `json:"total"`
				Used  string `json:"used"`
			} `json:"size"`
		} `json:"storagePools"`
		Volumes []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			FSType      string `json:"fs_type"`
			Status      string `json:"status"`
			Size        struct {
				Total string `json:"total"`
				Used  string `json:"used"`
			} `json:"size"`
		} `json:"volumes"`
	}
	if err := s.c.Call(ctx, apiStorage, "load_info", 1, nil, &out); err != nil {
		return Overview{}, err
	}

	overview := Overview{Healthy: true}

	for _, d := range out.Disks {
		healthy := isNormal(d.Status) && isNormal(d.Smart)
		if !healthy {
			overview.Healthy = false
		}
		overview.Disks = append(overview.Disks, Disk{
			ID:      d.ID,
			Model:   strings.TrimSpace(d.Model),
			Vendor:  strings.TrimSpace(d.Vendor),
			Size:    parseSize(d.SizeTotal),
			Temp:    d.Temp,
			Status:  d.Status,
			Smart:   d.Smart,
			Slot:    d.SlotID,
			IsSSD:   d.IsSSD,
			Role:    role(d.UsedBy, d.TrayStatus),
			Pool:    d.UsedBy,
			Healthy: healthy,
		})
	}

	for _, p := range out.Pools {
		if !isNormal(p.Status) {
			overview.Healthy = false
		}
		overview.Pools = append(overview.Pools, Pool{
			ID:     p.ID,
			RAID:   p.RAIDType,
			Status: p.Status,
			Total:  parseSize(p.Size.Total),
			Used:   parseSize(p.Size.Used),
			Disks:  p.Disks,
		})
	}

	for _, v := range out.Volumes {
		if !isNormal(v.Status) {
			overview.Healthy = false
		}
		overview.Volumes = append(overview.Volumes, Volume{
			ID:     v.ID,
			Name:   v.DisplayName,
			FSType: v.FSType,
			Status: v.Status,
			Total:  parseSize(v.Size.Total),
			Used:   parseSize(v.Size.Used),
		})
	}
	return overview, nil
}

// role называет назначение диска кодом, который интерфейс переведёт сам.
func role(usedBy, tray string) string {
	switch {
	case strings.Contains(strings.ToLower(tray), "cache"):
		return "cache"
	case usedBy != "":
		return "pool"
	default:
		return "free"
	}
}

func isNormal(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return s == "" || s == "normal" || s == "healthy"
}

// parseSize читает размер: DSM отдаёт его строкой, иногда пустой.
func parseSize(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
