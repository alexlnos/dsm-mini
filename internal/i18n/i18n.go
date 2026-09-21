// Package i18n — тексты, которые читает человек: сообщения бота, вид бота в
// Telegram и сообщения об ошибках, уходящие в Mini App.
//
// Язык берётся у Telegram: в чате это `language_code` отправителя, в Mini App
// — тот же код из подписанных параметров запуска. Незнакомый язык получает
// английский.
//
// Русский словарь — источник истины. Полноту остальных проверяет тест
// (Go не умеет требовать этого типом, как TypeScript на фронтенде).
//
// Множественного числа здесь намеренно нет: сообщения написаны так, чтобы
// число стояло после двоеточия («Активных задач: 5»), и согласование не
// требовалось ни в одном из языков.
package i18n

import (
	"sort"
	"strings"
)

// Lang — код языка в том виде, в каком его знает словарь.
type Lang string

// Fallback — язык для всех, чей язык нам незнаком.
const Fallback Lang = "en"

// P — подстановки в строку: {name} заменяется на значение.
type P map[string]string

var dicts = map[Lang]map[string]string{
	"en": en, "ru": ru, "es": es, "pt": pt, "de": de,
	"fr": fr, "it": it, "tr": tr, "uk": uk, "pl": pl,
}

// Languages перечисляет известные языки в устойчивом порядке.
//
// Порядок важен там, где обход словарей виден снаружи: без сортировки Go
// выдаёт ключи карты вразнобой, и журнал запуска каждый раз выглядел бы
// по-новому.
func Languages() []Lang {
	out := make([]Lang, 0, len(dicts))
	for lang := range dicts {
		out = append(out, lang)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Match подбирает язык по коду от Telegram: «ru», «en», «pt-BR».
//
// Берём основную часть кода: региональных словарей у нас нет, а «pt-BR» и
// «pt-PT» должны показать португальский, а не английский.
func Match(code string) Lang {
	base := code
	if i := strings.IndexAny(base, "-_"); i >= 0 {
		base = base[:i]
	}
	lang := Lang(strings.ToLower(base))
	if _, ok := dicts[lang]; ok {
		return lang
	}
	return Fallback
}

// T — строка на заданном языке.
//
// Неизвестный ключ возвращается как есть: пустое место в интерфейсе хуже
// английской строки, а английская строка хуже самого ключа только тем, что
// некрасива — зато сразу видно, чего не хватает.
func T(lang Lang, key string, params ...P) string {
	dict, ok := dicts[lang]
	if !ok {
		dict = dicts[Fallback]
	}
	text, ok := dict[key]
	if !ok {
		if text, ok = dicts[Fallback][key]; !ok {
			if text, ok = ru[key]; !ok {
				return key
			}
		}
	}
	for _, p := range params {
		for name, value := range p {
			text = strings.ReplaceAll(text, "{"+name+"}", value)
		}
	}
	return text
}
