package dsm

import (
	"errors"
	"fmt"
	"strings"
)

// APIError is an error DSM returned in the "error" field of a successful HTTP response.
//
// DSM almost always answers HTTP 200 and puts the real outcome into the JSON,
// so status codes cannot be relied upon.
type APIError struct {
	Code   int
	API    string
	Method string
	// Failed lists the items of a batch operation that did not go through.
	Failed []ItemFailure
}

func (e *APIError) Error() string {
	if len(e.Failed) > 0 {
		parts := make([]string, 0, len(e.Failed))
		for _, f := range e.Failed {
			parts = append(parts, fmt.Sprintf("%s (%s)", f.ID, itemErrorText(f.Code)))
		}
		return fmt.Sprintf("%s.%s: failed for tasks: %s",
			e.API, e.Method, strings.Join(parts, ", "))
	}
	if msg, ok := errorText[e.Code]; ok {
		return fmt.Sprintf("%s.%s: %s (code %d)", e.API, e.Method, msg, e.Code)
	}
	return fmt.Sprintf("%s.%s: DSM error with code %d", e.API, e.Method, e.Code)
}

// needsRelogin reports that the error is cured by logging in again.
func (e *APIError) needsRelogin() bool {
	switch e.Code {
	case 105, 106, 107, 119:
		return true
	}
	return false
}

// ErrAuth is returned when the login failed: wrong password, a disabled
// account or a demand for a 2FA code. Repeating such a request is pointless.
var ErrAuth = errors.New("authentication with DSM failed")

// AuthError is a refused login together with DSM's code, so that whoever
// shows it to a person can say which of the refusals it was. It matches
// ErrAuth under errors.Is.
type AuthError struct {
	Code int
}

func (e *AuthError) Error() string {
	msg, ok := authErrorText[e.Code]
	if !ok {
		msg = fmt.Sprintf("code %d", e.Code)
	}
	return ErrAuth.Error() + ": " + msg
}

func (e *AuthError) Is(target error) bool { return target == ErrAuth }

// errorText holds the common DSM codes plus login and Download Station ones.
//
// Synology's 400+ range overlaps: 403 during login means "2FA code needed",
// while in Download Station it means "destination folder does not exist". We
// tell them apart when building the error, not here.
var errorText = map[int]string{
	100: "unknown error",
	101: "invalid parameter",
	102: "no such API on this NAS",
	103: "the API has no such method",
	104: "API version is not supported",
	105: "the account lacks permission",
	106: "session expired",
	107: "session interrupted by another login",
	119: "session not found (SID is invalid)",
}

var authErrorText = map[int]string{
	400: "wrong user name or password",
	401: "the account is disabled",
	402: "access denied",
	403: "a two-factor code is required — create a user without 2FA",
	404: "wrong two-factor code",
	406: "email confirmation is required",
	407: "the address is blocked",
	408: "the password has expired",
	409: "the password is expired and must be changed",
	410: "the password is expired",
	411: "the account is locked",
}

// itemErrorText decodes the failure code of an individual task.
func itemErrorText(code int) string {
	if msg, ok := DownloadStationErrorText[code]; ok {
		return msg
	}
	return fmt.Sprintf("code %d", code)
}

// DownloadStationErrorText translates codes specific to Download Station.
//
// Codes 400-408 come from Synology's official documentation (Download Station
// Official API). Code 544 is missing there but arrives in practice when an
// action is addressed to a task that does not exist.
var DownloadStationErrorText = map[int]string{
	400: "file upload failed",
	401: "the task limit has been reached",
	402: "the destination folder is not available",
	403: "the destination folder does not exist",
	404: "no task with this id",
	405: "the action is not allowed for this task",
	406: "no default destination folder is set",
	407: "could not set the destination folder",
	408: "the file does not exist",
	544: "the task is gone or the action does not apply to it",
	// The codes below are missing from the documentation but arrive in practice.
	1203: "the destination folder was not found on the NAS",
	1913: "details are available only while the task is downloading or seeding",
}
