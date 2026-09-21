package i18n

import (
	"regexp"
	"sort"
	"testing"
)

var placeholder = regexp.MustCompile(`\{[a-z]+\}`)

// Полнота словарей: в Go нет типа, который потребовал бы все ключи, как это
// делает TypeScript на фронтенде, — поэтому требует тест.
func TestDictionariesMatchRussian(t *testing.T) {
	for lang, dict := range dicts {
		for key := range ru {
			if _, ok := dict[key]; !ok {
				t.Errorf("%s: нет ключа %q", lang, key)
			}
		}
		for key := range dict {
			if _, ok := ru[key]; !ok {
				t.Errorf("%s: лишний ключ %q — его нет в русском словаре", lang, key)
			}
		}
	}
}

// Подстановки: потерянный {folder} превращает сообщение в обрубок, и заметить
// это без проверки можно только на живом боте.
func TestPlaceholdersSurviveTranslation(t *testing.T) {
	for key, source := range ru {
		want := placeholder.FindAllString(source, -1)
		sort.Strings(want)
		for lang, dict := range dicts {
			got := placeholder.FindAllString(dict[key], -1)
			sort.Strings(got)
			if len(got) != len(want) {
				t.Errorf("%s, ключ %q: подстановки %v, а в русском %v", lang, key, got, want)
				continue
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("%s, ключ %q: подстановки %v, а в русском %v", lang, key, got, want)
					break
				}
			}
		}
	}
}

// Чужая письменность в словаре — почти всегда обрывок исходной строки,
// забытый при переводе. Кириллица в португальском тексте выглядит как опечатка
// и читается так же: «A ação не foi concluída».
func TestNoStrayCyrillic(t *testing.T) {
	cyrillic := regexp.MustCompile(`\p{Cyrillic}`)
	for lang, dict := range dicts {
		if lang == "ru" || lang == "uk" {
			continue
		}
		for key, text := range dict {
			if cyrillic.MatchString(text) {
				t.Errorf("%s, ключ %q: кириллица в тексте — %q", lang, key, text)
			}
		}
	}
}

func TestMatch(t *testing.T) {
	cases := map[string]Lang{
		"ru":      "ru",
		"RU":      "ru",
		"pt-BR":   "pt",
		"pt_PT":   "pt",
		"en-US":   "en",
		"ja":      Fallback,
		"":        Fallback,
		"unknown": Fallback,
	}
	for code, want := range cases {
		if got := Match(code); got != want {
			t.Errorf("Match(%q) = %q, ожидалось %q", code, got, want)
		}
	}
}

func TestSubstitution(t *testing.T) {
	got := T("ru", "bot.queuedTo", P{"folder": "Media/Music"})
	if want := "Поставил в очередь → Media/Music"; got != want {
		t.Errorf("T = %q, ожидалось %q", got, want)
	}
	// Незнакомый язык не должен ронять сообщение в пустоту.
	if T("ja", "bot.queued") != en["bot.queued"] {
		t.Error("незнакомый язык не получил английский текст")
	}
	// Неизвестный ключ виден как ключ, а не как пустая строка.
	if got := T("ru", "нет.такого.ключа"); got != "нет.такого.ключа" {
		t.Errorf("неизвестный ключ вернул %q", got)
	}
}
