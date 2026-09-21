<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · **Deutsch** · [Français](README_FR.md) · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Prüfungen](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![MIT-Lizenz](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Ein Telegram-Bot mit Mini App für das heimische Synology-NAS: Downloads der
Download Station und Dateien der File Station direkt aus dem Messenger, ohne VPN
und ohne die DSM-Weboberfläche.

Einen Magnet-Link in den Chat geworfen — der Bot fragt mit Schaltflächen, wohin
damit, und stellt ihn in die Warteschlange. Die App geöffnet — und man sieht, was
lädt, wie lange es noch dauert, was auf den Platten liegt und wie es dem NAS
geht.

<p align="center">
  <img src="docs/screenshots/de-home.webp" width="19%" alt="NAS-Übersicht">
  <img src="docs/screenshots/de-downloads.webp" width="19%" alt="Downloads">
  <img src="docs/screenshots/de-task.webp" width="19%" alt="Eine Aufgabe">
  <img src="docs/screenshots/de-files.webp" width="19%" alt="Dateien">
  <img src="docs/screenshots/de-storage.webp" width="19%" alt="Speicher">
</p>

> Funktioniert: der Bot, die Mini App und die Benachrichtigungen. Nötig ist DSM
> 7 oder neuer — auf DSM 6 lässt sich das Paket nicht installieren, und geprüft
> wurde es dort nie. Getestet auf DSM 7.2.2 mit Download Station 4.1.2 und
> File Station 1.4.4.

## Was es kann

**Downloads**

- Die Aufgabenliste: Fortschritt, Geschwindigkeit, Restzeit, Seeds und Peers
- Pausieren, fortsetzen, löschen
- Hinzufügen per Magnet-Link, per direktem Link und per `.torrent`-Datei
- Auswahl des Zielordners, mit einer einstellbaren Schnellzugriffsliste
- Auswahl der Dateien innerhalb eines Torrents und ihrer Priorität
- Eine Nachricht im Chat, wenn eine Aufgabe fertig ist oder scheitert

**Dateien**

- Ordner durchsehen, Vorschau, Hochladen auf das NAS
- Umbenennen, kopieren, verschieben, löschen

**Zustand des NAS**

- CPU- und Speicherauslastung, Laufzeit, Netzwerk
- Platten, Pools und Volumes: Temperatur, belegter Platz, Zustand
- Virtuelle Maschinen und Container: starten und stoppen
- Das DSM-Ereignisprotokoll

**Sprache**

App und Bot sprechen die in Telegram gewählte Sprache: Englisch, Russisch,
Spanisch, Portugiesisch, Deutsch, Französisch, Italienisch, Türkisch,
Ukrainisch, Polnisch. Eine unbekannte Sprache bekommt Englisch.

---

## Installation

Es geht Schritt für Schritt weiter. Vorwissen braucht man keines, aber plane eine
halbe Stunde ein: das meiste davon geht für das Zertifikat drauf, nicht für den
Dienst selbst.

### Was gebraucht wird

- **Ein Synology-NAS** mit DSM 7. Vorher muss nichts installiert werden: der
  Dienst kommt als DSM-Paket und läuft auf dem NAS selbst.
- **Download Station** — aus dem Paket-Zentrum installieren, falls noch nicht da.
- **Telegram** auf dem Telefon.
- **Zugang zum Router** — zwei Ports müssen weitergeleitet werden.

> **Warum eine öffentliche Adresse nötig ist.** Telegram öffnet eine Mini App nur
> über `https://` mit einem echten Zertifikat. Ein selbstsigniertes, ein lokales
> `192.168.…` und eine Adresse wie `nas:5001` taugen nicht: die App geht schlicht
> nicht auf. Der Bot läuft auch ohne Adresse — nur eben ohne die Schaltfläche.

---

### Schritt 1. Den Bot anlegen

1. Öffne [@BotFather](https://t.me/BotFather) in Telegram und drücke **Start**.
2. Sende `/newbot`.
3. Gib den **Namen** des Bots ein — beliebig, er steht in der Chat-Kopfzeile.
   Zum Beispiel: `Mein NAS`.
4. Gib den **Benutzernamen** des Bots ein — lateinische Buchstaben und
   zwingend auf `bot` endend. Zum Beispiel: `alex_home_nas_bot`. Ist er vergeben,
   fragt BotFather nach einem anderen.
5. Zurück kommt eine Zeile wie
   `1234567890:AAExampleTokenReplaceThisWithYours0`. Das ist das **Token**.
   Kopiere es — im Schritt 5 wird es gebraucht.

> Das Token ist das Passwort des Bots. Wer es hat, steuert den Bot. Nicht in
> Chats und nicht auf GitHub veröffentlichen.

### Schritt 2. Die eigene Telegram-ID herausfinden

Das ist die Zahl, an der der Dienst erkennt, dass du schreibst und kein Fremder.

1. Öffne [@userinfobot](https://t.me/userinfobot) und drücke **Start**.
2. Er antwortet mit einer Zahl in der Zeile `Id`, zum Beispiel `123456789`.
   Notieren.

### Schritt 3. Einen eigenen Benutzer auf dem NAS anlegen

Der Dienst kann Dateien löschen, ihm einen Administrator zu geben ist also eine
schlechte Idee.

1. In DSM: **Systemsteuerung → Benutzer & Gruppe → Benutzer → Erstellen**.
2. Name: `dsm-mini`. Das Passwort lang und zufällig; notieren.
3. **Die zweistufige Verifizierung nicht einschalten.** Den Einmalcode kann
   niemand herholen, und die Anmeldung geht schlicht nicht durch.
4. Gruppen: `users` so lassen.
5. Freigegebene Ordner: Zugriff **nur** auf die geben, in die heruntergeladen
   wird (meist `download` oder `Media`). Für den Rest „Kein Zugriff“.
6. Anwendungen: **Download Station** und **File Station** erlauben, den Rest
   verbieten.

> Steht in Schritt 5 im Protokoll `authentication with DSM failed` — komm hierher
> zurück und erlaube diesem Benutzer zusätzlich die Anwendung **DSM**: bei
> manchen Versionen geht die Anmeldung ohne sie nicht durch, auch nicht über die
> API.

### Schritt 4. Adresse und Zertifikat besorgen

Wer schon eine Domain mit gültigem Zertifikat auf dem NAS hat, überspringt den
Schritt.

1. **Der Name.** Systemsteuerung → **Externer Zugriff → DDNS → Hinzufügen**.
   Dienstanbieter `Synology`, Hostname ein beliebiger freier, zum Beispiel
   `alex-nas`. Heraus kommt die Adresse `alex-nas.synology.me`. Speichern.
2. **Ports am Router.** In den Router-Einstellungen **Port 80** und **Port 443**
   auf die interne Adresse des NAS weiterleiten. Ohne 80 wird kein Zertifikat
   ausgestellt, ohne 443 geht die App nicht auf.
3. **Das Zertifikat.** Systemsteuerung → **Sicherheit → Zertifikat → Hinzufügen →
   Ein Zertifikat von Let's Encrypt abrufen**. Domainname ist genau dieses
   `alex-nas.synology.me`, die E-Mail deine. Das Ausstellen dauert eine Minute.

Prüfe es: öffne `https://alex-nas.synology.me:5001` vom Telefon über mobiles
Internet (nicht über das heimische WLAN). DSM muss ohne Zertifikatswarnungen
aufgehen.

### Schritt 5. Das Paket installieren

Am einfachsten ist es, **eine Paketquelle hinzuzufügen**, dann laufen
Installation und Aktualisierungen direkt über das Paket-Zentrum:

1. **Paket-Zentrum → Einstellungen → Paketquellen → Hinzufügen**.
2. Name: `dsm-mini`. Die Adresse je nach Architektur:
   - Intel und AMD (die meisten Modelle): `https://alexlnos.github.io/dsm-mini/amd64.json`
   - ARM (günstige Modelle): `https://alexlnos.github.io/dsm-mini/arm64.json`
3. Pakete von Drittanbietern erlauben: **Einstellungen → Allgemein →
   Vertrauensebene → Beliebiger Herausgeber**.
4. Links erscheint der Bereich **Community** und darin `dsm-mini`. Auf
   Installieren drücken — danach fragt der Assistent die Einstellungen ab.

Du kennst deine Architektur nicht — probier `amd64`: ein unpassendes Paket
verweigert DSM einfach, kaputtmachen kann man damit nichts.

**Oder von Hand, ohne Quelle:**

1. Lade die `.spk` von der Seite mit den
   [Veröffentlichungen](https://github.com/alexlnos/dsm-mini/releases): `-amd64`
   für Modelle mit Intel und AMD (DS918+, DS923+, DS1522+, SA6400 und
   ähnliche), `-arm64` für die günstigen mit ARM (DS223, DS124). Im Zweifel
   `amd64` nehmen: ein unpassendes Paket verweigert DSM einfach.
2. **Paket-Zentrum → Manuelle Installation → Durchsuchen** und die
   heruntergeladene Datei auswählen.
3. DSM sagt, der Herausgeber sei unbekannt. Das ist bei einem Fremdpaket normal:
   einmalig erlauben unter **Paket-Zentrum → Einstellungen → Allgemein →
   Vertrauensebene → Beliebiger Herausgeber**.

#### Was der Assistent fragt

Der Installer hat zwei Seiten und sieben Felder. Alles dafür wurde in den
Schritten 1 bis 4 zusammengetragen.

| Feld | Was hineingehört |
|---|---|
| DSM-Adresse | Schon eingetragen: `https://localhost:5001`. Der Dienst läuft auf dem NAS selbst, also so lassen |
| DSM-Benutzer | Der Name des Benutzers aus Schritt 3, zum Beispiel `dsm-mini` |
| DSM-Passwort | Das Passwort dieses Benutzers |
| Bot-Token | Das Token aus Schritt 1 |
| Erlaubte Telegram-IDs | Deine Nummer aus Schritt 2. Mehrere Personen: durch Komma getrennt |
| Öffentliche HTTPS-Adresse | Deine Adresse aus Schritt 4, zum Beispiel `https://alex-nas.synology.me` |
| Lokaler Port | `8080` lassen. Nur ändern, wenn auf dem NAS schon etwas diesen Port belegt |

**Die Fehler, die hier wirklich passieren:**

| Geschrieben | Richtig |
|---|---|
| `alex-nas.synology.me` | `https://alex-nas.synology.me` — mit Protokoll |
| `https://alex-nas.synology.me/` | ohne Schrägstrich am Ende |
| Eine leere Liste von IDs | leer heißt **niemand**; trag deine Nummer ein |
| Die öffentliche Adresse im Feld für die DSM-Adresse | die DSM-Adresse bleibt `https://localhost:5001` |
| Ein Einmalcode der 2FA als Passwort | das Konto darf gar keine zweistufige Verifizierung haben (Schritt 3) |

Nach der Installation startet das Paket von allein und kommt zusammen mit dem
NAS hoch. Die Einstellungen liegen dann in
`/var/packages/dsm-mini/var/config.env` (Rechte `600`), das Protokoll daneben in
`dsm-mini.log`.

Wenn der Dienst nicht starten kann — falsches Token, falsches Passwort, kein
Netz — sagt er das im **Benachrichtigungscenter von DSM**, mitsamt dem Grund.
Das vollständige Protokoll steht im Paket-Zentrum auf der Seite des Pakets.

### Schritt 6. Die Adresse auf den Dienst lenken

Im Moment lauscht der Dienst nur innerhalb des NAS, auf Port 8080. Der Reverse
Proxy nimmt Anfragen aus dem Internet über HTTPS entgegen und gibt sie weiter.

1. **Systemsteuerung → Anmeldeportal → Erweitert → Reverse Proxy → Erstellen**.
   (In DSM 7.0–7.1 ist das **Systemsteuerung → Anwendungsportal → Reverse
   Proxy**.)
2. **Quelle**: Protokoll `HTTPS`, Hostname `alex-nas.synology.me`, Port `443`.
3. **Ziel**: Protokoll `HTTP`, Hostname `localhost`, Port `8080`.
4. Speichern.

> **Port 80 für diesen Namen nicht proxen**: darüber erneuert DSM das
> Let's-Encrypt-Zertifikat, und ein Abfangen zerlegt die Erneuerung drei Monate
> später.

### Schritt 7. Prüfen

Öffne `https://alex-nas.synology.me/healthz` im Browser. Es sollte antworten:

```json
{"status":"ok"}
```

Hat es geantwortet, lebt der Dienst und ist von außen erreichbar. Daten gibt er
dabei nicht heraus: jede Anfrage ohne Telegram-Signatur wird abgewiesen.

### Schritt 8. Die App öffnen

1. Finde deinen Bot in Telegram über den Benutzernamen aus Schritt 1.
2. Drücke **Start**.
3. Unten, neben dem Eingabefeld, erscheint eine Schaltfläche **Downloads** — sie
   öffnet die App. Der Bot setzt sie beim Start selbst, von Hand ist nichts
   einzurichten.
4. Schick dem Bot irgendeinen Magnet-Link — er bietet Ordner als Schaltflächen
   an.

Fertig.

---

## Wenn etwas schiefgegangen ist

| Was du siehst | Woran es liegt | Was zu tun ist |
|---|---|---|
| Der Bot schweigt auf `/start` | Falsches Token, oder das Paket läuft nicht | Paket-Zentrum → `dsm-mini` → das Protokoll |
| „Der Zugang zu diesem Bot ist geschlossen“ | Deine ID steht nicht auf der Liste | Die Nummer aus Schritt 2 in die erlaubten IDs eintragen (siehe „Einstellungen ändern“ unten) |
| Es gibt keine App-Schaltfläche | Die öffentliche Adresse ist leer oder nicht `https://` | Dieselbe Stelle: die Einstellungsdatei, dann das Paket neu starten |
| Die Schaltfläche ist da, die App geht nicht auf | Reverse Proxy oder Zertifikat funktionieren nicht | `https://deine-adresse/healthz` im Browser öffnen |
| „Öffne die App über den Bot“ | Die App wurde per direktem Link im Browser geöffnet | So ist es gedacht: aus dem Bot heraus öffnen |
| „Zugriff verweigert: deine Telegram-ID…“ | Der Dienst hat dich nicht erkannt | In den erlaubten IDs nur Ziffern, durch Komma getrennt |
| `authentication with DSM failed` im Protokoll | Das Passwort, 2FA oder die Rechte des Benutzers | Schritt 3: Passwort ohne Tippfehler, 2FA aus, Anwendungen erlaubt |
| `Could not get the task list` im Protokoll | Download Station ist nicht installiert oder dem Benutzer verboten | Paket-Zentrum und die Rechte aus Schritt 3 |
| Das Paket stoppt gleich nach dem Start | Eine Einstellung stimmt nicht — das Protokoll nennt welche | Der Grund kommt auch ins Benachrichtigungscenter von DSM |

Das Protokoll des Pakets ist die wichtigste Quelle der Wahrheit: es benennt
genau, was fehlt. Es liegt in `/var/packages/dsm-mini/var/dsm-mini.log` und
öffnet sich aus dem Paket-Zentrum. Das Protokoll läuft auf Englisch, Oberfläche
und Bot-Nachrichten in deiner Sprache.

### Einstellungen ändern

Alles, was der Assistent gefragt hat, steht auf dem NAS in einer Datei:
`/var/packages/dsm-mini/var/config.env`, Rechte `600`.

Das Paket über sich selbst zu installieren fragt **nicht** erneut: der
Assistent läuft bei der Installation, und eine Aktualisierung lässt die Datei
absichtlich in Ruhe — darum überstehen die Einstellungen ein Update. Bleiben
zwei Wege:

- **Die Datei über SSH bearbeiten** (Systemsteuerung → Terminal & SNMP → SSH
  einschalten) und das Paket im Paket-Zentrum neu starten. So bleibt alles
  andere erhalten:

  ```bash
  sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='dein-neues-token'|" /var/packages/dsm-mini/var/config.env
  sudo synopkg restart dsm-mini
  ```

- **Deinstallieren und neu installieren** — der Assistent fragt alles noch
  einmal. Mit den Einstellungen verschwindet auch die Datenbank daneben: die
  angehefteten Ordner, die Sprache und die zuletzt bekannten Aufgabenzustände.

## Aktualisieren

Ist die Paketquelle eingetragen, zeigt das Paket-Zentrum die Aktualisierung von
selbst. Ohne Quelle die frische `.spk` aus den
[Veröffentlichungen](https://github.com/alexlnos/dsm-mini/releases) laden und
darüber installieren.

Einstellungen und Datenbank bleiben in beiden Fällen: sie liegen im
`var`-Verzeichnis des Pakets, das eine Aktualisierung nicht anrührt.

## Wo die Daten liegen

Alles in `/var/packages/dsm-mini/var/`:

- `config.env` — was der Assistent gefragt hat, Rechte `600`;
- `dsm-mini.db` — eine SQLite-Datenbank: die angehefteten Ordner, die Sprache
  und die zuletzt bekannten Aufgabenzustände, an denen der Dienst erkennt,
  worüber er schon berichtet hat;
- `dsm-mini.log` — das Protokoll.

Ein Upgrade des Pakets behält alle drei. Das Deinstallieren löscht sie.

## Sicherheit

Der Dienst steht im Internet und kann Dateien auf dem NAS löschen, darum:

- **Ein eigener DSM-Benutzer**, kein Administrator (Schritt 3).
- **Ohne zweistufige Verifizierung** bei diesem Konto: ein Einmalcode kann aus
  einer Konfigurationsdatei nicht funktionieren.
- **Die erlaubten IDs sind eine Positivliste.** Eine leere Liste schließt den
  Zugang für alle, sie öffnet ihn nicht.
- Jede Anfrage an `/api/` wird zweimal geprüft: die `initData`-Signatur mit
  einem Schlüssel aus dem Bot-Token, und die Kennung gegen die Liste. Die
  Signatur beweist nur, dass jemand den Bot geöffnet hat — und öffnen kann ihn
  jeder.
- Nach außen geht ein allgemeiner Satz, die Einzelheiten ins Protokoll: eine
  Meldung, was genau an der Signatur nicht gestimmt hat, wäre ein Hinweis
  darauf, wie man sie fälscht.
- Bot-Token und DSM-Passwort stehen nur in `config.env` auf dem NAS selbst,
  Rechte `600`, und gelangen nie ins Protokoll: bei einer Ablehnung wird der
  Grund geschrieben, nie der Wert.
- Der Dienst lauscht nur auf `127.0.0.1` — also vom NAS selbst. Alles von außen
  läuft über den Reverse Proxy von DSM, der auch das TLS beendet.

## Entwicklung

```bash
cd web && npm install && npm run build   # die Mini App landet in internal/web/dist
cd .. && go build ./cmd/dsm-mini         # eine Binärdatei mit eingebetteter App
go test ./...
```

Das Frontend für sich, mit automatischem Neuladen:

```bash
cd web && npm run dev    # spricht mit dem Backend auf localhost:8080
```

Das Backend lokal zu starten liest dieselben Variablen, die der Paketassistent
in `config.env` schreibt. Praktisch ist es, sie in einer Datei zu halten:

```bash
cp .env.example .env     # ausfüllen
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

Die gebaute Oberfläche liegt im Repository (`internal/web/dist`) — von dort holt
sie `go:embed`. Nach Änderungen am Frontend neu bauen und das Ergebnis
committen, sonst gehen die Prüfungen nicht durch.

Die Integrationstests laufen gegen ein echtes NAS und werden standardmäßig
übersprungen:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Tests, die den Zustand des NAS ändern, brauchen eine gesonderte Erlaubnis:
`DSM_TEST_MUTATIONS=1`, und für Dateioperationen zusätzlich `DSM_TEST_FOLDER` —
den Ordner, in dem temporäre Dateien angelegt werden dürfen. Alles Angelegte
räumen sie hinter sich weg.

Eine neue Oberflächensprache ist je eine Wörterbuchdatei auf beiden Seiten:
`web/src/i18n/<Code>.ts` und `internal/i18n/<Code>.go`. Eine Zeile auszulassen
geht nicht: im Frontend wacht der Typ darüber, auf dem Server ein Test.

Code, Kommentare und Protokoll sind im Projekt englisch; die übrigen Sprachen
leben nur in den Wörterbüchern. Einzelheiten stehen in [CLAUDE.md](CLAUDE.md).

## Besonderheiten der Synology-API

Die Synology-Web-API verhält sich stellenweise anders als ihre Dokumentation:
ein und dieselbe Aktion antwortet in drei verschiedenen Formaten, `limit = -1`
legt die File Station lahm, und `_sid` muss beim Hochladen einer Datei anders
übergeben werden als überall sonst. Alles, was an einem echten NAS
herausgefunden wurde, steht in [docs/synology-api.md](docs/synology-api.md).

## Lizenz

[MIT](LICENSE)
