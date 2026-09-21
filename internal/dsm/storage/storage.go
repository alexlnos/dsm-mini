// Package storage covers the NAS disks, pools and volumes.
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

// Service reads the storage state.
type Service struct{ c apiClient }

// New creates the service.
func New(c apiClient) *Service { return &Service{c: c} }

// Disk is a physical disk.
type Disk struct {
	ID     string `json:"id"`
	Model  string `json:"model"`
	Vendor string `json:"vendor,omitempty"`
	Size   int64  `json:"size"`
	// Temp is the temperature in degrees Celsius.
	Temp int `json:"temp"`
	// Status and Smart arrive as words: normal, warning, critical.
	Status string `json:"status"`
	Smart  string `json:"smart"`
	// Slot is the bay number; M.2 disks are numbered separately.
	Slot int `json:"slot"`
	// IsSSD tells M.2 and SSD apart from spinning disks.
	IsSSD bool `json:"is_ssd"`
	// Role is what the disk is used for: "pool", "cache" or "free".
	//
	// A code rather than a ready phrase: the caption is translated on the
	// interface side, otherwise an English reader would see Russian.
	Role string `json:"role,omitempty"`
	// Pool is the pool name when the disk belongs to one.
	Pool string `json:"pool,omitempty"`
	// Healthy combines the disk state and SMART.
	Healthy bool `json:"healthy"`
}

// Pool is a group of disks.
type Pool struct {
	ID     string   `json:"id"`
	RAID   string   `json:"raid,omitempty"`
	Status string   `json:"status"`
	Total  int64    `json:"total"`
	Used   int64    `json:"used"`
	Disks  []string `json:"disks"`
}

// Volume is a volume that holds shared folders.
type Volume struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	FSType string `json:"fs_type,omitempty"`
	Status string `json:"status"`
	Total  int64  `json:"total"`
	Used   int64  `json:"used"`
}

// Overview is the whole storage in one answer.
type Overview struct {
	Disks   []Disk   `json:"disks"`
	Pools   []Pool   `json:"pools"`
	Volumes []Volume `json:"volumes"`
	// Healthy is whether all is well: one failing disk makes it false.
	Healthy bool `json:"healthy"`
}

// Load reads the storage state as a whole.
//
// DSM hands back disks, pools and volumes in one call, so there is nothing to split.
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

// role names the disk purpose with a code the interface translates itself.
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

// parseSize reads a size: DSM returns it as a string, sometimes empty.
func parseSize(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
