#!/usr/bin/env python3
"""Builds the install wizard files for the DSM Package Center.

The wizard asks for the settings during installation, so nothing has to be
edited afterwards. DSM shows the file matching its interface language:
install_uifile is the common one (English), install_uifile_rus is Russian and
so on.

Every field gets three things: a short label, an example in the empty field
(`emptyText`, which DSM shows as placeholder text) and a line underneath
saying what the setting is for. The person filling this in has usually never
seen the project before and is being asked for a password and a token — "DSM
user" alone does not tell them which user, or why not the administrator.

The texts live here in one table: adding a language means extending the dict.
"""
import json
import pathlib

OUT = pathlib.Path(__file__).resolve().parent.parent / "spk" / "WIZARD_UIFILES"

# Wizard fields, in the order they are shown.
#   env key, text key, component, default, placeholder, step
#
# Four, and every one of them is something only the person installing knows.
# The DSM address and the service port used to be here and are not any more:
# the service runs on this NAS, so the address is always the loopback one, and
# 8080 is free on a NAS that has not been made to be otherwise. postinst still
# writes both — with its own defaults — and the settings screen inside DSM
# edits them for the rare installation where they are wrong. A question whose
# answer is the same for everyone is not a question, it is a thing to get
# wrong.
#
# The placeholders are the same in every language — they are addresses,
# numbers and a token, not prose.
FIELDS = [
    ("wizard_dsm_user", "dsm_user", "textfield", "", "dsm-mini", "nas"),
    ("wizard_dsm_password", "dsm_password", "password", "", "", "nas"),
    ("wizard_bot_token", "bot_token", "password", "", "1234567890:AAExampleTokenReplaceThisWithYours0", "tg"),
    ("wizard_allowed_ids", "allowed_ids", "textfield", "", "123456789,987654321", "tg"),
    ("wizard_public_url", "public_url", "textfield", "", "https://nas.example.com", "tg"),
]

# The suffixes are DSM language codes from the Synology developer guide, not
# Telegram ones: enu, rus, ger, fre, ita, spn, ptb, plk, trk. Italian is ita —
# itn was here once, and a suffix DSM does not know is not an error, the
# installer just quietly shows English.
TEXTS = {
    None: {  # English — the common file
        "step_nas": "NAS access",
        "nas_intro": "The service signs in to DSM the way you do, and it can delete files. Give it an account of its own rather than an administrator: Control Panel → User & Group → Create, access to Download Station and File Station only.",
        "dsm_user": "DSM user for the service",
        "dsm_user_note": "The account you have just created. Do not turn on two-factor verification for it: a one-time code cannot be typed in from a configuration file, and the sign-in will simply fail.",
        "dsm_password": "Password of that user",
        "dsm_password_note": "Kept on this NAS in the package's own settings file, readable by root only, and never written to the log — a refusal is logged with its reason, not with the value.",

        "step_telegram": "Telegram",
        "telegram_intro": "The bot is the entrance: people write to it, and the app opens from it.",
        "bot_token": "Bot token",
        "bot_token_note": "Open @BotFather in Telegram, send /newbot, and it answers with a line like the one shown here. Whoever holds the token controls the bot, so treat it as a password.",
        "allowed_ids": "Who may use the bot — numeric IDs, comma separated",
        "allowed_ids_note": "Your bot is public: anyone who finds it can press Start. This list is what decides whose messages are answered and who may open the app — everyone else is refused. Get your number from @userinfobot; it is the account id, not the @username. An empty list lets nobody in, not everybody.",
        "public_url": "Public HTTPS address of the app",
        "public_url_note": "Telegram opens a Mini App only over https with a real certificate: a local address or a self-signed one will not open. Point this name at the NAS, then add Control Panel → Login Portal → Advanced → Reverse Proxy from it to localhost and the port below. No slash at the end.",

        "required": "This field is required",
        "url_error": "Must start with https:// and have no trailing slash",
        "ids_error": "Digits and commas only",
    },

    "rus": {
        "step_nas": "Доступ к NAS",
        "nas_intro": "Служба входит в DSM так же, как вы, и умеет удалять файлы. Заведите ей отдельную учётную запись, а не отдавайте администратора: Панель управления → Пользователь и группа → Создать, доступ только к Download Station и File Station.",
        "dsm_user": "Пользователь DSM для службы",
        "dsm_user_note": "Та учётная запись, которую вы только что создали. Двухэтапную проверку ей не включайте: одноразовый код неоткуда взять в файле настроек, и вход просто не пройдёт.",
        "dsm_password": "Пароль этого пользователя",
        "dsm_password_note": "Хранится на этом NAS в файле настроек пакета, читать его может только root, и в журнал он не попадает — при отказе пишется причина, а не значение.",

        "step_telegram": "Telegram",
        "telegram_intro": "Бот — это вход: люди пишут ему, и приложение открывается из него.",
        "bot_token": "Токен бота",
        "bot_token_note": "Откройте в Telegram @BotFather, отправьте /newbot — он ответит строкой вроде показанной здесь. Кто получил токен, тот управляет ботом, поэтому обращайтесь с ним как с паролем.",
        "allowed_ids": "Кому можно пользоваться ботом — числовые ID через запятую",
        "allowed_ids_note": "Ваш бот открыт: нажать «Старт» может любой, кто его найдёт. Именно этот список решает, кому отвечать и кого пускать в приложение, — остальные получат отказ. Свой номер узнайте у @userinfobot: это идентификатор учётной записи, а не @имя. Пустой список не пускает никого, а не всех.",
        "public_url": "Публичный HTTPS-адрес приложения",
        "public_url_note": "Telegram открывает Mini App только по https с настоящим сертификатом: локальный адрес и самоподписанный не откроются. Направьте это имя на NAS, потом добавьте Панель управления → Портал входа → Дополнительно → Обратный прокси-сервер с него на localhost и порт ниже. Без слеша в конце.",

        "required": "Поле обязательно",
        "url_error": "Должно начинаться с https:// и быть без слеша в конце",
        "ids_error": "Только цифры и запятые",
    },

    "ger": {
        "step_nas": "NAS-Zugang",
        "nas_intro": "Der Dienst meldet sich bei DSM an wie Sie und kann Dateien löschen. Geben Sie ihm ein eigenes Konto statt eines Administrators: Systemsteuerung → Benutzer & Gruppe → Erstellen, Zugriff nur auf Download Station und File Station.",
        "dsm_user": "DSM-Benutzer für den Dienst",
        "dsm_user_note": "Das Konto, das Sie gerade angelegt haben. Schalten Sie dafür keine zweistufige Verifizierung ein: einen Einmalcode kann eine Konfigurationsdatei nicht liefern, und die Anmeldung scheitert schlicht.",
        "dsm_password": "Passwort dieses Benutzers",
        "dsm_password_note": "Liegt auf diesem NAS in der Einstellungsdatei des Pakets, lesbar nur für root, und kommt nie ins Protokoll — bei einer Ablehnung wird der Grund geschrieben, nicht der Wert.",

        "step_telegram": "Telegram",
        "telegram_intro": "Der Bot ist der Eingang: Menschen schreiben ihm, und die App öffnet sich daraus.",
        "bot_token": "Bot-Token",
        "bot_token_note": "Öffnen Sie @BotFather in Telegram und senden Sie /newbot — die Antwort ist eine Zeile wie die hier gezeigte. Wer das Token hat, steuert den Bot; behandeln Sie es wie ein Passwort.",
        "allowed_ids": "Wer den Bot benutzen darf — numerische IDs, durch Komma getrennt",
        "allowed_ids_note": "Ihr Bot ist offen: Start drücken kann jeder, der ihn findet. Erst diese Liste entscheidet, wem geantwortet wird und wer die App öffnen darf — alle anderen werden abgewiesen. Ihre Nummer nennt Ihnen @userinfobot; es ist die Konto-ID, nicht der @Name. Eine leere Liste lässt niemanden hinein, nicht jeden.",
        "public_url": "Öffentliche HTTPS-Adresse der App",
        "public_url_note": "Telegram öffnet eine Mini App nur über https mit echtem Zertifikat: eine lokale Adresse oder ein selbstsigniertes geht nicht auf. Zeigen Sie diesen Namen auf das NAS und legen Sie dann unter Systemsteuerung → Anmeldeportal → Erweitert → Reverse Proxy eine Weiterleitung auf localhost und den Port unten an. Ohne Schrägstrich am Ende.",

        "required": "Pflichtfeld",
        "url_error": "Muss mit https:// beginnen, ohne Schrägstrich am Ende",
        "ids_error": "Nur Ziffern und Kommas",
    },

    "fre": {
        "step_nas": "Accès au NAS",
        "nas_intro": "Le service se connecte à DSM comme vous et sait supprimer des fichiers. Donnez-lui un compte à lui plutôt qu'un administrateur : Panneau de configuration → Utilisateur et groupe → Créer, accès à Download Station et File Station uniquement.",
        "dsm_user": "Utilisateur DSM pour le service",
        "dsm_user_note": "Le compte que vous venez de créer. Ne lui activez pas la vérification en deux étapes : un code à usage unique ne peut pas venir d'un fichier de configuration, et la connexion échouera simplement.",
        "dsm_password": "Mot de passe de cet utilisateur",
        "dsm_password_note": "Conservé sur ce NAS dans le fichier de réglages du paquet, lisible par root seulement, et jamais écrit dans le journal — sur un refus on note la raison, pas la valeur.",

        "step_telegram": "Telegram",
        "telegram_intro": "Le bot est l'entrée : les gens lui écrivent, et l'application s'ouvre depuis lui.",
        "bot_token": "Jeton du bot",
        "bot_token_note": "Ouvrez @BotFather dans Telegram et envoyez /newbot — il répond par une ligne comme celle montrée ici. Qui détient le jeton commande le bot : traitez-le comme un mot de passe.",
        "allowed_ids": "Qui peut utiliser le bot — identifiants numériques séparés par des virgules",
        "allowed_ids_note": "Votre bot est ouvert : n'importe qui le trouvant peut appuyer sur Démarrer. C'est cette liste qui décide à qui l'on répond et qui peut ouvrir l'application — les autres sont refusés. @userinfobot vous donne votre numéro ; c'est l'identifiant du compte, pas le @nom. Une liste vide ne laisse entrer personne, pas tout le monde.",
        "public_url": "Adresse HTTPS publique de l'application",
        "public_url_note": "Telegram n'ouvre une Mini App qu'en https avec un vrai certificat : une adresse locale ou un certificat auto-signé ne s'ouvriront pas. Faites pointer ce nom vers le NAS, puis ajoutez Panneau de configuration → Portail de connexion → Avancé → Proxy inversé depuis cette adresse vers localhost et le port ci-dessous. Sans barre oblique à la fin.",

        "required": "Champ obligatoire",
        "url_error": "Doit commencer par https:// et finir sans barre oblique",
        "ids_error": "Chiffres et virgules uniquement",
    },

    "ita": {
        "step_nas": "Accesso al NAS",
        "nas_intro": "Il servizio accede a DSM come te e sa cancellare file. Dagli un account suo invece di un amministratore: Pannello di controllo → Utente e gruppo → Crea, accesso solo a Download Station e File Station.",
        "dsm_user": "Utente DSM per il servizio",
        "dsm_user_note": "L'account che hai appena creato. Non attivargli la verifica in due passaggi: un codice usa e getta non può arrivare da un file di configurazione e l'accesso semplicemente fallisce.",
        "dsm_password": "Password di quell'utente",
        "dsm_password_note": "Resta su questo NAS nel file delle impostazioni del pacchetto, leggibile solo da root, e non finisce mai nel registro — in un rifiuto si scrive il motivo, non il valore.",

        "step_telegram": "Telegram",
        "telegram_intro": "Il bot è l'ingresso: le persone gli scrivono e l'applicazione si apre da lì.",
        "bot_token": "Token del bot",
        "bot_token_note": "Apri @BotFather in Telegram e manda /newbot: risponde con una riga come quella mostrata qui. Chi ha il token comanda il bot, quindi trattalo come una password.",
        "allowed_ids": "Chi può usare il bot — ID numerici separati da virgole",
        "allowed_ids_note": "Il tuo bot è aperto: chiunque lo trovi può premere Avvia. È questo elenco a decidere a chi rispondere e chi può aprire l'applicazione — gli altri vengono respinti. Il tuo numero te lo dà @userinfobot; è l'id dell'account, non il @nome. Un elenco vuoto non fa entrare nessuno, non tutti.",
        "public_url": "Indirizzo HTTPS pubblico dell'applicazione",
        "public_url_note": "Telegram apre una Mini App solo via https con un certificato vero: un indirizzo locale o uno autofirmato non si apriranno. Fai puntare questo nome al NAS, poi aggiungi Pannello di controllo → Portale di accesso → Avanzate → Proxy inverso da qui a localhost e alla porta qui sotto. Senza barra finale.",

        "required": "Campo obbligatorio",
        "url_error": "Deve iniziare con https:// e non finire con una barra",
        "ids_error": "Solo cifre e virgole",
    },

    "spn": {
        "step_nas": "Acceso al NAS",
        "nas_intro": "El servicio entra en DSM igual que tú y puede borrar archivos. Dale una cuenta propia en vez de un administrador: Panel de control → Usuario y grupo → Crear, acceso solo a Download Station y File Station.",
        "dsm_user": "Usuario de DSM para el servicio",
        "dsm_user_note": "La cuenta que acabas de crear. No le actives la verificación en dos pasos: un código de un solo uso no puede salir de un archivo de configuración y el inicio de sesión sencillamente fallará.",
        "dsm_password": "Contraseña de ese usuario",
        "dsm_password_note": "Se guarda en este NAS, en el archivo de ajustes del paquete, legible solo por root, y nunca llega al registro: ante un rechazo se escribe el motivo, no el valor.",

        "step_telegram": "Telegram",
        "telegram_intro": "El bot es la entrada: la gente le escribe y la aplicación se abre desde él.",
        "bot_token": "Token del bot",
        "bot_token_note": "Abre @BotFather en Telegram y envía /newbot: responde con una línea como la que se ve aquí. Quien tiene el token manda en el bot, así que trátalo como una contraseña.",
        "allowed_ids": "Quién puede usar el bot — ID numéricos separados por comas",
        "allowed_ids_note": "Tu bot está abierto: cualquiera que lo encuentre puede pulsar Iniciar. Es esta lista la que decide a quién se responde y quién puede abrir la aplicación; al resto se le niega. Tu número te lo da @userinfobot; es el id de la cuenta, no el @nombre. Una lista vacía no deja entrar a nadie, no a todos.",
        "public_url": "Dirección HTTPS pública de la aplicación",
        "public_url_note": "Telegram abre una Mini App solo por https con un certificado de verdad: una dirección local o uno autofirmado no abrirán. Apunta este nombre al NAS y añade Panel de control → Portal de inicio de sesión → Avanzado → Proxy inverso desde él hacia localhost y el puerto de abajo. Sin barra al final.",

        "required": "Campo obligatorio",
        "url_error": "Debe empezar por https:// y no llevar barra al final",
        "ids_error": "Solo dígitos y comas",
    },

    "ptb": {
        "step_nas": "Acesso ao NAS",
        "nas_intro": "O serviço entra no DSM do mesmo jeito que você e consegue excluir arquivos. Dê a ele uma conta própria em vez de um administrador: Painel de Controle → Usuário e grupo → Criar, acesso só ao Download Station e ao File Station.",
        "dsm_user": "Usuário do DSM para o serviço",
        "dsm_user_note": "A conta que você acabou de criar. Não ligue a verificação em duas etapas nela: um código único não tem como vir de um arquivo de configuração e o login simplesmente falha.",
        "dsm_password": "Senha desse usuário",
        "dsm_password_note": "Fica neste NAS, no arquivo de configurações do pacote, legível só pelo root, e nunca vai para o registro — numa recusa escreve-se o motivo, não o valor.",

        "step_telegram": "Telegram",
        "telegram_intro": "O bot é a entrada: as pessoas escrevem para ele e o aplicativo abre a partir dele.",
        "bot_token": "Token do bot",
        "bot_token_note": "Abra o @BotFather no Telegram e mande /newbot: ele responde com uma linha como a mostrada aqui. Quem tem o token comanda o bot, então trate-o como uma senha.",
        "allowed_ids": "Quem pode usar o bot — IDs numéricos separados por vírgula",
        "allowed_ids_note": "Seu bot é aberto: qualquer um que o encontre pode apertar Iniciar. É esta lista que decide para quem responder e quem pode abrir o aplicativo — os demais são recusados. Seu número vem do @userinfobot; é o id da conta, não o @nome. Uma lista vazia não deixa ninguém entrar, e não todo mundo.",
        "public_url": "Endereço HTTPS público do aplicativo",
        "public_url_note": "O Telegram abre um Mini App só por https com certificado de verdade: um endereço local ou um autoassinado não abrem. Aponte este nome para o NAS e depois adicione Painel de Controle → Portal de login → Avançado → Proxy reverso dele para o localhost e a porta abaixo. Sem barra no fim.",

        "required": "Campo obrigatório",
        "url_error": "Precisa começar com https:// e não terminar com barra",
        "ids_error": "Somente dígitos e vírgulas",
    },

    "plk": {
        "step_nas": "Dostęp do NAS-a",
        "nas_intro": "Usługa loguje się do DSM tak samo jak ty i potrafi kasować pliki. Daj jej własne konto zamiast administratora: Panel sterowania → Użytkownik i grupa → Utwórz, dostęp tylko do Download Station i File Station.",
        "dsm_user": "Użytkownik DSM dla usługi",
        "dsm_user_note": "Konto, które przed chwilą założyłeś. Nie włączaj mu weryfikacji dwuetapowej: kod jednorazowy nie ma jak trafić z pliku konfiguracyjnego i logowanie po prostu się nie uda.",
        "dsm_password": "Hasło tego użytkownika",
        "dsm_password_note": "Leży na tym NAS-ie w pliku ustawień pakietu, czytelnym tylko dla roota, i nigdy nie trafia do dziennika — przy odmowie zapisywana jest przyczyna, nie wartość.",

        "step_telegram": "Telegram",
        "telegram_intro": "Bot jest wejściem: ludzie do niego piszą, a aplikacja otwiera się z niego.",
        "bot_token": "Token bota",
        "bot_token_note": "Otwórz @BotFather w Telegramie i wyślij /newbot — odpowie linią taką jak pokazana tutaj. Kto ma token, ten rządzi botem, więc traktuj go jak hasło.",
        "allowed_ids": "Kto może używać bota — numeryczne ID po przecinku",
        "allowed_ids_note": "Twój bot jest otwarty: każdy, kto go znajdzie, może nacisnąć Start. To ta lista decyduje, komu odpowiadać i kto może otworzyć aplikację — reszta dostaje odmowę. Swój numer poznasz u @userinfobot; to identyfikator konta, nie @nazwa. Pusta lista nie wpuszcza nikogo, a nie wszystkich.",
        "public_url": "Publiczny adres HTTPS aplikacji",
        "public_url_note": "Telegram otwiera Mini App tylko po https z prawdziwym certyfikatem: adres lokalny ani samopodpisany się nie otworzą. Skieruj tę nazwę na NAS-a, a potem dodaj Panel sterowania → Portal logowania → Zaawansowane → Zwrotny serwer proxy z niej na localhost i port poniżej. Bez ukośnika na końcu.",

        "required": "Pole wymagane",
        "url_error": "Musi zaczynać się od https:// i nie kończyć ukośnikiem",
        "ids_error": "Tylko cyfry i przecinki",
    },

    "trk": {
        "step_nas": "NAS erişimi",
        "nas_intro": "Servis DSM'ye sizin gibi girer ve dosya silebilir. Ona yönetici yerine kendi hesabını verin: Denetim Masası → Kullanıcı ve Grup → Oluştur, yalnızca Download Station ve File Station erişimi.",
        "dsm_user": "Servis için DSM kullanıcısı",
        "dsm_user_note": "Az önce oluşturduğunuz hesap. Buna iki aşamalı doğrulama açmayın: tek kullanımlık kod bir yapılandırma dosyasından gelemez ve oturum açma basitçe başarısız olur.",
        "dsm_password": "O kullanıcının parolası",
        "dsm_password_note": "Bu NAS'ta, paketin kendi ayar dosyasında durur, yalnızca root okuyabilir ve hiçbir zaman günlüğe yazılmaz — bir reddedişte değer değil, neden yazılır.",

        "step_telegram": "Telegram",
        "telegram_intro": "Bot giriş kapısıdır: insanlar ona yazar ve uygulama oradan açılır.",
        "bot_token": "Bot belirteci",
        "bot_token_note": "Telegram'da @BotFather'ı açıp /newbot gönderin: burada görülen gibi bir satırla yanıt verir. Belirteci elinde tutan botu yönetir, o yüzden ona parola gibi davranın.",
        "allowed_ids": "Botu kimler kullanabilir — virgülle ayrılmış sayısal kimlikler",
        "allowed_ids_note": "Botunuz açıktır: onu bulan herkes Başlat'a basabilir. Kime yanıt verileceğine ve uygulamayı kimin açabileceğine bu liste karar verir; geri kalanı reddedilir. Numaranızı @userinfobot söyler; bu, @adınız değil hesap kimliğidir. Boş liste herkesi değil, hiç kimseyi içeri almaz.",
        "public_url": "Uygulamanın genel HTTPS adresi",
        "public_url_note": "Telegram bir Mini App'i yalnızca gerçek sertifikalı https üzerinden açar: yerel bir adres ya da kendinden imzalı bir sertifika açılmaz. Bu adı NAS'a yönlendirin, sonra Denetim Masası → Oturum Açma Portalı → Gelişmiş → Ters Proxy ile buradan localhost'a ve aşağıdaki bağlantı noktasına aktarın. Sonunda eğik çizgi olmasın.",

        "required": "Bu alan zorunlu",
        "url_error": "https:// ile başlamalı ve sonunda eğik çizgi olmamalı",
        "ids_error": "Yalnızca rakam ve virgül",
    },
}


def field(key, text_key, kind, default, placeholder, t):
    """One wizard field in the shape DSM understands."""
    validator = {"allowBlank": False, "errorText": t["required"]}
    if text_key == "public_url":
        validator["regex"] = {"expr": "/^https:\\/\\/[^\\/]+$/", "errorText": t["url_error"]}
    elif text_key == "allowed_ids":
        validator["regex"] = {"expr": "/^[0-9]+(,[0-9]+)*$/", "errorText": t["ids_error"]}
    elif text_key == "port":
        validator["regex"] = {"expr": "/^[0-9]{2,5}$/", "errorText": t["port_error"]}
    elif text_key == "dsm_url":
        validator["regex"] = {"expr": "/^https?:\\/\\/[^\\/]+$/", "errorText": t["url_error"]}

    subitem = {
        "key": key,
        "desc": t[text_key],
        "defaultValue": default,
        "validator": validator,
    }
    # emptyText is the placeholder DSM shows in an empty field; with a default
    # filled in there is nothing to show it in.
    if placeholder and not default:
        subitem["emptyText"] = placeholder

    return {"type": kind, "subitems": [subitem]}


def wizard(t):
    steps = {"nas": [{"desc": t["nas_intro"]}], "tg": [{"desc": t["telegram_intro"]}]}
    for key, text_key, kind, default, placeholder, step in FIELDS:
        steps[step].append(field(key, text_key, kind, default, placeholder, t))
        # The explanation goes after its field, where it reads as a caption
        # rather than as another thing to fill in.
        steps[step].append({"desc": t[f"{text_key}_note"]})
    return [
        {"step_title": t["step_nas"], "items": steps["nas"]},
        {"step_title": t["step_telegram"], "items": steps["tg"]},
    ]


OUT.mkdir(parents=True, exist_ok=True)
for lang, texts in TEXTS.items():
    name = "install_uifile" if lang is None else f"install_uifile_{lang}"
    (OUT / name).write_text(json.dumps(wizard(texts), ensure_ascii=False, indent=3) + "\n",
                            encoding="utf-8")
    print("ready", name)
