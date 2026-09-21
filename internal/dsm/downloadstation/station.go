package downloadstation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alexlnos/dsm-mini/internal/dsm"
)

// apiClient is the part of the DSM client this package needs.
// An interface rather than *dsm.Client so that implementations can be tested
// without a live NAS.
type apiClient interface {
	Call(ctx context.Context, api, method string, version int, params map[string]any, out any) error
	CallUpload(ctx context.Context, api, method string, version int,
		fields map[string]string, file dsm.UploadFile, out any) error
	HasAPI(ctx context.Context, api string) bool
	APIMaxVersion(ctx context.Context, api string) int
}

// Station covers Download Station operations shared by both API generations.
type Station interface {
	// List returns every task together with its progress.
	List(ctx context.Context) ([]Task, error)
	// Pause stops tasks.
	Pause(ctx context.Context, ids []string) error
	// Resume restarts tasks.
	Resume(ctx context.Context, ids []string) error
	// Delete removes tasks. forceComplete means delete while treating the
	// unfinished part as done (files stay on disk).
	Delete(ctx context.Context, ids []string, forceComplete bool) error
	// Create queues a new task.
	Create(ctx context.Context, req CreateRequest) error
	// Stats returns the combined speeds.
	Stats(ctx context.Context) (Stats, error)
	// Volumes lists the NAS volumes and the space on them.
	Volumes(ctx context.Context) ([]Volume, error)
	// DefaultDestination is the default destination folder.
	DefaultDestination(ctx context.Context) (string, error)
	// Generation tells which API generation is in use: "v2" or "legacy".
	// Needed for diagnostics in the log and in /healthz.
	Generation() string

	// Files lists the files inside a torrent. Works only for an active task —
	// otherwise ErrNotActive.
	Files(ctx context.Context, taskID string) ([]File, error)
	// SetFile changes a file's priority and whether to download it at all.
	SetFile(ctx context.Context, taskID string, indexes []int, priority FilePriority, wanted *bool) error
	// Trackers lists the torrent's trackers.
	Trackers(ctx context.Context, taskID string) ([]Tracker, error)
	// SetDestination moves tasks to another folder.
	SetDestination(ctx context.Context, taskIDs []string, destination string) error
	// SetPriority changes the queue priority of tasks.
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

// Generation is which API generation to use.
type Generation string

const (
	// GenerationAuto picks the best of what this NAS offers.
	GenerationAuto Generation = "auto"
	// GenerationV2 forces SYNO.DownloadStation2 (DSM 7).
	GenerationV2 Generation = "v2"
	// GenerationLegacy forces SYNO.DownloadStation (DSM 6 and older).
	GenerationLegacy Generation = "legacy"
)

// NewWithGeneration creates an implementation of the given generation.
// Needed for debugging and for tests: on DSM 7 both APIs are alive at once.
func NewWithGeneration(ctx context.Context, c apiClient, gen Generation) (Station, error) {
	switch gen {
	case GenerationV2:
		if !c.HasAPI(ctx, apiTaskV2) {
			return nil, fmt.Errorf("this NAS has no %s", apiTaskV2)
		}
		return &stationV2{c: c}, nil
	case GenerationLegacy:
		if !c.HasAPI(ctx, apiTaskLegacy) {
			return nil, fmt.Errorf("this NAS has no %s", apiTaskLegacy)
		}
		return &stationLegacy{c: c}, nil
	default:
		return New(ctx, c)
	}
}

// New picks the implementation for a particular NAS.
//
// DownloadStation2 (DSM 7) is preferred: it reports richer task details.
// When it is missing we work through the legacy API, present in DSM 6 too.
func New(ctx context.Context, c apiClient) (Station, error) {
	if c.HasAPI(ctx, apiTaskV2) {
		return &stationV2{c: c}, nil
	}
	if c.HasAPI(ctx, apiTaskLegacy) {
		return &stationLegacy{c: c}, nil
	}
	return nil, fmt.Errorf("the Download Station package was not found on this NAS: " +
		"install it from Package Center and make sure the account has access to it")
}

// actionResult accepts the result of a batch action in any of the shapes
// Download Station answers with.
//
// Shapes observed on a live DSM 7.2.2 (Synology documents none of them):
//
//	v2 resume/delete → {"failed_task": [{"id", "error"}]}   object
//	legacy delete    → [{"id", "error"}]                     bare array
//
// The third shape — a refusal through error.errors.failed_task with
// success:false — is handled by the DSM client itself and arrives as APIError.
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
	// An unfamiliar response shape is no reason to fail an action that may
	// well have gone through. It does not hide silent refusals either: those
	// come in the two shapes above.
	return nil
}

// failuresToError turns per-task refusals into an error.
//
// Without it the action looks successful: Download Station answers success
// even when not a single task accepted it.
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
