package downloadstation

import (
	"context"
	"fmt"
	"strings"
)

// Below are the capabilities the legacy API does not have.
//
// Files, trackers and priorities appeared only in SYNO.DownloadStation2, and
// substituting something similar makes no sense: it is better to say plainly
// that they are unavailable on this NAS than to show an empty list.
var errLegacyUnsupported = fmt.Errorf(
	"this feature needs Download Station with the DownloadStation2 API (DSM 7 and newer)")

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

// SetDestination does exist in the legacy API: the edit method takes
// destination from version 2 and is documented by Synology.
func (s *stationLegacy) SetDestination(ctx context.Context, taskIDs []string, destination string) error {
	if len(taskIDs) == 0 {
		return fmt.Errorf("no tasks given")
	}
	destination = strings.Trim(strings.TrimSpace(destination), "/")
	if destination == "" {
		return fmt.Errorf("no folder given")
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
