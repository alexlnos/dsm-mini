package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// apiClient — та часть клиента DSM, которая нужна этому пакету.
// Интерфейс, а не *dsm.Client, чтобы реализации можно было тестировать
// без живого NAS.
type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
	CallUpload(ctx context.Context, api, method string, version int,
		fields map[string]string, file dsm.UploadFile, out any) error
	HasAPI(ctx context.Context, api string) bool
	APIMaxVersion(ctx context.Context, api string) int
}

// Station — операции над Download Station, одинаковые для обоих поколений API.
type Station interface {
	// List возвращает все задачи со сведениями о ходе загрузки.
	List(ctx context.Context) ([]Task, error)
	// Pause останавливает задачи.
	Pause(ctx context.Context, ids []string) error
	// Resume возобновляет задачи.
	Resume(ctx context.Context, ids []string) error
	// Delete удаляет задачи. forceComplete — удалить, считая незавершённое
	// законченным (файлы на диске остаются).
	Delete(ctx context.Context, ids []string, forceComplete bool) error
	// Create ставит новую задачу.
	Create(ctx context.Context, req CreateRequest) error
	// Stats возвращает суммарные скорости.
	Stats(ctx context.Context) (Stats, error)
	// Volumes перечисляет тома NAS и место на них.
	Volumes(ctx context.Context) ([]Volume, error)
	// DefaultDestination — папка назначения по умолчанию.
	DefaultDestination(ctx context.Context) (string, error)
	// Generation — какое поколение API используется: "v2" или "legacy".
	// Нужно для диагностики в логе и в /healthz.
	Generation() string

	// Files перечисляет файлы внутри раздачи. Работает только у активной
	// задачи — иначе ErrNotActive.
	Files(ctx context.Context, taskID string) ([]File, error)
	// SetFile меняет приоритет файла и признак «качать ли его».
	SetFile(ctx context.Context, taskID string, indexes []int, priority FilePriority, wanted *bool) error
	// Trackers перечисляет трекеры раздачи.
	Trackers(ctx context.Context, taskID string) ([]Tracker, error)
	// SetDestination переносит задачи в другую папку.
	SetDestination(ctx context.Context, taskIDs []string, destination string) error
	// SetPriority меняет приоритет задач в очереди.
	SetPriority(ctx context.Context, taskIDs []string, priority FilePriority) error
}

const (
	apiTaskV2      = "SYNO.DownloadStation2.Task"
	apiFileV2      = "SYNO.DownloadStation2.Task.BT.File"
	apiTrackerV2   = "SYNO.DownloadStation2.Task.BT.Tracker"
	apiSettingsV2  = "SYNO.DownloadStation2.Settings.Global"
	apiLocationV2  = "SYNO.DownloadStation2.Settings.Location"
	apiStatisticV2 = "SYNO.DownloadStation2.Task.Statistic"
	apiTaskLegacy  = "SYNO.DownloadStation.Task"
	apiInfoLegacy  = "SYNO.DownloadStation.Info"
	apiStatsLegacy = "SYNO.DownloadStation.Statistic"
)

// Generation — какое поколение API использовать.
type Generation string

const (
	// GenerationAuto — выбрать лучшее из доступного на этом NAS.
	GenerationAuto Generation = "auto"
	// GenerationV2 — принудительно SYNO.DownloadStation2 (DSM 7).
	GenerationV2 Generation = "v2"
	// GenerationLegacy — принудительно SYNO.DownloadStation (DSM 6 и старше).
	GenerationLegacy Generation = "legacy"
)

// NewWithGeneration создаёт реализацию заданного поколения.
// Нужно для отладки и для тестов: на DSM 7 живы оба API сразу.
func NewWithGeneration(ctx context.Context, c apiClient, gen Generation) (Station, error) {
	switch gen {
	case GenerationV2:
		if !c.HasAPI(ctx, apiTaskV2) {
			return nil, fmt.Errorf("на этом NAS нет %s", apiTaskV2)
		}
		return &stationV2{c: c}, nil
	case GenerationLegacy:
		if !c.HasAPI(ctx, apiTaskLegacy) {
			return nil, fmt.Errorf("на этом NAS нет %s", apiTaskLegacy)
		}
		return &stationLegacy{c: c}, nil
	default:
		return New(ctx, c)
	}
}

// New выбирает реализацию под конкретный NAS.
//
// Предпочитается DownloadStation2 (DSM 7): у него богаче сведения о задачах.
// Если его нет — работаем через легаси-API, который есть и в DSM 6.
func New(ctx context.Context, c apiClient) (Station, error) {
	if c.HasAPI(ctx, apiTaskV2) {
		return &stationV2{c: c}, nil
	}
	if c.HasAPI(ctx, apiTaskLegacy) {
		return &stationLegacy{c: c}, nil
	}
	return nil, fmt.Errorf("на этом NAS не найден пакет Download Station: " +
		"установите его в Центре пакетов и убедитесь, что у учётной записи есть к нему доступ")
}

// actionResult принимает результат пакетного действия в любой из форм,
// которыми отвечает Download Station.
//
// Формы, снятые с живого DSM 7.2.2 (в документации Synology их нет):
//
//	v2 resume/delete → {"failed_task": [{"id", "error"}]}   объект
//	legacy delete    → [{"id", "error"}]                     голый массив
//
// Третью форму — отказ через error.errors.failed_task при success:false —
// разбирает сам клиент DSM и отдаёт готовым APIError.
type actionResult struct {
	Failed []dsm.ItemFailure
}

func (a *actionResult) UnmarshalJSON(b []byte) error {
	var arr []dsm.ItemFailure
	if err := json.Unmarshal(b, &arr); err == nil {
		a.Failed = arr
		return nil
	}
	var obj struct {
		FailedTask []dsm.ItemFailure `json:"failed_task"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		a.Failed = obj.FailedTask
		return nil
	}
	// Незнакомая форма ответа — не повод ронять действие, которое,
	// возможно, выполнено. Молчаливых отказов это не прячет: они приходят
	// в двух формах выше.
	return nil
}

// failuresToError превращает отказы по отдельным задачам в ошибку.
//
// Без этого действие выглядит выполненным: Download Station отвечает
// success даже тогда, когда ни одна задача его не приняла.
func failuresToError(api, method string, res actionResult) error {
	var failed []dsm.ItemFailure
	for _, it := range res.Failed {
		if it.Code != 0 {
			failed = append(failed, it)
		}
	}
	if len(failed) == 0 {
		return nil
	}
	return &dsm.APIError{API: api, Method: method, Failed: failed}
}
