//go:build integration

package system_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/alexlnos/dsm-mini/internal/dsm"
	"github.com/alexlnos/dsm-mini/internal/dsm/containers"
	"github.com/alexlnos/dsm-mini/internal/dsm/storage"
	"github.com/alexlnos/dsm-mini/internal/dsm/system"
	"github.com/alexlnos/dsm-mini/internal/dsm/vmm"
)

func newClient(t *testing.T) *dsm.Client {
	t.Helper()
	url, user, pass := os.Getenv("DSM_URL"), os.Getenv("DSM_USER"), os.Getenv("DSM_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("DSM_URL / DSM_USER / DSM_PASSWORD are not set")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	c := dsm.New(dsm.Options{BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 30 * time.Second})
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })
	return c
}

// TestSystemOverview reads the NAS state. It changes nothing.
func TestSystemOverview(t *testing.T) {
	ctx := context.Background()
	svc := system.New(newClient(t))

	info, err := svc.Info(ctx)
	if err != nil {
		t.Fatalf("system details: %v", err)
	}
	t.Logf("model %s, firmware %s, %d cores, %d MB memory, uptime %d h",
		info.Model, info.Firmware, info.CPUCores, info.RAMMB, info.UptimeSeconds/3600)
	if info.Model == "" || info.CPUCores == 0 {
		t.Error("the system details are empty")
	}
	if info.UptimeSeconds == 0 {
		t.Error("the uptime did not parse")
	}

	usage, err := svc.Usage(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Logf("CPU %d%%, memory %d%% of %d MB, network ↓%d ↑%d B/s",
		usage.CPUPercent, usage.MemoryPercent, usage.MemoryTotalMB,
		usage.NetworkRx, usage.NetworkTx)
	if usage.MemoryPercent <= 0 || usage.MemoryPercent > 100 {
		t.Errorf("the used memory share is out of range: %d", usage.MemoryPercent)
	}

	packages, err := svc.Packages(ctx)
	if err != nil {
		t.Fatalf("packages: %v", err)
	}
	t.Logf("packages: %d", len(packages))
	for _, id := range []string{"DownloadStation", "FileStation", "Virtualization", "ContainerManager"} {
		p, ok := svc.PackageState(ctx, id)
		if !ok {
			t.Logf("  %s: not installed", id)
			continue
		}
		t.Logf("  %s: %s, running=%v", p.Name, p.Version, p.Running)
	}
}

// TestSystemLog reads the log and checks the filtering of problem records.
func TestSystemLog(t *testing.T) {
	ctx := context.Background()
	svc := system.New(newClient(t))

	entries, err := svc.Log(ctx, 10, false)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("the log is empty")
	}
	t.Logf("latest records: %d", len(entries))
	for i, e := range entries {
		if i >= 4 {
			break
		}
		t.Logf("  [%s] %s %s", e.Level, e.Time.Format("02.01 15:04"), trim(e.Message, 60))
	}
	if entries[0].Time.IsZero() {
		t.Error("the record time did not parse")
	}

	// Filtering by level on the NAS side does not work, we filter here.
	problems, err := svc.Log(ctx, 10, true)
	if err != nil {
		t.Fatalf("problem log: %v", err)
	}
	t.Logf("problem records: %d", len(problems))
	for _, e := range problems {
		if e.Level != "error" && e.Level != "warn" {
			t.Errorf("a record of level %q got into the selection", e.Level)
		}
	}
}

// TestStorageOverview reads disks, pools and volumes.
func TestStorageOverview(t *testing.T) {
	ctx := context.Background()
	svc := storage.New(newClient(t))

	overview, err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Logf("%d disks, %d pools, %d volumes, all healthy=%v",
		len(overview.Disks), len(overview.Pools), len(overview.Volumes), overview.Healthy)

	if len(overview.Disks) == 0 {
		t.Fatal("the disk list is empty")
	}
	for _, d := range overview.Disks {
		t.Logf("  %s %s %d GB %d°C %s (%s)",
			d.ID, trim(d.Model, 22), d.Size/1e9, d.Temp, d.Status, d.Role)
		if d.Size == 0 {
			t.Errorf("the size of disk %s did not parse", d.ID)
		}
	}
	for _, v := range overview.Volumes {
		if v.Total == 0 {
			t.Errorf("the size of volume %s did not parse", v.ID)
			continue
		}
		t.Logf("  volume %s: %.2f of %.2f TB (%d%%)", v.ID,
			float64(v.Used)/1e12, float64(v.Total)/1e12, v.Used*100/v.Total)
	}
}

// TestVirtualMachines reads the machine list and the resource summary.
func TestVirtualMachines(t *testing.T) {
	ctx := context.Background()
	svc := vmm.New(newClient(t))

	guests, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("machine list: %v", err)
	}
	t.Logf("machines: %d", len(guests))
	for _, g := range guests {
		t.Logf("  %-18s %-9s %d vCPU, %d MB, disk %d MB, autostart=%v",
			g.Name, g.Status, g.VCPU, g.RAMMB, g.DiskMB, g.Autorun)
		if g.ID == "" || g.Name == "" {
			t.Error("a machine has no name or id")
		}
	}

	host, err := svc.Host(ctx)
	if err != nil {
		t.Fatalf("host summary: %v", err)
	}
	t.Logf("%d of %d running, %d vCPU busy, %d MB free",
		host.RunningVMs, host.TotalVMs, host.UsedVCPU, host.FreeRAMMB)
}

// TestContainers reads the container list. An empty list is fine.
func TestContainers(t *testing.T) {
	ctx := context.Background()
	svc := containers.New(newClient(t))

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("containers: %v", err)
	}
	t.Logf("containers: %d", len(list))
	for _, c := range list {
		t.Logf("  %s (%s) %s", c.Name, c.Image, c.Status)
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
