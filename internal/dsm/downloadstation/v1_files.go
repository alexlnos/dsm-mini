package downloadstation

import (
	"context"
	"fmt"
	"strings"
)

// Ниже — возможности, которых у легаси-API нет.
//
// Файлы, трекеры и приоритеты появились только в SYNO.DownloadStation2, и
// подменять их чем-то похожим смысла нет: лучше честно сказать, что на этом
// NAS они недоступны, чем показать пустой список.
var errLegacyUnsupported = fmt.Errorf(
	"эта возможность требует Download Station с API DownloadStation2 (DSM 7 и новее)")

func (s *stationLegacy) Files(context.Context, string) ([]File, error) {
	return nil, errLegacyUnsupported
}

func (s *stationLegacy) SetFile(context.Context, string, []int, FilePriority, *bool) error {
	return errLegacyUnsupported
}

func (s *stationLegacy) Trackers(context.Context, string) ([]Tracker, error) {
	return nil, errLegacyUnsupported
}

func (s *stationLegacy) SetPriority(context.Context, []string, FilePriority) error {
	return errLegacyUnsupported
}

// SetDestination у легаси-API есть: метод edit принимает destination с
// версии 2 и документирован Synology.
func (s *stationLegacy) SetDestination(ctx context.Context, taskIDs []string, destination string) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("не указано ни одной задачи")
	}
	destination = strings.Trim(strings.TrimSpace(destination), "/")
	if destination == "" {
		return fmt.Errorf("не указана папка")
	}
	var res actionResult
	if err := s.c.Call(ctx, apiTaskLegacy, "edit", 2, map[string]any{
		"id":          strings.Join(taskIDs, ","),
		"destination": destination,
	}, &res); err != nil {
		return err
	}
	return failuresToError(apiTaskLegacy, "edit", res)
}
