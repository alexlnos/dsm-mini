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
		t.Skip("не заданы DSM_URL / DSM_USER / DSM_PASSWORD")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("DSM_INSECURE_TLS"))
	c := dsm.New(dsm.Options{BaseURL: url, User: user, Password: pass,
		InsecureTLS: insecure, Timeout: 30 * time.Second})
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })
	return c
}

// TestSystemOverview читает состояние NAS. Ничего не меняет.
func TestSystemOverview(t *testing.T) {
	ctx := context.Background()
	svc := system.New(newClient(t))

	info, err := svc.Info(ctx)
	if err != nil {
		t.Fatalf("сведения о системе: %v", err)
	}
	t.Logf("модель %s, прошивка %s, ядер %d, память %d МБ, аптайм %d ч",
		info.Model, info.Firmware, info.CPUCores, info.RAMMB, info.UptimeSeconds/3600)
	if info.Model == "" || info.CPUCores == 0 {
		t.Error("сведения о системе пусты")
	}
	if info.UptimeSeconds == 0 {
		t.Error("аптайм не разобрался")
	}

	usage, err := svc.Usage(ctx)
	if err != nil {
		t.Fatalf("загрузка: %v", err)
	}
	t.Logf("CPU %d%%, память %d%% из %d МБ, сеть ↓%d ↑%d Б/с",
		usage.CPUPercent, usage.MemoryPercent, usage.MemoryTotalMB,
		usage.NetworkRx, usage.NetworkTx)
	if usage.MemoryPercent <= 0 || usage.MemoryPercent > 100 {
		t.Errorf("доля занятой памяти вне диапазона: %d", usage.MemoryPercent)
	}

	packages, err := svc.Packages(ctx)
	if err != nil {
		t.Fatalf("пакеты: %v", err)
	}
	t.Logf("пакетов: %d", len(packages))
	for _, id := range []string{"DownloadStation", "FileStation", "Virtualization", "ContainerManager"} {
		p, ok := svc.PackageState(ctx, id)
		if !ok {
			t.Logf("  %s: не установлен", id)
			continue
		}
		t.Logf("  %s: %s, работает=%v", p.Name, p.Version, p.Running)
	}
}

// TestSystemLog читает журнал и проверяет отбор проблемных записей.
func TestSystemLog(t *testing.T) {
	ctx := context.Background()
	svc := system.New(newClient(t))

	entries, err := svc.Log(ctx, 10, false)
	if err != nil {
		t.Fatalf("журнал: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("журнал пуст")
	}
	t.Logf("последних записей: %d", len(entries))
	for i, e := range entries {
		if i >= 4 {
			break
		}
		t.Logf("  [%s] %s %s", e.Level, e.Time.Format("02.01 15:04"), trim(e.Message, 60))
	}
	if entries[0].Time.IsZero() {
		t.Error("время записи не разобралось")
	}

	// Фильтр по уровню на стороне NAS не работает, отбор делается у нас.
	problems, err := svc.Log(ctx, 10, true)
	if err != nil {
		t.Fatalf("журнал проблем: %v", err)
	}
	t.Logf("проблемных записей: %d", len(problems))
	for _, e := range problems {
		if e.Level != "error" && e.Level != "warn" {
			t.Errorf("в отбор попала запись уровня %q", e.Level)
		}
	}
}

// TestStorageOverview читает диски, пулы и тома.
func TestStorageOverview(t *testing.T) {
	ctx := context.Background()
	svc := storage.New(newClient(t))

	overview, err := svc.Load(ctx)
	if err != nil {
		t.Fatalf("хранилище: %v", err)
	}
	t.Logf("дисков %d, пулов %d, томов %d, всё исправно=%v",
		len(overview.Disks), len(overview.Pools), len(overview.Volumes), overview.Healthy)

	if len(overview.Disks) == 0 {
		t.Fatal("список дисков пуст")
	}
	for _, d := range overview.Disks {
		t.Logf("  %s %s %d ГБ %d°C %s (%s)",
			d.ID, trim(d.Model, 22), d.Size/1e9, d.Temp, d.Status, d.Role)
		if d.Size == 0 {
			t.Errorf("у диска %s не разобрался размер", d.ID)
		}
	}
	for _, v := range overview.Volumes {
		if v.Total == 0 {
			t.Errorf("у тома %s не разобрался размер", v.ID)
			continue
		}
		t.Logf("  том %s: %.2f из %.2f ТБ (%d%%)", v.ID,
			float64(v.Used)/1e12, float64(v.Total)/1e12, v.Used*100/v.Total)
	}
}

// TestVirtualMachines читает список машин и сводку по ресурсам.
func TestVirtualMachines(t *testing.T) {
	ctx := context.Background()
	svc := vmm.New(newClient(t))

	guests, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("список машин: %v", err)
	}
	t.Logf("машин: %d", len(guests))
	for _, g := range guests {
		t.Logf("  %-18s %-9s %d vCPU, %d МБ, диск %d МБ, автозапуск=%v",
			g.Name, g.Status, g.VCPU, g.RAMMB, g.DiskMB, g.Autorun)
		if g.ID == "" || g.Name == "" {
			t.Error("у машины нет имени или идентификатора")
		}
	}

	host, err := svc.Host(ctx)
	if err != nil {
		t.Fatalf("сводка хоста: %v", err)
	}
	t.Logf("работает %d из %d, занято %d vCPU, свободно %d МБ",
		host.RunningVMs, host.TotalVMs, host.UsedVCPU, host.FreeRAMMB)
}

// TestContainers читает список контейнеров. Пустой список — нормально.
func TestContainers(t *testing.T) {
	ctx := context.Background()
	svc := containers.New(newClient(t))

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("контейнеры: %v", err)
	}
	t.Logf("контейнеров: %d", len(list))
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
