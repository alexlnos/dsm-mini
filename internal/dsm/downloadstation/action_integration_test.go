//go:build integration

package downloadstation

import (
	"context"
	"strings"
	"testing"
)

// TestActionOnMissingTaskFails закрывает ловушку обоих поколений API:
// Download Station отвечает «успех», а отказ по конкретной задаче прячет
// в массиве результатов. Действие над несуществующей задачей обязано
// дойти до вызывающего как ошибка.
//
// Состояние NAS тест не меняет: задачи dbid_999999 не существует.
func TestActionOnMissingTaskFails(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	const missing = "dbid_999999"

	for _, gen := range []Generation{GenerationV2, GenerationLegacy} {
		t.Run(string(gen), func(t *testing.T) {
			st, err := NewWithGeneration(ctx, c, gen)
			if err != nil {
				t.Fatalf("создание: %v", err)
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
					t.Errorf("%s над несуществующей задачей вернул успех — отказ потерян", tc.name)
					continue
				}
				t.Logf("%s → %v", tc.name, err)
				if strings.Contains(err.Error(), "код 0") {
					t.Errorf("%s: ошибка без кода: %v", tc.name, err)
				}
			}
		})
	}
}

// TestEmptyIDsRejected: пустой список не должен уходить на NAS.
func TestEmptyIDsRejected(t *testing.T) {
	ctx := context.Background()
	c := newClient(t)
	if err := c.Login(ctx); err != nil {
		t.Fatalf("вход в DSM: %v", err)
	}
	t.Cleanup(func() { _ = c.Logout(context.Background()) })

	st, err := New(ctx, c)
	if err != nil {
		t.Fatalf("создание: %v", err)
	}
	if err := st.Pause(ctx, nil); err == nil {
		t.Error("пауза с пустым списком должна отклоняться до запроса к NAS")
	}
}
