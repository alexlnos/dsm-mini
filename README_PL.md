<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · [Deutsch](README_DE.md) · [Français](README_FR.md) · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · **Polski**

[![Kontrole](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![Licencja MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Bot Telegrama z Mini App do zarządzania domowym NAS-em Synology: pobierania
Download Station i pliki File Station prosto z komunikatora, bez VPN-a i bez
interfejsu webowego DSM.

Wrzucasz link magnet na czat — bot pyta przyciskami, gdzie pobrać, i wstawia do
kolejki. Otwierasz aplikację — widać, co się pobiera, ile zostało, co leży na
dyskach i jak się miewa NAS.

<p align="center">
  <img src="docs/screenshots/pl-home.webp" width="19%" alt="Przegląd NAS-a">
  <img src="docs/screenshots/pl-downloads.webp" width="19%" alt="Pobierania">
  <img src="docs/screenshots/pl-task.webp" width="19%" alt="Zadanie">
  <img src="docs/screenshots/pl-files.webp" width="19%" alt="Pliki">
  <img src="docs/screenshots/pl-storage.webp" width="19%" alt="Magazyn">
</p>

> Działa: bot, Mini App i powiadomienia. Potrzebny jest DSM 7 albo nowszy — na
> DSM 6 pakiet się nie zainstaluje i nikt go tam nie sprawdzał. Sprawdzone na
> DSM 7.2.2 z Download Station 4.1.2 i File Station 1.4.4.

## Co to potrafi

**Pobierania**

- Lista zadań: postęp, prędkość, pozostały czas, seedy i peery
- Wstrzymanie, wznowienie, usunięcie
- Dodawanie linkiem magnet, linkiem bezpośrednim i plikiem `.torrent`
- Wybór folderu docelowego, z konfigurowalną listą szybkiego dostępu
- Wybór plików wewnątrz torrenta i ich priorytetu
- Wiadomość na czacie, gdy zadanie się skończy albo padnie

**Pliki**

- Przeglądanie folderów, podgląd, wysyłanie na NAS
- Zmiana nazwy, kopiowanie, przenoszenie, usuwanie

**Stan NAS-a**

- Obciążenie procesora i pamięci, czas pracy, sieć
- Dyski, pule i woluminy: temperatura, zajęte miejsce, kondycja
- Maszyny wirtualne i kontenery: uruchamianie i zatrzymywanie
- Dziennik zdarzeń DSM

**Powiadomienia i ustawienia**

- To, co zgłasza sam DSM — doradca bezpieczeństwa, dyski, aktualizacje — trafia na czat
- Wybór, co bot może wysyłać: nic, tylko pobierania, wszystko
- Własny ekran w menu głównym DSM: każde ustawienie, bez edycji pliku przez SSH

**Język**

Aplikacja i bot mówią w języku wybranym w Telegramie: angielski, rosyjski,
hiszpański, portugalski, niemiecki, francuski, włoski, turecki, ukraiński,
polski. Nieznany język dostaje angielski.

---

## Instalacja

Pół godziny, z czego większość pochłania certyfikat, a nie sama usługa.
Potrzebny jest NAS Synology z DSM 7 i Download Station; niczego więcej nie
trzeba instalować wcześniej.

> **Po co publiczny adres.** Telegram otwiera Mini App tylko po `https://` z
> prawdziwym certyfikatem: lokalny `192.168.…` ani samopodpisany się nie
> otworzą. Sam bot działa i bez adresu, tylko bez przycisku.

**1. Utworzyć bota.** [@BotFather](https://t.me/BotFather) → `/newbot` → nazwa
i login kończący się na `bot`. W odpowiedzi przyjdzie token; zachowaj go, to
hasło do twojego bota.

**2. Poznać swój numer.** [@userinfobot](https://t.me/userinfobot) → Start.
Odpowie linią `Id`.

**3. Założyć użytkownika DSM dla usługi.** Panel sterowania → Użytkownik i
grupa → Utwórz. Dostęp tylko do Download Station i File Station, bez
weryfikacji dwuetapowej: kod jednorazowy nie ma jak trafić z pliku
konfiguracyjnego. Nie dawaj administratora: usługa potrafi kasować pliki.

**4. Zdobyć adres i certyfikat.** Pomiń, jeśli NAS ma już domenę z ważnym
certyfikatem.

- Panel sterowania → Dostęp zewnętrzny → DDNS → Dodaj, usługodawca `Synology`:
  wyjdzie coś w rodzaju `alex-nas.synology.me`.
- Na routerze przekieruj na NAS porty **80** i **443**. Bez 80 nie wystawi się
  certyfikat, bez 443 nie otworzy się aplikacja.
- Panel sterowania → Zabezpieczenia → Certyfikat → Dodaj → z Let's Encrypt, na
  tę samą nazwę.

Sprawdź z telefonu przez internet mobilny:
`https://alex-nas.synology.me:5001` powinien otworzyć DSM bez ostrzeżeń.

**5. Zainstalować pakiet.** Centrum pakietów → Ustawienia → Źródła pakietów →
Dodaj, nazwa `dsm-mini` i adres według twojej architektury:

- Intel i AMD, większość modeli: `https://alexlnos.github.io/dsm-mini/amd64.json`
- ARM, modele budżetowe: `https://alexlnos.github.io/dsm-mini/arm64.json`

Dalej Ustawienia → Ogólne → Poziom zaufania → **Dowolny wydawca**, i
zainstaluj **DSM mini (Telegram Mini App)** z sekcji **Społeczność**. Nie masz
pewności co do architektury? Spróbuj `amd64`: niepasujący pakiet po prostu
zostanie odrzucony.

Instalator pyta o pięć wartości i przy każdej tłumaczy, czym jest —
wszystko do nich zebrano w krokach powyżej. Albo zainstaluj `.spk` z
[wydań](https://github.com/alexlnos/dsm-mini/releases) ręcznie, przez Centrum
pakietów → Instalacja ręczna.

**6. Skierować adres na usługę.** Panel sterowania → Portal logowania →
Zaawansowane → Zwrotny serwer proxy → Utwórz. Źródło: `HTTPS`, twoja nazwa,
port `443`. Miejsce docelowe: `HTTP`, `localhost`, port `8080`.

> Portu **80** dla tej nazwy nie przepuszczaj przez proxy: przez niego DSM
> odnawia certyfikat, a przechwycenie zepsuje odnowienie trzy miesiące później.

**7. Sprawdzić.** `https://twoj-adres/healthz` w przeglądarce powinno
odpowiedzieć `{"status":"ok"}`. Bez podpisu Telegrama nic nie jest wydawane.

**8. Otworzyć aplikację.** Znajdź bota po loginie, naciśnij Start — obok pola
wpisywania pojawi się przycisk **Pobierania**. Wyślij mu dowolny link magnet,
zaproponuje foldery przyciskami.

## Jeśli coś poszło nie tak

| Co widzisz | O co chodzi | Co zrobić |
|---|---|---|
| Bot milczy na `/start` | Zły token albo pakiet nie jest uruchomiony | Centrum pakietów → **DSM mini (Telegram Mini App)** → dziennik |
| „Dostęp do tego bota jest zamknięty” | Twojego ID nie ma na liście | Wpisz liczbę z kroku 2 do dozwolonych identyfikatorów (patrz „Zmienić ustawienia” niżej) |
| Nie ma przycisku aplikacji | Publiczny adres jest pusty albo nie jest `https://` | To samo miejsce: plik ustawień, potem zrestartuj pakiet |
| Przycisk jest, aplikacja się nie otwiera | Nie działa zwrotny serwer proxy albo certyfikat | Otwórz `https://twoj-adres/healthz` w przeglądarce |
| „Otwórz aplikację przez bota” | Aplikację otwarto bezpośrednim linkiem w przeglądarce | Tak ma być: otwieraj z bota |
| „Dostęp zamknięty: twoje Telegram ID…” | Usługa cię nie rozpoznała | W dozwolonych identyfikatorach tylko cyfry, po przecinku |
| W dzienniku `authentication with DSM failed` | Hasło, 2FA albo uprawnienia użytkownika | Krok 3: hasło bez literówek, 2FA wyłączona, aplikacje dozwolone |
| W dzienniku `Could not get the task list` | Download Station nie jest zainstalowany albo zabroniony użytkownikowi | Centrum pakietów i uprawnienia z kroku 3 |
| Pakiet zatrzymuje się zaraz po starcie | Jakieś ustawienie jest złe — dziennik powie które | Przyczyna trafia też do centrum powiadomień DSM |

Dziennik pakietu to główne źródło prawdy: wprost mówi, czego brakuje. Leży w
`/var/packages/dsm-mini/var/dsm-mini.log` i otwiera się z Centrum pakietów.
Dziennik prowadzony jest po angielsku, interfejs i wiadomości bota — w twoim
języku.

### Zmienić ustawienia

Wszystko, o co pytał kreator, leży na NAS-ie w jednym pliku
`/var/packages/dsm-mini/var/config.env` z uprawnieniami `600`.

Instalacja pakietu na samym sobie **o nic nie zapyta**: kreator działa przy
instalacji, a aktualizacja celowo nie rusza tego pliku — właśnie dlatego
ustawienia ją przeżywają. Zostają trzy drogi:

- **Otwórz DSM mini w menu głównym DSM** — ekran ustawień zmienia każde z nich,
  a hasło i token są tam tylko do zapisu: puste pole zachowuje poprzednią
  wartość. Potem uruchom pakiet ponownie; ustawienie powiadomień działa od
  razu.

- **Poprawić plik przez SSH** (Panel sterowania → Terminal i SNMP → włącz SSH)
  i zrestartować pakiet w Centrum pakietów. Tak zostaje cała reszta:

  ```bash
  sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='twoj-nowy-token'|" /var/packages/dsm-mini/var/config.env
  sudo synopkg restart dsm-mini
  ```

- **Odinstalować i zainstalować ponownie** — kreator zapyta o wszystko od nowa.
  Razem z ustawieniami zniknie baza obok: przypięte foldery, język i ostatnie
  znane statusy zadań.

## Aktualizacja

Jeśli źródło pakietów jest dodane, Centrum pakietów samo pokaże aktualizację. Bez
źródła — pobierz świeży `.spk` z
[wydań](https://github.com/alexlnos/dsm-mini/releases) i zainstaluj na wierzchu.

Ustawienia i baza w obu przypadkach zostaną: leżą w katalogu `var` pakietu, a
aktualizacja go nie rusza.

## Gdzie trzymane są dane

Wszystko w `/var/packages/dsm-mini/var/`:

- `config.env` — to, o co pytał kreator, uprawnienia `600`;
- `dsm-mini.db` — baza SQLite: przypięte foldery, język i ostatnie znane statusy
  zadań, po których usługa wie, o czym już powiadomiła;
- `dsm-mini.log` — dziennik.

Aktualizacja pakietu zachowuje wszystkie trzy. Odinstalowanie je kasuje.

## Bezpieczeństwo

Usługa jest wystawiona do internetu i potrafi kasować pliki na NAS-ie, dlatego:

- **Osobny użytkownik DSM**, a nie administrator (krok 3).
- **Bez weryfikacji dwuetapowej** na tym koncie: kod jednorazowy nie ma prawa
  zadziałać z pliku konfiguracyjnego.
- **Dozwolone identyfikatory to biała lista.** Pusta lista zamyka dostęp
  wszystkim, a nie otwiera.
- Każde żądanie do `/api/` sprawdzane jest dwa razy: podpis `initData` kluczem z
  tokenu bota i identyfikator wobec listy dozwolonych. Podpis dowodzi tylko
  tego, że ktoś otworzył bota — a otworzyć go może każdy.
- Na zewnątrz idzie ogólne zdanie, szczegóły do dziennika: komunikat mówiący, co
  dokładnie nie zgadzało się w podpisie, podpowiadałby, jak go podrobić.
- Token bota i hasło DSM żyją wyłącznie w `config.env` na samym NAS-ie z
  uprawnieniami `600` i nigdy nie trafiają do dziennika: przy odmowie zapisywana
  jest przyczyna, nigdy wartość.
- Usługa słucha tylko na `127.0.0.1` — czyli z samego NAS-a. Wszystko z zewnątrz
  idzie przez zwrotny serwer proxy DSM, który kończy też TLS.

## Rozwój

```bash
cd web && npm install && npm run build   # Mini App trafi do internal/web/dist
cd .. && go build ./cmd/dsm-mini         # jeden plik binarny z wbudowaną aplikacją
go test ./...
```

Frontend osobno, z automatycznym przeładowaniem:

```bash
cd web && npm run dev    # rozmawia z backendem na localhost:8080
```

Lokalne uruchomienie backendu czyta te same zmienne, które kreator pakietu
zapisuje w `config.env`. Wygodnie trzymać je w pliku:

```bash
cp .env.example .env     # wypełnij
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

Zbudowany interfejs leży w repozytorium (`internal/web/dist`) — stamtąd bierze go
`go:embed`. Po zmianach we frontendzie przebuduj i zacommituj wynik, bo inaczej
kontrole nie przejdą.

Testy integracyjne działają na prawdziwym NAS-ie i domyślnie są pomijane:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Testy zmieniające stan NAS-a wymagają osobnej zgody: `DSM_TEST_MUTATIONS=1`, a
do operacji na plikach jeszcze `DSM_TEST_FOLDER` — folderu, wewnątrz którego
wolno tworzyć pliki tymczasowe. Wszystko, co utworzą, sprzątają po sobie.

Nowy język interfejsu to jeden plik słownika po każdej stronie:
`web/src/i18n/<kod>.ts` i `internal/i18n/<kod>.go`. Pominąć linii się nie da: na
froncie pilnuje tego typ, na serwerze — test.

Kod, komentarze i dziennik w projekcie są po angielsku; pozostałe języki żyją
tylko w słownikach. Szczegóły w [CLAUDE.md](CLAUDE.md).

## Osobliwości API Synology

Web API Synology miejscami zachowuje się inaczej, niż napisano w dokumentacji: ta
sama akcja odpowiada w trzech różnych formatach, `limit = -1` kładzie File
Station, a `_sid` przy wysyłaniu pliku trzeba przekazać inaczej niż wszędzie.
Wszystko, co ustalono na żywym NAS-ie, zebrano w
[docs/synology-api.md](docs/synology-api.md).

## Licencja

[MIT](LICENSE)
