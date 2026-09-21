# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · [Deutsch](README_DE.md) · **Français** · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Vérifications](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![Licence MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Un bot Telegram avec Mini App pour piloter un NAS Synology domestique : les
téléchargements de Download Station et les fichiers de File Station directement
depuis la messagerie, sans VPN et sans l'interface web de DSM.

Vous lâchez un lien magnet dans la discussion — le bot demande avec des boutons
où le mettre et le met en file. Vous ouvrez l'application — et vous voyez ce qui
se télécharge, ce qu'il reste, ce qu'il y a sur les disques et comment se porte
le NAS.

<p align="center">
  <img src="docs/screenshots/fr-home.webp" width="19%" alt="Aperçu du NAS">
  <img src="docs/screenshots/fr-downloads.webp" width="19%" alt="Téléchargements">
  <img src="docs/screenshots/fr-task.webp" width="19%" alt="Une tâche">
  <img src="docs/screenshots/fr-files.webp" width="19%" alt="Fichiers">
  <img src="docs/screenshots/fr-storage.webp" width="19%" alt="Stockage">
</p>

> Ce qui marche : le bot, la Mini App et les notifications. Vérifié sur DSM 7.2.2
> avec Download Station 4.1.2 et File Station 1.4.4. La prise en charge de DSM 6
> est écrite d'après la documentation, mais n'a pas été vérifiée sur un vrai
> DSM 6.

## Ce qu'il sait faire

**Téléchargements**

- La liste des tâches : progression, vitesse, temps restant, sources et pairs
- Mettre en pause, reprendre, supprimer
- Ajout par lien magnet, par lien direct et par fichier `.torrent`
- Choix du dossier de destination, avec une liste d'accès rapide configurable
- Choix des fichiers à l'intérieur d'un torrent et de leur priorité
- Un message dans la discussion quand une tâche se termine ou échoue

**Fichiers**

- Parcourir les dossiers, aperçus, envoi vers le NAS
- Renommer, copier, déplacer, supprimer

**État du NAS**

- Charge processeur et mémoire, temps de fonctionnement, réseau
- Disques, groupes et volumes : température, espace occupé, santé
- Machines virtuelles et conteneurs : démarrage et arrêt
- Le journal des événements DSM

**Langue**

L'application et le bot parlent la langue choisie dans Telegram : anglais, russe,
espagnol, portugais, allemand, français, italien, turc, ukrainien, polonais. Une
langue inconnue reçoit l'anglais.

---

## Installation

La suite se fait pas à pas. Il n'y a rien à savoir d'avance, mais prévoyez une
demi-heure : l'essentiel passe dans le certificat, pas dans le service lui-même.

### Ce qu'il vous faut

- **Un NAS Synology** avec DSM 7. Rien à installer au préalable : le service
  arrive sous forme de paquet DSM et tourne sur le NAS lui-même.
- **Download Station** — installez-le depuis le Centre de paquets s'il n'y est
  pas encore.
- **Telegram** sur le téléphone.
- **L'accès au routeur** — il faudra rediriger deux ports.

> **Pourquoi une adresse publique est nécessaire.** Telegram n'ouvre une Mini App
> qu'en `https://` avec un vrai certificat. Un certificat auto-signé, un
> `192.168.…` local et une adresse du genre `nas:5001` ne conviennent pas :
> l'application ne s'ouvrira tout simplement pas. Le bot, lui, fonctionne sans
> adresse — mais sans le bouton.

---

### Étape 1. Créer le bot

1. Ouvrez [@BotFather](https://t.me/BotFather) dans Telegram et appuyez sur
   **Start**.
2. Envoyez `/newbot`.
3. Saisissez le **nom** du bot — n'importe lequel, c'est ce qu'on voit en tête de
   la discussion. Par exemple : `Mon NAS`.
4. Saisissez l'**identifiant** du bot — en lettres latines et se terminant
   obligatoirement par `bot`. Par exemple : `alex_home_nas_bot`. S'il est pris,
   BotFather en demande un autre.
5. La réponse est une ligne du genre
   `8929377165:AAFHfZfqKDUEFdv-Yq4TJ9etz4Pp-yBU3Vg`. C'est le **jeton**.
   Copiez-le — il servira à l'étape 5.

> Le jeton est le mot de passe du bot. Qui l'a, commande le bot. Ne le publiez
> pas dans des discussions ni sur GitHub.

### Étape 2. Connaître son identifiant Telegram

C'est le nombre grâce auquel le service comprend que c'est bien vous qui écrivez
et pas un inconnu.

1. Ouvrez [@userinfobot](https://t.me/userinfobot) et appuyez sur **Start**.
2. Il répond par un nombre sur la ligne `Id`, par exemple `123456789`. Notez-le.

### Étape 3. Créer un utilisateur à part sur le NAS

Le service sait supprimer des fichiers, lui donner un administrateur est donc une
mauvaise idée.

1. Dans DSM : **Panneau de configuration → Utilisateur et groupe → Utilisateur →
   Créer**.
2. Nom : `dsm-mini`. Le mot de passe long et aléatoire ; notez-le.
3. **N'activez pas la vérification en deux étapes.** Le code à usage unique n'a
   nulle part où être pris, et la connexion ne passera tout simplement pas.
4. Groupes : laissez `users`.
5. Dossiers partagés : donnez l'accès **uniquement** à ceux où vous
   téléchargerez (d'ordinaire `download` ou `Media`). Aux autres, « Aucun accès ».
6. Applications : autorisez **Download Station** et **File Station**, refusez le
   reste.

> Si à l'étape 5 le journal affiche `authentication with DSM failed`, revenez ici
> et autorisez aussi à cet utilisateur l'application **DSM** : sur certaines
> versions la connexion ne passe pas sans elle, même par l'API.

### Étape 4. Obtenir une adresse et un certificat

Si vous avez déjà un domaine avec un certificat valide sur le NAS, passez
l'étape.

1. **Le nom.** Panneau de configuration → **Accès externe → DDNS → Ajouter**.
   Fournisseur `Synology`, nom d'hôte n'importe lequel de libre, par exemple
   `alex-nas`. On obtient l'adresse `alex-nas.synology.me`. Enregistrez.
2. **Les ports sur le routeur.** Dans les réglages du routeur, redirigez le
   **port 80** et le **port 443** vers l'adresse interne du NAS. Sans le 80 le
   certificat ne sera pas délivré, sans le 443 l'application ne s'ouvrira pas.
3. **Le certificat.** Panneau de configuration → **Sécurité → Certificat →
   Ajouter → Obtenir un certificat de Let's Encrypt**. Le nom de domaine est ce
   même `alex-nas.synology.me`, l'adresse e-mail la vôtre. La délivrance prend
   une minute.

Vérifiez : ouvrez `https://alex-nas.synology.me:5001` depuis le téléphone en
données mobiles (pas par le Wi-Fi de la maison). DSM doit s'ouvrir sans
avertissement de certificat.

### Étape 5. Installer le paquet

Le plus simple est d'**ajouter une source de paquets**, ainsi l'installation et
les mises à jour passent directement par le Centre de paquets :

1. **Centre de paquets → Paramètres → Sources de paquets → Ajouter**.
2. Nom : `dsm-mini`. L'adresse dépend de votre architecture :
   - Intel et AMD (la plupart des modèles) : `https://alexlnos.github.io/dsm-mini/amd64.json`
   - ARM (modèles d'entrée de gamme) : `https://alexlnos.github.io/dsm-mini/arm64.json`
3. Autorisez les paquets tiers : **Paramètres → Général → Niveau de confiance →
   N'importe quel éditeur**.
4. À gauche apparaît la section **Communauté**, et dedans `dsm-mini`. Appuyez sur
   Installer — l'assistant demandera ensuite les réglages.

Vous ne connaissez pas votre architecture — essayez `amd64` : un paquet qui ne
convient pas, DSM refuse simplement de l'installer, rien ne peut être cassé
comme ça.

**Ou à la main, sans source :**

1. Téléchargez le `.spk` depuis la page des
   [versions](https://github.com/alexlnos/dsm-mini/releases) : `-amd64` pour les
   modèles Intel et AMD (DS918+, DS923+, DS1522+, SA6400 et semblables),
   `-arm64` pour ceux d'entrée de gamme en ARM (DS223, DS124). Dans le doute,
   prenez `amd64` : un paquet qui ne convient pas, DSM refuse simplement de
   l'installer.
2. **Centre de paquets → Installation manuelle → Parcourir** et choisissez le
   fichier téléchargé.
3. DSM dira que l'éditeur est inconnu. C'est normal pour un paquet tiers :
   autorisez une fois dans **Centre de paquets → Paramètres → Général → Niveau
   de confiance → N'importe quel éditeur**.

#### Ce que demande l'assistant

L'installateur a deux écrans et sept champs. Tout ce qu'il leur faut a été
rassemblé aux étapes 1 à 4.

| Champ | Quoi mettre |
|---|---|
| Adresse DSM | Déjà remplie : `https://localhost:5001`. Le service tourne sur le NAS lui-même, laissez-la telle quelle |
| Utilisateur DSM | Le nom de l'utilisateur de l'étape 3, par exemple `dsm-mini` |
| Mot de passe DSM | Le mot de passe de cet utilisateur |
| Jeton du bot | Le jeton de l'étape 1 |
| Identifiants Telegram autorisés | Votre numéro de l'étape 2. Plusieurs personnes : séparées par des virgules |
| Adresse publique HTTPS | Votre adresse de l'étape 4, par exemple `https://alex-nas.synology.me` |
| Port local | Laissez `8080`. Ne le changez que si quelque chose occupe déjà ce port sur le NAS |

**Les erreurs qu'on fait vraiment ici :**

| Écrit | Correct |
|---|---|
| `alex-nas.synology.me` | `https://alex-nas.synology.me` — avec le protocole |
| `https://alex-nas.synology.me/` | sans barre oblique à la fin |
| Une liste d'identifiants vide | vide veut dire **personne** ; mettez votre numéro |
| Votre adresse publique dans le champ de l'adresse DSM | l'adresse DSM reste `https://localhost:5001` |
| Un code 2FA à usage unique comme mot de passe | le compte ne doit pas avoir de vérification en deux étapes du tout (étape 3) |

Après l'installation, le paquet démarre tout seul et se relève avec le NAS. Les
réglages se trouvent alors dans `/var/packages/dsm-mini/var/config.env` (droits
`600`), et le journal à côté, dans `dsm-mini.log`.

Si le service n'arrive pas à démarrer — mauvais jeton, mauvais mot de passe, pas
de réseau — il le dit dans le **centre de notifications de DSM**, avec la raison.
Le journal complet est dans le Centre de paquets, sur la page du paquet.

### Étape 6. Diriger l'adresse vers le service

Pour l'instant le service n'écoute qu'à l'intérieur du NAS, sur le port 8080. Le
proxy inverse reçoit les requêtes d'Internet en HTTPS et les lui transmet.

1. **Panneau de configuration → Portail de connexion → Avancé → Proxy inversé →
   Créer**.
   (Sous DSM 7.0–7.1 c'est **Panneau de configuration → Portail des applications
   → Proxy inversé**.)
2. **Source** : protocole `HTTPS`, nom d'hôte `alex-nas.synology.me`, port `443`.
3. **Destination** : protocole `HTTP`, nom d'hôte `localhost`, port `8080`.
4. Enregistrez.

> **Ne faites pas passer le port 80 par le proxy pour ce nom** : c'est par lui
> que DSM renouvelle le certificat Let's Encrypt, et l'intercepter casse le
> renouvellement trois mois plus tard.

### Étape 7. Vérifier

Ouvrez `https://alex-nas.synology.me/healthz` dans un navigateur. Il doit
répondre :

```json
{"status":"ok"}
```

S'il a répondu, le service est vivant et joignable de l'extérieur. Et il ne livre
aucune donnée au passage : toute requête sans signature Telegram est refusée.

### Étape 8. Ouvrir l'application

1. Trouvez votre bot dans Telegram par l'identifiant de l'étape 1.
2. Appuyez sur **Start**.
3. En bas, à côté du champ de saisie, apparaît un bouton **Téléchargements** — il
   ouvre l'application. Le bot le place lui-même au démarrage, il n'y a rien à
   régler à la main.
4. Envoyez au bot n'importe quel lien magnet — il proposera des dossiers avec des
   boutons.

C'est prêt.

---

## Si quelque chose s'est mal passé

| Ce que vous voyez | De quoi il s'agit | Que faire |
|---|---|---|
| Le bot reste muet sur `/start` | Mauvais jeton, ou le paquet ne tourne pas | Centre de paquets → `dsm-mini` → le journal |
| « L'accès à ce bot est fermé » | Votre identifiant n'est pas dans la liste | Mettez le numéro de l'étape 2 dans les identifiants autorisés (voir « Changer les réglages » plus bas) |
| Il n'y a pas de bouton d'application | L'adresse publique est vide ou pas en `https://` | Au même endroit : le fichier de réglages, puis redémarrez le paquet |
| Le bouton est là, l'application ne s'ouvre pas | Le proxy inversé ou le certificat ne fonctionnent pas | Ouvrez `https://votre-adresse/healthz` dans un navigateur |
| « Ouvrez l'application par le bot » | L'application a été ouverte par un lien direct dans un navigateur | C'est voulu : ouvrez-la depuis le bot |
| « Accès refusé : votre identifiant Telegram… » | Le service ne vous a pas reconnu | Dans les identifiants autorisés, des chiffres seulement, séparés par des virgules |
| `authentication with DSM failed` dans le journal | Le mot de passe, la 2FA ou les droits de l'utilisateur | Étape 3 : mot de passe sans faute de frappe, 2FA désactivée, applications autorisées |
| `Could not get the task list` dans le journal | Download Station n'est pas installé, ou est refusé à l'utilisateur | Centre de paquets et les droits de l'étape 3 |
| Le paquet s'arrête juste après le démarrage | Un réglage est faux — le journal dit lequel | La raison arrive aussi dans le centre de notifications de DSM |

Le journal du paquet est la source de vérité principale : il nomme exactement ce
qui manque. Il se trouve dans `/var/packages/dsm-mini/var/dsm-mini.log` et
s'ouvre depuis le Centre de paquets. Le journal est en anglais, l'interface et
les messages du bot dans votre langue.

### Changer les réglages

Tout ce que l'assistant a demandé tient dans un fichier,
`/var/packages/dsm-mini/var/config.env`. Le plus simple pour changer une valeur
est d'installer le paquet par-dessus lui-même — l'assistant redemande. Pour
éditer le fichier directement il faut un accès SSH au NAS ; après l'édition,
arrêtez puis démarrez le paquet dans le Centre de paquets.

## Mise à jour

Si la source de paquets a été ajoutée, le Centre de paquets montre la mise à jour
tout seul. Sans source, téléchargez le `.spk` récent depuis les
[versions](https://github.com/alexlnos/dsm-mini/releases) et installez-le
par-dessus.

Les réglages et la base restent dans les deux cas : ils sont dans le répertoire
`var` du paquet, auquel une mise à jour ne touche pas.

## Où sont gardées les données

Tout est dans `/var/packages/dsm-mini/var/` :

- `config.env` — ce que l'assistant a demandé, droits `600` ;
- `dsm-mini.db` — une base SQLite : les dossiers épinglés, la langue et les
  derniers états connus des tâches, à partir desquels le service sait ce dont il
  a déjà rendu compte ;
- `dsm-mini.log` — le journal.

Une mise à jour du paquet garde les trois. La désinstallation les supprime.

## Sécurité

Le service est exposé sur Internet et sait supprimer des fichiers sur le NAS,
donc :

- **Un utilisateur DSM à part**, pas un administrateur (étape 3).
- **Sans vérification en deux étapes** sur ce compte : un code à usage unique ne
  peut pas fonctionner depuis un fichier de configuration.
- **Les identifiants autorisés sont une liste blanche.** Une liste vide ferme
  l'accès à tout le monde, elle ne l'ouvre pas.
- Chaque requête vers `/api/` est vérifiée deux fois : la signature `initData`
  avec une clé dérivée du jeton du bot, et l'identifiant face à la liste. La
  signature prouve seulement que quelqu'un a ouvert le bot — et l'ouvrir, tout
  le monde le peut.
- Vers l'extérieur part une phrase générale, les détails vont au journal : un
  message disant ce qui exactement n'a pas collé dans la signature indiquerait
  comment la fabriquer.
- Le jeton du bot et le mot de passe DSM ne vivent que dans `config.env` sur le
  NAS lui-même, droits `600`, et n'atteignent jamais le journal : sur un refus
  c'est la raison qui est écrite, jamais la valeur.
- Le service n'écoute que sur `127.0.0.1` — c'est-à-dire depuis le NAS
  lui-même. Tout ce qui vient de l'extérieur passe par le proxy inversé de DSM,
  qui termine aussi le TLS.

## Développement

```bash
cd web && npm install && npm run build   # la Mini App atterrit dans internal/web/dist
cd .. && go build ./cmd/dsm-mini         # un binaire avec l'application intégrée
go test ./...
```

Le frontend à part, avec rechargement automatique :

```bash
cd web && npm run dev    # parle au backend sur localhost:8080
```

Lancer le backend en local lit les mêmes variables que l'assistant du paquet
écrit dans `config.env`. Il est commode de les garder dans un fichier :

```bash
cp .env.example .env     # remplissez-le
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

L'interface construite est dans le dépôt (`internal/web/dist`) — c'est de là que
`go:embed` la prend. Après une retouche du frontend, reconstruisez et validez le
résultat, sinon les vérifications ne passeront pas.

Les tests d'intégration tournent contre un vrai NAS et sont ignorés par défaut :

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Les tests qui changent l'état du NAS demandent une permission à part :
`DSM_TEST_MUTATIONS=1`, et pour les opérations sur les fichiers aussi
`DSM_TEST_FOLDER` — le dossier à l'intérieur duquel des fichiers temporaires
peuvent être créés. Ils suppriment tout ce qu'ils ont créé.

Une nouvelle langue d'interface, c'est un fichier de dictionnaire de chaque côté :
`web/src/i18n/<code>.ts` et `internal/i18n/<code>.go`. Impossible d'oublier une
chaîne : sur le frontend le type y veille, sur le serveur un test.

Le code, les commentaires et le journal du projet sont en anglais ; les autres
langues ne vivent que dans les dictionnaires. Les détails sont dans
[CLAUDE.md](CLAUDE.md).

## Particularités de l'API Synology

L'API web de Synology se comporte par endroits autrement que sa documentation :
une même action répond en trois formats différents, `limit = -1` met File Station
à terre, et le `_sid` lors de l'envoi d'un fichier doit être transmis autrement
que partout ailleurs. Tout ce qui a été découvert sur un vrai NAS est rassemblé
dans [docs/synology-api.md](docs/synology-api.md).

## Licence

[MIT](LICENSE)
