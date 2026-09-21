# dsm-mini

**English** · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · [Deutsch](README_DE.md) · [Français](README_FR.md) · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Checks](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![MIT license](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A Telegram bot with a Mini App for running a home Synology NAS: Download Station
downloads and File Station files straight from the messenger, with no VPN and no
DSM web interface.

Drop a magnet link into the chat — the bot asks with buttons where to put it and
queues it. Open the app — you see what is downloading, how long is left, what is
on the disks and how the NAS is feeling.

<p align="center">
  <img src="docs/screenshots/en-home.webp" width="19%" alt="NAS overview">
  <img src="docs/screenshots/en-downloads.webp" width="19%" alt="Downloads">
  <img src="docs/screenshots/en-task.webp" width="19%" alt="A task">
  <img src="docs/screenshots/en-files.webp" width="19%" alt="Files">
  <img src="docs/screenshots/en-storage.webp" width="19%" alt="Storage">
</p>

> Working: the bot, the Mini App and notifications. Tested on DSM 7.2.2 with
> Download Station 4.1.2 and File Station 1.4.4. DSM 6 support is written from
> the documentation but has not been checked on a live DSM 6.

## What it can do

**Downloads**

- The task list: progress, speed, time left, seeds and peers
- Pause, resume, delete
- Adding by magnet link, by direct link and by `.torrent` file
- Choosing the destination folder, with a configurable quick-access list
- Choosing the files inside a torrent and their priority
- A message in the chat when a task finishes or fails

**Files**

- Browsing folders, previews, uploading to the NAS
- Rename, copy, move, delete

**NAS state**

- CPU and memory load, uptime, network
- Disks, pools and volumes: temperature, space used, health
- Virtual machines and containers: start and stop
- The DSM event log

**Language**

The app and the bot speak the language chosen in Telegram: English, Russian,
Spanish, Portuguese, German, French, Italian, Turkish, Ukrainian, Polish. An
unknown language gets English.

---

## Installation

What follows is step by step. You need to know nothing in advance, but set aside
half an hour: most of it goes on the certificate, not on the service itself.

### What you will need

- **A Synology NAS** with DSM 7. Nothing else has to be installed beforehand:
  the service ships as a DSM package and runs on the NAS itself.
- **Download Station** — install it from Package Center if it is not there yet.
- **Telegram** on your phone.
- **Access to your router** — two ports have to be forwarded.

> **Why a public address is needed.** Telegram only opens a Mini App over
> `https://` with a real certificate. A self-signed one, a local `192.168.…` and
> an address like `nas:5001` will not do: the app simply will not open. The bot
> works without an address — just without the button.

---

### Step 1. Create the bot

1. Open [@BotFather](https://t.me/BotFather) in Telegram and press **Start**.
2. Send `/newbot`.
3. Enter the bot's **name** — anything, it is what you see in the chat header.
   For example: `My NAS`.
4. Enter the bot's **username** — Latin letters, and it must end in `bot`. For
   example: `alex_home_nas_bot`. If it is taken, BotFather asks for another one.
5. The answer is a line like
   `8929377165:AAFHfZfqKDUEFdv-Yq4TJ9etz4Pp-yBU3Vg`. That is the **token**. Copy
   it — you need it in step 5.

> The token is the bot's password. Whoever has it controls the bot. Do not post
> it in chats or on GitHub.

### Step 2. Find out your Telegram ID

This is the number the service uses to know it is you writing and not a stranger.

1. Open [@userinfobot](https://t.me/userinfobot) and press **Start**.
2. It answers with a number on the `Id` line, for example `123456789`. Write it
   down.

### Step 3. Create a separate user on the NAS

The service can delete files, so giving it an administrator is a bad idea.

1. In DSM: **Control Panel → User & Group → User → Create**.
2. Name: `dsm-mini`. The password — long and random; write it down.
3. **Do not turn on two-factor verification.** There is nowhere to get the
   one-time code from, and the sign-in simply will not go through.
4. Groups: leave `users`.
5. Shared folders: give access **only** to the ones you will download into
   (usually `download` or `Media`). "No access" for the rest.
6. Applications: allow **Download Station** and **File Station**, deny the rest.

> If in step 5 the log says "authentication with DSM failed" — come back here and
> allow this user the **DSM** application as well: on some versions the sign-in
> does not go through without it, even over the API.

### Step 4. Get an address and a certificate

If you already have a domain with a valid certificate on the NAS — skip this step.

1. **The name.** Control Panel → **External Access → DDNS → Add**. Service
   provider `Synology`, hostname anything free, for example `alex-nas`. That
   gives the address `alex-nas.synology.me`. Save.
2. **Ports on the router.** In the router settings forward **port 80** and
   **port 443** to the NAS's internal address. Without 80 the certificate will
   not be issued, without 443 the app will not open.
3. **The certificate.** Control Panel → **Security → Certificate → Add → Get a
   certificate from Let's Encrypt**. Domain name is that same
   `alex-nas.synology.me`, the email is yours. Issuing takes a minute.

Check it: open `https://alex-nas.synology.me:5001` from your phone over mobile
internet (not over the home Wi-Fi). DSM should open with no certificate warnings.

### Step 5. Install the package

The easiest route is to **add a package source**, so installation and updates go
straight through Package Center:

1. **Package Center → Settings → Package Sources → Add**.
2. Name: `dsm-mini`. The address depends on your architecture:
   - Intel and AMD (most models): `https://alexlnos.github.io/dsm-mini/amd64.json`
   - ARM (budget models): `https://alexlnos.github.io/dsm-mini/arm64.json`
3. Allow third-party packages: **Settings → General → Trust Level → Any
   publisher**.
4. A **Community** section appears on the left, with `dsm-mini` in it. Press
   Install — the wizard then asks for the settings.

If you do not know your architecture — try `amd64`: DSM simply refuses to
install a package that does not fit, nothing can be broken this way.

**Or by hand, without a source:**

1. Download the `.spk` from the
   [releases](https://github.com/alexlnos/dsm-mini/releases) page: `-amd64` for
   Intel and AMD models (DS918+, DS923+, DS1522+, SA6400 and the like), `-arm64`
   for budget ARM ones (DS223, DS124). Not sure — take `amd64`: DSM simply
   refuses to install a package that does not fit.
2. **Package Center → Manual Install → Browse** and pick the downloaded file.
3. DSM says the publisher is unknown. That is normal for a third-party package:
   allow it once in **Package Center → Settings → General → Trust Level → Any
   publisher**.

#### What the wizard asks

The installer has two screens and seven fields. Everything you need for them was
collected in steps 1 to 4.

| Field | What to put in |
|---|---|
| DSM address | Already filled in: `https://localhost:5001`. The service runs on the NAS itself, so leave it as it is |
| DSM user | The name of the user from step 3, for example `dsm-mini` |
| DSM password | That user's password |
| Bot token | The token from step 1 |
| Allowed Telegram IDs | Your number from step 2. Several people — comma separated |
| Public HTTPS address | Your address from step 4, for example `https://alex-nas.synology.me` |
| Local port | Leave `8080`. Change it only if something on the NAS already holds that port |

**The mistakes people actually make here:**

| Written | Correct |
|---|---|
| `alex-nas.synology.me` | `https://alex-nas.synology.me` — with the protocol |
| `https://alex-nas.synology.me/` | no trailing slash |
| An empty list of IDs | empty means **nobody**; put your number in |
| Your public address as the DSM address | the DSM address stays `https://localhost:5001` |
| A one-time 2FA code as the password | the account must be without two-factor at all (step 3) |

The package starts by itself after installation and comes up together with the
NAS. The settings then live in `/var/packages/dsm-mini/var/config.env` (mode
`600`), the log next to them in `dsm-mini.log`.

If the service cannot start — a wrong token, a wrong password, no network — it
says so in the **DSM notification centre**, with the reason. The full log is in
Package Center, on the package's page.

### Step 6. Point the address at the service

Right now the service only listens inside the NAS, on port 8080. The reverse
proxy takes requests from the internet over HTTPS and passes them on to it.

1. **Control Panel → Login Portal → Advanced → Reverse Proxy → Create**.
   (In DSM 7.0–7.1 this is **Control Panel → Application Portal → Reverse
   Proxy**.)
2. **Source**: protocol `HTTPS`, hostname `alex-nas.synology.me`, port `443`.
3. **Destination**: protocol `HTTP`, hostname `localhost`, port `8080`.
4. Save.

> **Do not proxy port 80 for this hostname**: DSM renews the Let's Encrypt
> certificate through it, and intercepting it breaks the renewal three months
> later.

### Step 7. Check

Open `https://alex-nas.synology.me/healthz` in a browser. It should answer:

```json
{"status":"ok"}
```

If it answered — the service is alive and reachable from outside. It gives no
data away while doing so: any request without a Telegram signature is refused.

### Step 8. Open the app

1. Find your bot in Telegram by the username from step 1.
2. Press **Start**.
3. At the bottom, next to the input field, a **Downloads** button appears — it
   opens the app. The bot puts the button there itself at startup, nothing has to
   be set up by hand.
4. Send the bot any magnet link — it offers folders as buttons.

Done.

---

## If something went wrong

| What you see | What it is | What to do |
|---|---|---|
| The bot is silent on `/start` | Wrong token, or the package is not running | Package Center → `dsm-mini` → the log |
| "Access to this bot is closed" | Your ID is not on the list | Put the number from step 2 into the allowed IDs (see "Changing the settings" below) |
| There is no app button | The public address is empty or not `https://` | Same place: the settings file, then restart the package |
| The button is there, the app does not open | The reverse proxy or the certificate is not working | Open `https://your-address/healthz` in a browser |
| "Open the app through the bot" | The app was opened by a direct link in a browser | That is intended: open it from the bot |
| "Access denied: your Telegram ID…" | The service did not recognise you | The allowed IDs take digits only, comma separated |
| "authentication with DSM failed" in the log | The password, 2FA or the user's permissions | Step 3: password without typos, 2FA off, applications allowed |
| "Could not get the task list" in the log | Download Station is not installed, or is denied to the user | Package Center and the permissions from step 3 |
| The package stops right after starting | A setting is wrong — the log names which | The DSM notification centre shows the reason too |

The package log is the main source of truth: it names exactly what is missing.
It lives at `/var/packages/dsm-mini/var/dsm-mini.log` and opens from Package
Center.

### Changing the settings

Everything the wizard asked for is in one file,
`/var/packages/dsm-mini/var/config.env`. The simplest way to change a value is to
install the package over itself — the wizard asks again. To edit the file
directly you need SSH to the NAS; after editing, stop and start the package in
Package Center.

## Updating

If the package source was added, Package Center shows the update itself. Without
a source, download the fresh `.spk` from the
[releases](https://github.com/alexlnos/dsm-mini/releases) and install it over the
old one.

The settings and the database survive either way: they live in the package's
`var` directory, which an upgrade does not touch.

## Where the data is kept

Everything is in `/var/packages/dsm-mini/var/`:

- `config.env` — what the wizard asked for, mode `600`;
- `dsm-mini.db` — an SQLite database: the pinned folders, the language and the
  last known task statuses, from which the service knows what it has already
  reported;
- `dsm-mini.log` — the log.

An upgrade of the package keeps all three. Uninstalling removes them.

## Security

The service is exposed to the internet and can delete files on the NAS, so:

- **A separate DSM user**, not an administrator (step 3).
- **No two-factor verification** on that account: a one-time code cannot work
  from a configuration file.
- **The allowed IDs are an allow list.** An empty list closes access to everyone,
  it does not open it.
- Every request to `/api/` is checked twice: the `initData` signature with a key
  derived from the bot token, and the id against the allowed list. The signature
  only proves that a person opened the bot — and anyone can open it.
- The outside gets a generic phrase, the details go to the log: a message saying
  exactly what did not match in the signature would be a hint on how to forge it.
- The bot token and the DSM password live only in `config.env` on the NAS, mode
  `600`, and never reach the log: on a refusal the reason is written, never the
  value.
- The service listens on `127.0.0.1` only — from the NAS itself. Everything from
  outside goes through the DSM reverse proxy, which terminates TLS.

## Development

```bash
cd web && npm install && npm run build   # the Mini App lands in internal/web/dist
cd .. && go build ./cmd/dsm-mini         # a binary with the app embedded
go test ./...
```

The frontend on its own, with hot reload:

```bash
cd web && npm run dev    # talks to the backend on localhost:8080
```

Running the backend locally reads the same variables the package wizard writes
into `config.env`. It is convenient to keep them in a file:

```bash
cp .env.example .env     # fill it in
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

The built interface is in the repository (`internal/web/dist`) — `go:embed`
picks it up. After changing the frontend, rebuild and commit the result,
otherwise the checks will not pass.

Integration tests run against a real NAS and are skipped by default:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Tests that change the state of the NAS need separate permission:
`DSM_TEST_MUTATIONS=1`, and for file operations also `DSM_TEST_FOLDER` — the
folder inside which temporary files may be created. They delete everything they
created.

A new interface language is one dictionary file on each side:
`web/src/i18n/<code>.ts` and `internal/i18n/<code>.go`. You cannot miss a string:
on the frontend the type watches for that, on the server a test does.

## Synology API quirks

The Synology Web API behaves differently from its documentation in places: one
and the same action answers in three different shapes, `limit = -1` takes File
Station down, and `_sid` during a file upload has to be passed differently from
everywhere else. Everything learned on a live NAS is collected in
[docs/synology-api.md](docs/synology-api.md).

## License

[MIT](LICENSE)
