# dsm-mini — working rules for this project

A Telegram bot with a Mini App for managing a Synology NAS: Download Station
tasks and File Station files. Open source, MIT, Go + React.

## The main rule: never guess about the Synology API

The Synology Web API behaves differently from its documentation in places, and
`SYNO.DownloadStation2.*` is not documented at all. The order of work:

1. First **download the official PDF** — links are in [docs/synology-api.md](docs/synology-api.md).
2. Then **check the call against a live NAS** with an integration test.
3. Any discrepancy found goes **straight into `docs/synology-api.md`**, with a sample response.

Explaining API behaviour with a guess is not allowed: this project has already
seen a wrong cause given for a failure that way (see "limit = -1" in the same file).

**When DSM refuses something, read its own logs on the NAS before theorising.**
Over SSH: `/var/log/synopkgmgr.log` carries the verdict of every package check
with a code and an English description, `/var/log/synopkg.log` the install
step by step. Both are `system:log`, mode 660 — root only, and the Web API
does not expose them: `SYNO.Core.SyslogClient.Log` shows "package installed"
and nothing about a refusal. The message in Package Center is written for
somebody else's problem and names neither the stage nor the reason. A month
went into guessing why an upgrade failed while one line of that log named the
cause from the first attempt (see "preupgrade / postupgrade" in
`docs/synology-api.md`).

Known traps that specific lines of code exist for:

- Batch actions answer in **three different shapes**, and in two of them a
  refusal arrives under `success: true`. Failing to parse the array means
  showing the user success where the NAS did nothing.
- `limit = -1` works normally in Download Station and **takes File Station down**
  with HTTP 502 on every API version.
- Parameter encoding differs in three places: ordinary calls use JSON,
  `FileStation.Upload` takes no quotes, `DownloadStation2` create with a file is
  JSON again. With multipart, `_sid` goes **in the query string**.
- Paths: File Station wants a leading slash, Download Station does not.
- A task's files, trackers and peers (`Task.BT.*`) are available **only while the
  task is active**; a finished one answers with code 1913, and that is not a failure.
- **Task** priority applies to eMule, not to torrents: for BT the call is
  accepted but does nothing and cannot be read back. Priority of **files** inside
  a torrent is a working feature.
- When the API says nothing about what it can do, the answer is in the web UI
  code on the NAS: `/var/packages/<Package>/target/ui/*.js` shows which call DSM
  itself makes and with which parameters. Over HTTP the same files are at
  `/webman/3rdparty/<Package>/`, and the package's `config` there names them —
  that is how the whole task-status table was read out of Download Station
  instead of being guessed one failing task at a time.
- **Task statuses come as numbers** in DownloadStation2, and the table is in
  `docs/synology-api.md`. Ours was shifted by one from code 7 on, so a task
  being unpacked (10) was shown as failed. Anything outside the table is an
  error — that is what Download Station's own `getStatusString` does with its
  default branch, and why a failure arrives as 101 with no string of its own.
- In `Task.edit` the parameter is called `id`, in `Task.BT.File` it is `task_id`.
- A trailing slash in the `CopyMove` destination path gives error 418.
- `SYNO.Core.System` does not return the host name — `FileStation.Info` has it.
- The level filter in the system log does not work: we filter on our side.
- `Thumb` and `Download` return bytes, and report an error by switching the
  Content-Type to JSON.

This file is kept up to date continuously: a new API finding, the cause of a
non-obvious failure or an architecture decision is written here in the same
session, together with the code.

## Tests

```bash
go test ./...                               # no NAS needed, must always pass
DSM_URL=… DSM_USER=… DSM_PASSWORD=… \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Integration tests run against a real NAS and are skipped by default. The ones
that change state need separate permission:

- `DSM_TEST_MUTATIONS=1` — creating and deleting tasks;
- `DSM_TEST_FOLDER=/Download` — the folder for file operations;
- `DSM_TEST_MAGNET`, `DSM_TEST_TORRENT` — your own sources instead of the built-in ones.

A test **must clean up** everything it created through `t.Cleanup`, even when it
fails. If the cleanup did not work — say so in the failure message and name what
has to be deleted by hand.

State-changing actions on the NAS are agreed with the user before they run —
test tasks and files included.

## Where things live

```
cmd/dsm-mini/        entry point
internal/dsm/        DSM Web API client: sessions, versions, multipart
  downloadstation/   tasks: DownloadStation2 and legacy behind one interface
  filestation/       files: browsing, upload, rename, delete
internal/httpapi/    REST for the Mini App, Telegram signature check
internal/bot/        the bot: links and .torrent files from chat, bot appearance from code
internal/watcher/    notifications about finished tasks
internal/dsm/system/     load, device details, packages, log
internal/dsm/storage/    disks, pools and volumes
internal/dsm/vmm/        virtual machines
internal/dsm/containers/ Container Manager containers
internal/store/      user settings on top of SQLite
internal/db/         schema and migrations
web/                 the React Mini App, built into internal/web/dist
docs/synology-api.md what has been learned about the API in practice
```

## Security

The service is exposed to the internet and can delete files on the NAS.

- **Every request to `/api/`** is checked twice: the `initData` signature with a
  key derived from the bot token, and the user id against the allow list. The
  signature only proves that a person opened the bot — and anyone can open it.
- **An empty `ALLOWED_USER_IDS` closes access to everyone**, it does not open it.
- Secrets live only in `.env` (which is in `.gitignore`), mode `0600`. Neither
  the token nor the DSM password goes into the log — on a refusal the reason is
  logged, never the value.
- The outside gets a generic phrase, the details go to the log: a message saying
  exactly what did not match in the signature is a hint on how to forge it.
- For the NAS a **separate user** is created, with access only to the packages
  needed and without two-factor, not an administrator.

- **The project never holds anyone's bot token**, and that rules out one idea
  that keeps looking attractive: a "helper bot" that creates the user's bot for
  them. Telegram does support it — managed bots, `bots.createBot` over MTProto
  — but the manager bot can call `bots.exportBotToken` at any time afterwards,
  with a `revoke` flag that reissues the token and takes the bot away from its
  owner, and the documented methods offer **no way to detach a manager** once
  it is set. So the price of saving one trip to @BotFather is a service that
  permanently holds the keys to every user's bot. Considered and declined in
  September 2026; the installation wizard explains the BotFather step instead.

- **The service binds loopback only** (`127.0.0.1:8080`), because the DSM
  reverse proxy reaches it through localhost. `:8080` would also publish it to
  the whole local network — which is what the package actually did until
  September 2026, while the README claimed the opposite. A security claim in
  the documentation is worth checking against a live port, not against the
  intent of the person who wrote it.
- **Examples in documentation are invented, never copied from a live `.env`.**
  A real bot token once went into the README as the example token, spread into
  ten translations, nine wizard files and the compose block, and stayed there
  until the bot started getting spam. The only token allowed in the repository
  is `1234567890:AAExampleTokenReplaceThisWithYours0`; CI fails on anything
  else shaped like a token, in the "No live secrets" job. The same goes for
  real Telegram ids — `tools/shoot.js` had one.

Changes in this area are covered by tests in `internal/httpapi/auth_test.go`.

## Frontend

- State of screens that lead out into the NAS overview is **kept in `App`**:
  navigating unmounts the screen, and unsaved edits are lost. This has already
  broken the folder settings and the add screen twice. For the same reason the
  open folder of the Files tab, the selected downloads tab, the path typed by
  hand in the folder settings and the screen a person was interrupted on live
  there too: switching tabs would otherwise reset all of it.
- Adding a download, the folder settings and the task screen are **parts of the
  downloads section**, not separate places: the tab bar is not hidden on them,
  and coming back to the tab lands where the person left off. Tapping the active
  tab again goes back to the start of the section.
- Initial values that arrive as a prop (`initialPath`) are taken **once** through
  `useRef`: sending them back up from a handler makes the screen reload itself.
- A handler that writes and then navigates **waits for the write first**. The
  screens read their data when they mount, so switching first and saving after
  lands on a screen that fetched a moment too early: pinning a folder from the
  NAS browser showed "nothing pinned" until the screen was opened again, while
  the server already had it. Order: write, refresh, then `setScreen`.
- Settings are saved **immediately**, with no "Save" button: the interface
  updates before the server answers and rolls back on an error.
- Padding inside cards comes from the card's own `padding`, not from `margin` on
  its children: such a rule breaks at the very first wrapper.
- Class names are checked against the ones already taken: a card with the class
  `app` inherited the root container's styles and stretched across the screen,
  and an empty disk bay marked `empty` picked up the 44px padding of the big
  "nothing here" block and grew to twice the height of its neighbours. A
  modifier that belongs to one component is named after it — `bay free`, not
  `free`.
- Drive capacity is formatted with `diskSize` (decimal), everything else with
  `size` (binary): disks are sold in decimal, so a 1 TB drive has to read as
  "1 TB" and not as the 0.9 the binary arithmetic gives.
- The theme comes from Telegram's `themeParams`; confirmations and feedback are
  native (`showConfirm`, `HapticFeedback`), not the browser's. The theme and the
  height are re-read on the `themeChanged` and `viewportChanged` events: reading
  them once at startup is not enough.
- The app height is Telegram's `viewportStableHeight`, not `100dvh`: inside the
  client `dvh` equals the whole screen height, and in a collapsed app the tab bar
  slides below the bottom edge. `viewportHeight` will not do for this — it jumps
  around during gestures.
- Vertical swipes are disabled (`disableVerticalSwipes`, Bot API 7.7): otherwise
  a scroll gesture over a list collapses the app.
- New Telegram features are called only under `isVersionAtLeast`: the
  documentation does not promise that a call is safe on an old client. Outside
  Telegram the script stub reports version 6.0 — the checks cover that case too.
- About the safe area (Bot API 8.0) the documentation **says nothing** about
  whether `safeAreaInset` and `contentSafeAreaInset` add up or are nested. As
  long as full-screen mode is not used, it is not used either, and the insets
  come from `env(safe-area-inset-*)`. If it is ever wanted — check it on a live
  client first, rather than picking blindly.
- Until the data arrives the screen shows **placeholders**
  (`components/Skeleton`), not an empty list and not zeros in counters: "No
  machines" while machines are running is a plain lie. Placeholders are built
  from the same classes as the real cards, so the content takes their place
  without the layout jumping. The column with bars needs `sk-grow`: inside a
  `flex` with no width of its own, percentages are computed from zero and the
  bars collapse.
- Screens with lists from the NAS start from `localStorage` (`cache.ts`): on a
  second open there is a picture from the first frame, and placeholders appear
  only when there is genuinely nothing to show.
- `index.html` is served with `Cache-Control: no-cache`, files with a hash in the
  name for a year. Without this Telegram shows the old build until the app is
  restarted.

## Packaging and publishing

- There is **one way to install it: the DSM package.** Docker was dropped in
  September 2026 — a second path meant the same settings documented twice, in
  ten languages, and the two drifting apart. A new required setting is now
  edited in one place: the wizard table in `tools/make-wizard.py`.
- The `.gitignore` entry for the built binary is written with a leading slash
  (`/dsm-mini`): without it the pattern also matches the `cmd/dsm-mini`
  directory, and the entry point quietly drops out of the repository.
- The **`.spk` package** is built with plain tar (`tools/build-spk.sh`), without
  the Synology toolkit: the binary is static, a chroot with their environment is
  not needed. The file order in the archive follows spksrc: `package.tgz`,
  `INFO`, `scripts`, then the rest.
- `package` in INFO is the **identifier**, `displayname` is what people see.
  `/var/packages/dsm-mini`, the upgrade path and every API call go by the
  identifier, so it stays `dsm-mini` whatever the visible name becomes. The
  visible name lives in three places that have to agree: `displayname` in
  INFO, `dname` in the catalogue and `DISPLAY_NAME` in the spksrc recipe.
- The package ships **`scripts/preupgrade` and `scripts/postupgrade`**, and
  they may do nothing at all. Without them an upgrade from a package source
  is refused at the `prepare` stage with
  `{"code":261,"description":"package does not have preupgrade / postupgrade
  script"}` in `/var/log/synopkgmgr.log`, and the person is shown "Invalid
  file format, contact the package developer" — which says nothing about
  scripts. **Installing the same .spk works**, logging only a harmless
  `{"code":289,"description":"spk is not from synology"}`, so the gap is
  invisible until somebody presses Update. This cost a month and seven
  releases of guessing; the answer was in a log the Web API does not expose.
  The whole account, including what was wrongly blamed first, is in
  `docs/synology-api.md`.
- INFO carries **`checksum`, the md5 of `package.tgz`** because the developer
  guide documents it. It was **not** the cause of "Invalid file format",
  though this file said so for a while: 1.0.0-1 had no `checksum` at all and
  installed from a source without complaint.
- The version in INFO ends with a **build number**, pinned at `-0`. A Synology
  version is `major.minor.build-buildnumber`, and every package that installs
  from a source on a live DSM carries one; ours did while it was `1.0.0-1` and
  lost it when the suffix was dropped for looking like noise. `build-spk.sh`
  and `make-feed.py` both append it, because Package Center compares the
  catalogue's version against INFO's and the two have to be spelled the same
  way. The tag and the file name stay plain — nothing compares those.
- The `arch` values in INFO are **Synology platform names**, not processors
  (`epyc7002`, `rtd1296`). One package covers every model of the same
  architecture.
- DSM 7 runs package scripts as the package user, not as root (`conf/privilege`
  with `run-as: package`). Writing is only possible into the package's `var` —
  which is also what survives an upgrade, unlike `target`.
- The wizard asks **five** things, and every one of them is something only the
  person installing knows: the DSM user and password, the bot token, the list
  of ids and the public address. The DSM address and the service port used to
  be there and are not any more — the service runs on the NAS, so the address
  is always loopback, and 8080 is free unless somebody made it otherwise.
  `postinst` still writes both with its own defaults, and the settings screen
  inside DSM edits them for the installation where they are wrong. A question
  whose answer is the same for everyone is not a question.
- Each wizard field carries three things: a short label, an example shown in
  the empty field (`emptyText`) and a line underneath saying what the setting
  is for. Whoever fills this in has never seen the project and is being asked
  for a password and a token — "DSM user" alone does not say which user, and
  the list of allowed ids does not explain itself at all.
- The wizard runs **only on installation**. There is no `upgrade_uifile` and no
  upgrade script, so an update leaves `config.env` untouched — which is exactly
  why settings survive one. The consequence is easy to get wrong: installing
  the package over itself does **not** re-ask anything. Changing a setting
  means editing `config.env` over SSH, or uninstalling and installing again,
  which also takes the database next to it. The README said the opposite for a
  while, contradicting its own paragraph two lines below.
- Wizard settings arrive in `postinst` as environment variables named after the
  keys from `WIZARD_UIFILES`. They are written to the file in single quotes: a
  password with a space or a `$` would otherwise break the service start.
- The wizard is translated in separate `install_uifile_<language>` files; the
  suffixes are Synology's (`rus`, `ger`, `ptb`) and do not match Telegram's
  codes. **Italian is `ita`** — `itn` was used here once, and a suffix DSM does
  not know is not an error: the installer quietly shows English, so only an
  Italian speaker would ever notice. The full list is in `docs/synology-api.md`.
- The listing text lives in **`assets/store.json`**, once: `build-spk.sh` writes
  it into INFO as `description` and `description_<lang>`, `make-feed.py` puts
  the English one into the catalogue. One paragraph, no line breaks and no
  markup — of the 30 packages on a live DSM, Synology's own included, not one
  has a newline in its description.
- Screenshots for the listing come **only from the catalogue**, field
  `snapshot`; the `.spk` has no field for them. Package Center does not fetch
  them from the browser — the NAS does, through
  `SYNO.Core.Package.Screenshot.Server`, so the address has to be reachable
  from the NAS. `tools/make-store.py` composes the slides from the English
  screenshots into `docs/store/`, and `make-feed.py` publishes them.
- The catalogue for DSM "Package Sources" is static files on GitHub Pages, one
  per Synology platform name: Package Center expects a list **filtered by
  architecture**, and static files cannot filter. It is published on a tag; the
  `github-pages` environment has to allow `v*` tags, otherwise the job fails
  before its first step.
- Every catalogue entry carries **`md5` and `size` of the .spk**. Package Center
  reads both out of the feed and passes them to the backend as `checksum` and
  `filesize` — visible in `PkgManApp.js` on the NAS, in the
  `SYNO.Core.Package.Installation` `upgrade`/`install` call. `make-feed.py`
  takes them from the file itself and refuses to build a catalogue without it,
  so an entry cannot describe a package that is not there.
- The `link` in the catalogue points at a host that **does not compress** what
  it serves. GitHub Pages does: asked with `Accept-Encoding: gzip` it returns
  the .spk as a gzip stream, and no `md5` in the catalogue can describe both
  that and the file. Release assets are served as they are, and the redirect
  they answer with is harmless. `make-feed.py --verify` downloads its own link
  and refuses to publish a catalogue that describes something else. This was
  also blamed for "Invalid file format" and was **not** the cause.
- The package is built with `COPYFILE_DISABLE=1` and `--format=ustar`. macOS
  tar writes an AppleDouble companion beside every file (`._INFO` next to
  `INFO`) and hides them again when listing the archive, so a package built on
  a Mac had twenty junk entries and started with `._package.tgz` while
  `tar tvf` showed it clean. `build-spk.sh` reads the finished archive back
  with Python and fails if the junk returns or the first entry is not
  `package.tgz`.
- The catalogue is built from the **.spk files of the release**, downloaded back
  with `gh release download`, not from a fresh build: `md5` has to describe the
  exact file people will get, and a rebuild from a branch that moved on since
  the tag is a different file.
- Every icon in the repository is rendered from **`assets/icon.svg`** by
  `tools/make-icons.sh` — the package icons, the one in the SynoCommunity
  recipe, the Mini App favicon and the one in the READMEs. The PNGs are
  committed because none of the places that need them can run a renderer, so
  the rule is: change the SVG, run the script, never touch a PNG by hand.
  The arrow in the mark is a cut-out, not a white shape on top: at 40 px, the
  size Package Center and Telegram really show, a thin outline disappears.
- The wizard files in `spk/WIZARD_UIFILES/` are generated and committed; CI
  regenerates them and fails if the result differs. The same goes for the ten
  READMEs: a check compares the language links, because a translation nothing
  links to looks perfectly fine on its own.
- The recipe for SynoCommunity is in `contrib/spksrc/`: they build from source
  with their own toolchain, our `build-spk.sh` does not suit them. On a release
  the version numbers and checksums are updated there.

## The settings screen inside DSM

The package registers a window in the DSM main menu: `dsmuidir="ui"` and
`dsmappname` in INFO, `spk/ui/config` describing the app, and a thin Ext JS
wrapper (`dsm-wrapper.js`) whose only job is an iframe. Everything a person
sees is a plain page — no framework, no build step, because it is served
straight out of the package by DSM's own web server.

- **A browser session needs `X-SYNO-TOKEN`; an API session does not.** DSM has
  CSRF protection on by default (`SYNO.Core.Security.DSM`,
  `enable_csrf_protection`), and with it the session cookie alone is refused —
  with a plain `success: false`, not an HTTP error. A session made through
  `SYNO.API.Auth` is not refused, so the check passed every test and failed for
  the first person who opened the screen in DSM. The page reads the token from
  `SYNO.SDS.Session.SynoToken` on the desktop around it — same origin — rather
  than taking it through a URL, where it would land in logs and referrers.
- **`/webman/3rdparty/` is served to anyone.** A request with no session gets
  200, checked against a live NAS. So the path guards nothing and `api.cgi`
  authorises nothing: it only carries the browser's `Cookie` header to the
  service, which asks DSM whose session it is. The call is
  `SYNO.Core.Desktop.Initdata`, whose `Session` block has `user` and
  `is_admin`; there is no documented "who am I", and `SYNO.Core.CurrentUser`
  answers 102. Anything less than an administrator is refused — the screen
  edits the bot token and the NAS password.
- **The CGI cannot read `config.env`**: the file is 0600 and owned by the
  package user, and DSM's web server runs as another. It still needs the
  service's port, so `start-stop-status` writes that one value into
  `ui/backend.conf`, which is world-readable and holds nothing else.
- **Secrets are write-only.** The password and the token are never sent to the
  browser; the screen shows "set" and an empty field. An empty field means
  "leave it alone" — treating it as "erase" would lock the service out of the
  NAS on the first save of an unrelated setting.
- `config.env` is rewritten through a temporary file and values are quoted the
  way `postinst` writes them: a half-written file is a service that will not
  start, and an unquoted password with a space breaks the start script.
- Everything except the notification mode needs a restart to take effect: the
  service reads `config.env` once. The screen says so rather than pretending.
- The dictionary is generated by `tools/make-dsmui-i18n.py` and committed, the
  same arrangement as the installer wizard; CI fails if the two drift apart.

## Languages

The interface and the bot are translated into ten languages; the language comes
from the Telegram user's `language_code`, an unknown one gets English.

- Dictionaries: `web/src/i18n/<code>.ts` (interface) and `internal/i18n/<code>.go`
  (bot, notifications, API errors). English is the source of truth in both.
- **A new string is added to every dictionary at once.** On the frontend the type
  enforces it (`Record<Key, Phrase>`), on the server the `internal/i18n` test
  does: it compares keys, `{name}` substitutions and the absence of a foreign
  script — a scrap of Russian text inside a Portuguese string would otherwise
  only be noticed by a user.
- Plurals on the frontend go through `Intl.PluralRules` (the `one`, `few`, `many`
  forms); Go has nothing like it, so the messages are written with the number
  after a colon ("Active tasks: 5") and need no agreement.
- Numbers and dates are formatted through `Intl` with a locale tag, not by hand:
  the decimal separator and the date order differ everywhere.
- API error messages travel as a **key**, not as text: `s.fail(w, r, err,
  "api.readFolder")`. The person gets a translation, the log stays English, and
  the DSM code goes in a separate field — digits read the same in any language.
- The bot's appearance (descriptions, commands, menu button) is set for every
  language and **only when it changes**: across ten languages that is a couple
  of dozen requests otherwise.
- **The name is not set from code.** It belongs to whoever created the bot, and
  writing it on every start silently undid a rename made in @BotFather.
- **The avatar is set only when there is none.** `setMyProfilePhoto` arrived in
  Bot API 9.4 (February 2026) — before that it really was a trip to @BotFather,
  and a comment in this repository said so for a while after it stopped being
  true. The current picture is read with `getUserProfilePhotos` on the bot's
  own id: the documentation describes that method for users, but it answers for
  a bot too — checked against the live API, both branches. An empty profile
  gets `internal/bot/avatar.png`, a picture the owner chose is left alone.

## Build and run

```bash
cd web && npm install && npm run build   # Mini App → internal/web/dist
cd .. && go build ./cmd/dsm-mini         # a single binary with the frontend embedded
```

The SQLite driver is `modernc.org/sqlite`, pure Go. Swapping it for
`mattn/go-sqlite3` is not allowed: that one needs CGO, and with CGO the static
binary the package ships is gone — and with it the reason `build-spk.sh` needs
no Synology toolchain.

## Language of the project

**There is no Russian in the code.** Comments, names, log messages and error
text are in English: the project is open, and not only Russian speakers will
read it.

Russian lives in exactly three places, and all of them are **translations**:
`internal/i18n/ru.go`, `web/src/i18n/ru.ts`,
`spk/WIZARD_UIFILES/install_uifile_rus` (built by `tools/make-wizard.py`, which
holds the text table) plus `DESCRIPTION_RUS` in the SynoCommunity recipe. The
other nine languages live in the same places.

Documentation follows the same rule: `README.md` and this file are English,
translations of the README are separate files (`README_RU.md` and the rest), and
the language links at the top of every one of them are kept in step.

Commit messages stay in Russian: the history has been written that way from the
first commit, and mixing the two would make it unreadable either way.

Text for people is written for people, not for developers: "the folder does not
exist" instead of "code 403" — in any language.
