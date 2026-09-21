//go:build integration

package downloadstation

import (
	"context"
	"strings"
	"testing"
)

// TestActionOnMissingTaskFails covers a trap in both API generations:
// Download Station answers "success" and hides the per-task refusal inside the
// results array. An action on a task that does not exist must reach the
// caller as an error.
//
// The test changes no NAS state: task dbid_999999 does not exist.
func TestActionOnMissingTaskFails(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	const missing = "dbid_999999"

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("creation: %v", err)
			}

			for _, tc := range []struct {
				name string
				call func() error
			}{
				{"pause", func() error { return st.Pause(ctx, []string{missing}) }},
				{"resume", func() error { return st.Resume(ctx, []string{missing}) }},
				{"delete", func() error { return st.Delete(ctx, []string{missing}, false) }},
			} {
				err := tc.call()
				if err == nil {
					t.Errorf("%s on a missing task returned success — the refusal was lost", tc.name)
					continue
				}
				t.Logf("%s → %v", tc.name, err)
				if strings.Contains(err.Error(), "code 0") {
					t.Errorf("%s: an error with no code: %v", tc.name, err)
				}
			}
		})
	}
}

// TestEmptyIDsRejected: an empty list must not reach the NAS.
func TestEmptyIDsRejected(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("sign in to DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("creation: %v", err)
	}
	if err := st.Pause(ctx, nil); err == nil {
		t.Error("pausing with an empty list must be refused before the request to the NAS")
	}
}
