<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · [Deutsch](README_DE.md) · [Français](README_FR.md) · **Italiano** · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Controlli](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![Licenza MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Un bot Telegram con Mini App per governare un NAS Synology di casa: i download di
Download Station e i file di File Station direttamente dalla messaggistica, senza
VPN e senza l'interfaccia web di DSM.

Butti un link magnet in chat — il bot chiede con dei pulsanti dove metterlo e lo
mette in coda. Apri l'applicazione — e vedi cosa sta scaricando, quanto manca,
cosa c'è sui dischi e come sta il NAS.

<p align="center">
  <img src="docs/screenshots/it-home.webp" width="19%" alt="Panoramica del NAS">
  <img src="docs/screenshots/it-downloads.webp" width="19%" alt="Download">
  <img src="docs/screenshots/it-task.webp" width="19%" alt="Un'attività">
  <img src="docs/screenshots/it-files.webp" width="19%" alt="File">
  <img src="docs/screenshots/it-storage.webp" width="19%" alt="Archiviazione">
</p>

> Funzionano: il bot, la Mini App e le notifiche. Serve DSM 7 o più recente: su
> DSM 6 il pacchetto non si installa e lì non l'ha provato nessuno. Verificato
> su DSM 7.2.2 con Download Station 4.1.2 e File Station 1.4.4.

## Cosa sa fare

**Download**

- L'elenco delle attività: avanzamento, velocità, tempo rimasto, seed e peer
- Mettere in pausa, riprendere, eliminare
- Aggiunta con link magnet, con link diretto e con file `.torrent`
- Scelta della cartella di destinazione, con un elenco di accesso rapido
  configurabile
- Scelta dei file dentro un torrent e della loro priorità
- Un messaggio in chat quando un'attività finisce o fallisce

**File**

- Sfogliare le cartelle, anteprime, caricamento sul NAS
- Rinominare, copiare, spostare, eliminare

**Stato del NAS**

- Carico di processore e memoria, tempo di attività, rete
- Dischi, pool e volumi: temperatura, spazio occupato, salute
- Macchine virtuali e contenitori: avvio e arresto
- Il registro eventi di DSM

**Lingua**

L'applicazione e il bot parlano la lingua scelta in Telegram: inglese, russo,
spagnolo, portoghese, tedesco, francese, italiano, turco, ucraino, polacco. Una
lingua sconosciuta riceve l'inglese.

---

## Installazione

Quello che segue va passo per passo. Non serve sapere niente in anticipo, ma
mettiti da parte mezz'ora: il grosso se ne va nel certificato, non nel servizio.

### Cosa serve

- **Un NAS Synology** con DSM 7. Non c'è niente da installare prima: il servizio
  arriva come pacchetto DSM e gira sul NAS stesso.
- **Download Station** — installalo dal Centro pacchetti se non c'è ancora.
- **Telegram** sul telefono.
- **L'accesso al router** — servirà inoltrare due porte.

> **Perché serve un indirizzo pubblico.** Telegram apre una Mini App solo via
> `https://` con un certificato vero. Uno autofirmato, un `192.168.…` locale e un
> indirizzo tipo `nas:5001` non vanno bene: l'applicazione semplicemente non si
> apre. Il bot invece funziona anche senza indirizzo — solo senza il pulsante.

---

### Passo 1. Creare il bot

1. Apri [@BotFather](https://t.me/BotFather) in Telegram e premi **Start**.
2. Manda `/newbot`.
3. Scrivi il **nome** del bot — uno qualsiasi, è quello che si vede in cima alla
   chat. Per esempio: `Il mio NAS`.
4. Scrivi il **nome utente** del bot — in lettere latine e che finisca
   obbligatoriamente in `bot`. Per esempio: `alex_home_nas_bot`. Se è occupato,
   BotFather ne chiede un altro.
5. In risposta arriva una riga tipo
   `1234567890:AAExampleTokenReplaceThisWithYours0`. Quello è il **token**.
   Copialo: serve al passo 5.

> Il token è la password del bot. Chi ce l'ha comanda il bot. Non pubblicarlo in
> chat né su GitHub.

### Passo 2. Scoprire il proprio ID Telegram

È il numero da cui il servizio capisce che stai scrivendo tu e non un estraneo.

1. Apri [@userinfobot](https://t.me/userinfobot) e premi **Start**.
2. Risponde con un numero sulla riga `Id`, per esempio `123456789`. Segnatelo.

### Passo 3. Creare un utente a parte sul NAS

Il servizio sa cancellare file, quindi dargli un amministratore è una pessima
idea.

1. In DSM: **Pannello di controllo → Utente e gruppo → Utente → Crea**.
2. Nome: `dsm-mini`. La password lunga e casuale; segnatela.
3. **Non attivare la verifica in due passaggi.** Il codice usa e getta non si può
   prendere da nessuna parte e l'accesso semplicemente non passa.
4. Gruppi: lascia `users`.
5. Cartelle condivise: dai accesso **solo** a quelle in cui scaricherai (di
   solito `download` o `Media`). Alle altre «Nessun accesso».
6. Applicazioni: consenti **Download Station** e **File Station**, nega il resto.

> Se al passo 5 nel registro compare `authentication with DSM failed`, torna qui
> e consenti a questo utente anche l'applicazione **DSM**: su certe versioni
> l'accesso non passa senza, nemmeno via API.

### Passo 4. Ottenere un indirizzo e un certificato

Se hai già un dominio con un certificato valido sul NAS, salta il passo.

1. **Il nome.** Pannello di controllo → **Accesso esterno → DDNS → Aggiungi**.
   Fornitore `Synology`, nome host uno qualsiasi libero, per esempio `alex-nas`.
   Viene fuori l'indirizzo `alex-nas.synology.me`. Salva.
2. **Le porte sul router.** Nelle impostazioni del router inoltra la **porta 80**
   e la **porta 443** all'indirizzo interno del NAS. Senza la 80 il certificato
   non verrà emesso, senza la 443 l'applicazione non si apre.
3. **Il certificato.** Pannello di controllo → **Sicurezza → Certificato →
   Aggiungi → Ottieni un certificato da Let's Encrypt**. Il nome di dominio è
   proprio `alex-nas.synology.me`, l'e-mail la tua. L'emissione richiede un
   minuto.

Controlla: apri `https://alex-nas.synology.me:5001` dal telefono con la rete
mobile (non col Wi-Fi di casa). DSM deve aprirsi senza avvisi sul certificato.

### Passo 5. Installare il pacchetto

La via più facile è **aggiungere un'origine pacchetti**, così installazione e
aggiornamenti passano direttamente dal Centro pacchetti:

1. **Centro pacchetti → Impostazioni → Origini pacchetti → Aggiungi**.
2. Nome: `dsm-mini`. L'indirizzo secondo la tua architettura:
   - Intel e AMD (la maggior parte dei modelli): `https://alexlnos.github.io/dsm-mini/amd64.json`
   - ARM (modelli economici): `https://alexlnos.github.io/dsm-mini/arm64.json`
3. Consenti i pacchetti di terze parti: **Impostazioni → Generale → Livello di
   attendibilità → Qualsiasi editore**.
4. A sinistra compare la sezione **Community**, e dentro `dsm-mini`. Premi
   Installa — poi la procedura guidata chiede le impostazioni.

Non conosci la tua architettura — prova `amd64`: un pacchetto che non va bene DSM
si rifiuta semplicemente di installarlo, così non si rompe niente.

**Oppure a mano, senza origine:**

1. Scarica lo `.spk` dalla pagina delle
   [versioni](https://github.com/alexlnos/dsm-mini/releases): `-amd64` per i
   modelli Intel e AMD (DS918+, DS923+, DS1522+, SA6400 e simili), `-arm64` per
   quelli economici su ARM (DS223, DS124). Nel dubbio prendi `amd64`: un
   pacchetto che non va bene DSM si rifiuta semplicemente di installarlo.
2. **Centro pacchetti → Installazione manuale → Sfoglia** e scegli il file
   scaricato.
3. DSM dirà che l'editore è sconosciuto. È normale per un pacchetto di terze
   parti: consentilo una volta in **Centro pacchetti → Impostazioni → Generale →
   Livello di attendibilità → Qualsiasi editore**.

#### Cosa chiede la procedura guidata

L'installer ha due schermate e sette campi. Tutto quello che serve è stato
raccolto nei passi da 1 a 4.

| Campo | Cosa mettere |
|---|---|
| Indirizzo DSM | Già compilato: `https://localhost:5001`. Il servizio gira sul NAS stesso, quindi lascialo così |
| Utente DSM | Il nome dell'utente del passo 3, per esempio `dsm-mini` |
| Password DSM | La password di quell'utente |
| Token del bot | Il token del passo 1 |
| ID Telegram consentiti | Il tuo numero del passo 2. Più persone: separate da virgole |
| Indirizzo pubblico HTTPS | Il tuo indirizzo del passo 4, per esempio `https://alex-nas.synology.me` |
| Porta locale | Lascia `8080`. Cambiala solo se sul NAS quella porta è già occupata |

**Gli errori che si fanno davvero qui:**

| Scritto | Giusto |
|---|---|
| `alex-nas.synology.me` | `https://alex-nas.synology.me` — con il protocollo |
| `https://alex-nas.synology.me/` | senza barra alla fine |
| Un elenco di ID vuoto | vuoto vuol dire **nessuno**; metti il tuo numero |
| Il tuo indirizzo pubblico nel campo dell'indirizzo DSM | l'indirizzo DSM resta `https://localhost:5001` |
| Un codice 2FA usa e getta al posto della password | l'account non deve avere affatto la verifica in due passaggi (passo 3) |

Dopo l'installazione il pacchetto parte da solo e si rialza insieme al NAS. Le
impostazioni stanno in `/var/packages/dsm-mini/var/config.env` (permessi `600`),
e il registro lì accanto, in `dsm-mini.log`.

Se il servizio non riesce a partire — token sbagliato, password sbagliata, rete
assente — lo dice nel **centro notifiche di DSM**, con il motivo. Il registro
completo è nel Centro pacchetti, sulla pagina del pacchetto.

### Passo 6. Puntare l'indirizzo sul servizio

Adesso il servizio ascolta solo dentro il NAS, sulla porta 8080. Il proxy inverso
riceve le richieste da internet via HTTPS e gliele passa.

1. **Pannello di controllo → Portale di accesso → Avanzate → Proxy inverso →
   Crea**.
   (In DSM 7.0–7.1 è **Pannello di controllo → Portale applicazioni → Proxy
   inverso**.)
2. **Origine**: protocollo `HTTPS`, nome host `alex-nas.synology.me`, porta
   `443`.
3. **Destinazione**: protocollo `HTTP`, nome host `localhost`, porta `8080`.
4. Salva.

> **Non passare per il proxy la porta 80 di questo nome**: è da lì che DSM
> rinnova il certificato Let's Encrypt, e intercettarla rompe il rinnovo tre mesi
> dopo.

### Passo 7. Verificare

Apri `https://alex-nas.synology.me/healthz` in un browser. Deve rispondere:

```json
{"status":"ok"}
```

Ha risposto: il servizio è vivo e raggiungibile da fuori. E intanto non dà via
nessun dato: qualsiasi richiesta senza firma di Telegram viene rifiutata.

### Passo 8. Aprire l'applicazione

1. Trova il tuo bot in Telegram con il nome utente del passo 1.
2. Premi **Start**.
3. In basso, accanto al campo di scrittura, compare un pulsante **Download** che
   apre l'applicazione. Il bot lo mette da solo all'avvio, non c'è niente da
   configurare a mano.
4. Manda al bot un link magnet qualsiasi — ti propone le cartelle con dei
   pulsanti.

Fatto.

---

## Se qualcosa è andato storto

| Cosa vedi | Di cosa si tratta | Cosa fare |
|---|---|---|
| Il bot tace su `/start` | Token sbagliato, o il pacchetto non è in funzione | Centro pacchetti → `dsm-mini` → il registro |
| «L'accesso a questo bot è chiuso» | Il tuo ID non è nell'elenco | Metti il numero del passo 2 negli ID consentiti (vedi «Cambiare le impostazioni» più sotto) |
| Non c'è il pulsante dell'applicazione | L'indirizzo pubblico è vuoto o non è `https://` | Stesso posto: il file delle impostazioni, poi riavvia il pacchetto |
| Il pulsante c'è, l'applicazione non si apre | Il proxy inverso o il certificato non funzionano | Apri `https://tuo-indirizzo/healthz` in un browser |
| «Apri l'applicazione dal bot» | L'applicazione è stata aperta con un link diretto nel browser | È voluto: aprila dal bot |
| «Accesso negato: il tuo ID Telegram…» | Il servizio non ti ha riconosciuto | Negli ID consentiti solo cifre, separate da virgole |
| `authentication with DSM failed` nel registro | La password, la 2FA o i diritti dell'utente | Passo 3: password senza errori di battitura, 2FA spenta, applicazioni consentite |
| `Could not get the task list` nel registro | Download Station non è installato, o è negato all'utente | Centro pacchetti e i diritti del passo 3 |
| Il pacchetto si ferma subito dopo l'avvio | Un'impostazione è sbagliata — il registro dice quale | Il centro notifiche di DSM riporta anche il motivo |

Il registro del pacchetto è la fonte di verità principale: dice esattamente cosa
manca. Sta in `/var/packages/dsm-mini/var/dsm-mini.log` e si apre dal Centro
pacchetti. Il registro è in inglese, l'interfaccia e i messaggi del bot nella tua
lingua.

### Cambiare le impostazioni

Tutto quello che ha chiesto la procedura guidata sta in un file sul NAS,
`/var/packages/dsm-mini/var/config.env`, permessi `600`.

Installare il pacchetto sopra sé stesso **non** chiede di nuovo: la procedura
guidata gira all'installazione, e un aggiornamento lascia il file apposta — è
per questo che le impostazioni sopravvivono. Restano due strade:

- **Modificare il file via SSH** (Pannello di controllo → Terminale e SNMP →
  attivare SSH) e riavviare il pacchetto dal Centro pacchetti. Così resta
  tutto il resto:

  ```bash
  sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='il-tuo-nuovo-token'|" /var/packages/dsm-mini/var/config.env
  sudo synopkg restart dsm-mini
  ```

- **Disinstallare e installare di nuovo**: la procedura guidata richiede tutto
  daccapo. Insieme alle impostazioni sparisce il database lì accanto: cartelle
  fissate, lingua e ultimi stati noti delle attività.

## Aggiornare

Se l'origine pacchetti è stata aggiunta, il Centro pacchetti mostra
l'aggiornamento da solo. Senza origine, scarica lo `.spk` fresco dalle
[versioni](https://github.com/alexlnos/dsm-mini/releases) e installalo sopra.

Le impostazioni e il database restano in entrambi i casi: stanno nella cartella
`var` del pacchetto, che un aggiornamento non tocca.

## Dove sono tenuti i dati

Tutto sta in `/var/packages/dsm-mini/var/`:

- `config.env` — quello che ha chiesto la procedura guidata, permessi `600`;
- `dsm-mini.db` — un database SQLite: le cartelle fissate, la lingua e gli ultimi
  stati noti delle attività, da cui il servizio capisce di cosa ha già riferito;
- `dsm-mini.log` — il registro.

Un aggiornamento del pacchetto li mantiene tutti e tre. La disinstallazione li
cancella.

## Sicurezza

Il servizio è esposto a internet e sa cancellare file sul NAS, perciò:

- **Un utente DSM a parte**, non un amministratore (passo 3).
- **Senza verifica in due passaggi** su quell'account: un codice usa e getta non
  può funzionare da un file di configurazione.
- **Gli ID consentiti sono una lista di ammessi.** Una lista vuota chiude
  l'accesso a tutti, non lo apre.
- Ogni richiesta a `/api/` viene controllata due volte: la firma `initData` con
  una chiave ricavata dal token del bot, e l'identificatore contro l'elenco. La
  firma dimostra soltanto che qualcuno ha aperto il bot — e aprirlo può chiunque.
- Verso l'esterno va una frase generica, i dettagli nel registro: un messaggio
  che dicesse cosa esattamente non tornava nella firma sarebbe un suggerimento su
  come falsificarla.
- Il token del bot e la password DSM vivono solo in `config.env` sul NAS stesso,
  permessi `600`, e non finiscono mai nel registro: in un rifiuto si scrive il
  motivo, mai il valore.
- Il servizio ascolta solo su `127.0.0.1`, cioè dal NAS stesso. Tutto quello che
  arriva da fuori passa dal proxy inverso di DSM, che chiude anche il TLS.

## Sviluppo

```bash
cd web && npm install && npm run build   # la Mini App finisce in internal/web/dist
cd .. && go build ./cmd/dsm-mini         # un binario con l'applicazione incorporata
go test ./...
```

Il frontend da solo, con ricaricamento automatico:

```bash
cd web && npm run dev    # parla col backend su localhost:8080
```

Avviare il backend in locale legge le stesse variabili che la procedura guidata
del pacchetto scrive in `config.env`. È comodo tenerle in un file:

```bash
cp .env.example .env     # compilalo
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

L'interfaccia compilata sta nel repository (`internal/web/dist`) — è da lì che la
prende `go:embed`. Dopo aver toccato il frontend, ricompila e fai il commit del
risultato, altrimenti i controlli non passano.

I test di integrazione girano contro un NAS vero e di default vengono saltati:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

I test che cambiano lo stato del NAS richiedono un permesso a parte:
`DSM_TEST_MUTATIONS=1`, e per le operazioni sui file anche `DSM_TEST_FOLDER` — la
cartella dentro cui si possono creare file temporanei. Cancellano tutto quello
che creano.

Una nuova lingua dell'interfaccia è un file di dizionario per parte:
`web/src/i18n/<codice>.ts` e `internal/i18n/<codice>.go`. Saltare una stringa non
si può: sul frontend ci pensa il tipo, sul server un test.

Codice, commenti e registro nel progetto sono in inglese; le altre lingue vivono
solo nei dizionari. I dettagli sono in [CLAUDE.md](CLAUDE.md).

## Particolarità dell'API Synology

La Web API di Synology a tratti si comporta diversamente da come sta scritto
nella documentazione: la stessa azione risponde in tre formati diversi,
`limit = -1` stende File Station, e il `_sid` durante il caricamento di un file va
passato in modo diverso che altrove. Tutto quello che è emerso su un NAS vero è
raccolto in [docs/synology-api.md](docs/synology-api.md).

## Licenza

[MIT](LICENSE)
