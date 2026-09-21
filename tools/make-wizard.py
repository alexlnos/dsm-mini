#!/usr/bin/env python3
"""Собирает файлы мастера установки для Центра пакетов DSM.

Мастер спрашивает настройки при установке, поэтому после неё ничего править
не нужно. DSM показывает файл по языку своего интерфейса: install_uifile —
общий (английский), install_uifile_rus — русский, и так далее. Суффиксы
языков — из списка DSM, не совпадают с кодами Telegram.

Тексты лежат здесь одной таблицей: добавить язык — дописать словарь.
"""
import json
import pathlib

OUT = pathlib.Path(__file__).resolve().parent.parent / "spk" / "WIZARD_UIFILES"

# Поля мастера. Ключ становится переменной окружения в postinst.
FIELDS = [
    ("wizard_dsm_url", "dsm_url", "textfield", "https://localhost:5001"),
    ("wizard_dsm_user", "dsm_user", "textfield", ""),
    ("wizard_dsm_password", "dsm_password", "password", ""),
    ("wizard_bot_token", "bot_token", "password", ""),
    ("wizard_allowed_ids", "allowed_ids", "textfield", ""),
    ("wizard_public_url", "public_url", "textfield", ""),
    ("wizard_port", "port", "textfield", "8080"),
]

TEXTS = {
    None: {  # английский — общий файл
        "step_nas": "NAS access",
        "step_telegram": "Telegram",
        "nas_intro": "Create a separate DSM user for this service instead of using an administrator: Control Panel → User & Group → Create. Give it access to Download Station and File Station only, and do not enable two-factor authentication.",
        "dsm_url": "DSM address",
        "dsm_user": "DSM user",
        "dsm_password": "DSM password",
        "telegram_intro": "Create a bot with @BotFather and get your numeric ID from @userinfobot.",
        "bot_token": "Bot token",
        "allowed_ids": "Allowed Telegram IDs (comma separated)",
        "public_url": "Public HTTPS address of the app",
        "public_hint": "Telegram opens the Mini App only over HTTPS with a valid certificate. Set up a reverse proxy in DSM from this address to localhost and the port below.",
        "port": "Local port",
        "required": "This field is required",
        "url_error": "Must start with https:// and contain no trailing slash",
        "ids_error": "Digits and commas only",
        "port_error": "Port number only",
    },
    "rus": {
        "step_nas": "Доступ к NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Заведите для службы отдельного пользователя DSM, а не используйте администратора: Панель управления → Пользователь и группа → Создать. Дайте права только на Download Station и File Station и не включайте двухэтапную проверку.",
        "dsm_url": "Адрес DSM",
        "dsm_user": "Пользователь DSM",
        "dsm_password": "Пароль DSM",
        "telegram_intro": "Создайте бота у @BotFather, а свой числовой идентификатор узнайте у @userinfobot.",
        "bot_token": "Токен бота",
        "allowed_ids": "Разрешённые Telegram ID (через запятую)",
        "public_url": "Публичный HTTPS-адрес приложения",
        "public_hint": "Telegram открывает Mini App только по HTTPS с настоящим сертификатом. Настройте в DSM обратный прокси с этого адреса на localhost и порт ниже.",
        "port": "Локальный порт",
        "required": "Поле обязательно",
        "url_error": "Должен начинаться с https:// и быть без слеша в конце",
        "ids_error": "Только цифры и запятые",
        "port_error": "Только номер порта",
    },
    "ger": {
        "step_nas": "NAS-Zugang",
        "step_telegram": "Telegram",
        "nas_intro": "Legen Sie für den Dienst einen eigenen DSM-Benutzer an statt einen Administrator zu verwenden: Systemsteuerung → Benutzer & Gruppe → Erstellen. Erlauben Sie nur Download Station und File Station und aktivieren Sie keine Zwei-Faktor-Authentifizierung.",
        "dsm_url": "DSM-Adresse",
        "dsm_user": "DSM-Benutzer",
        "dsm_password": "DSM-Passwort",
        "telegram_intro": "Erstellen Sie einen Bot bei @BotFather und holen Sie Ihre numerische ID von @userinfobot.",
        "bot_token": "Bot-Token",
        "allowed_ids": "Erlaubte Telegram-IDs (durch Komma getrennt)",
        "public_url": "Öffentliche HTTPS-Adresse der App",
        "public_hint": "Telegram öffnet die Mini App nur über HTTPS mit gültigem Zertifikat. Richten Sie in DSM einen Reverse-Proxy von dieser Adresse auf localhost und den Port unten ein.",
        "port": "Lokaler Port",
        "required": "Pflichtfeld",
        "url_error": "Muss mit https:// beginnen, ohne Schrägstrich am Ende",
        "ids_error": "Nur Ziffern und Kommas",
        "port_error": "Nur eine Portnummer",
    },
    "fre": {
        "step_nas": "Accès au NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Créez un utilisateur DSM dédié au service plutôt qu'un administrateur : Panneau de configuration → Utilisateur et groupe → Créer. N'autorisez que Download Station et File Station, et n'activez pas la double authentification.",
        "dsm_url": "Adresse DSM",
        "dsm_user": "Utilisateur DSM",
        "dsm_password": "Mot de passe DSM",
        "telegram_intro": "Créez un bot avec @BotFather et récupérez votre identifiant numérique auprès de @userinfobot.",
        "bot_token": "Jeton du bot",
        "allowed_ids": "Identifiants Telegram autorisés (séparés par des virgules)",
        "public_url": "Adresse HTTPS publique de l'application",
        "public_hint": "Telegram n'ouvre la Mini App qu'en HTTPS avec un certificat valide. Configurez dans DSM un proxy inverse de cette adresse vers localhost et le port ci-dessous.",
        "port": "Port local",
        "required": "Champ obligatoire",
        "url_error": "Doit commencer par https:// et sans barre oblique finale",
        "ids_error": "Chiffres et virgules uniquement",
        "port_error": "Numéro de port uniquement",
    },
    "spn": {
        "step_nas": "Acceso al NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Cree un usuario de DSM propio para el servicio en lugar de usar un administrador: Panel de control → Usuario y grupo → Crear. Permita solo Download Station y File Station, y no active la verificación en dos pasos.",
        "dsm_url": "Dirección de DSM",
        "dsm_user": "Usuario de DSM",
        "dsm_password": "Contraseña de DSM",
        "telegram_intro": "Cree un bot con @BotFather y obtenga su identificador numérico en @userinfobot.",
        "bot_token": "Token del bot",
        "allowed_ids": "IDs de Telegram permitidos (separados por comas)",
        "public_url": "Dirección HTTPS pública de la aplicación",
        "public_hint": "Telegram abre la Mini App solo por HTTPS con un certificado válido. Configure en DSM un proxy inverso desde esta dirección a localhost y el puerto de abajo.",
        "port": "Puerto local",
        "required": "Campo obligatorio",
        "url_error": "Debe empezar por https:// y sin barra final",
        "ids_error": "Solo dígitos y comas",
        "port_error": "Solo el número de puerto",
    },
    "itn": {
        "step_nas": "Accesso al NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Crea un utente DSM dedicato al servizio invece di usare un amministratore: Pannello di controllo → Utente e gruppo → Crea. Concedi solo Download Station e File Station e non attivare la verifica in due passaggi.",
        "dsm_url": "Indirizzo DSM",
        "dsm_user": "Utente DSM",
        "dsm_password": "Password DSM",
        "telegram_intro": "Crea un bot con @BotFather e prendi il tuo identificativo numerico da @userinfobot.",
        "bot_token": "Token del bot",
        "allowed_ids": "ID Telegram consentiti (separati da virgola)",
        "public_url": "Indirizzo HTTPS pubblico dell'app",
        "public_hint": "Telegram apre la Mini App solo via HTTPS con certificato valido. Configura in DSM un proxy inverso da questo indirizzo a localhost e alla porta qui sotto.",
        "port": "Porta locale",
        "required": "Campo obbligatorio",
        "url_error": "Deve iniziare con https:// e senza barra finale",
        "ids_error": "Solo cifre e virgole",
        "port_error": "Solo il numero di porta",
    },
    "ptb": {
        "step_nas": "Acesso ao NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Crie um usuário do DSM só para o serviço em vez de usar um administrador: Painel de controle → Usuário e grupo → Criar. Libere apenas Download Station e File Station e não ative a verificação em duas etapas.",
        "dsm_url": "Endereço do DSM",
        "dsm_user": "Usuário do DSM",
        "dsm_password": "Senha do DSM",
        "telegram_intro": "Crie um bot com @BotFather e pegue seu identificador numérico com @userinfobot.",
        "bot_token": "Token do bot",
        "allowed_ids": "IDs do Telegram permitidos (separados por vírgula)",
        "public_url": "Endereço HTTPS público do aplicativo",
        "public_hint": "O Telegram abre o Mini App apenas por HTTPS com certificado válido. Configure no DSM um proxy reverso desse endereço para localhost e a porta abaixo.",
        "port": "Porta local",
        "required": "Campo obrigatório",
        "url_error": "Deve começar com https:// e sem barra no final",
        "ids_error": "Apenas dígitos e vírgulas",
        "port_error": "Apenas o número da porta",
    },
    "plk": {
        "step_nas": "Dostęp do NAS",
        "step_telegram": "Telegram",
        "nas_intro": "Załóż dla usługi osobnego użytkownika DSM zamiast używać administratora: Panel sterowania → Użytkownik i grupa → Utwórz. Daj uprawnienia tylko do Download Station i File Station i nie włączaj weryfikacji dwuetapowej.",
        "dsm_url": "Adres DSM",
        "dsm_user": "Użytkownik DSM",
        "dsm_password": "Hasło DSM",
        "telegram_intro": "Utwórz bota u @BotFather, a swój numeryczny identyfikator pobierz od @userinfobot.",
        "bot_token": "Token bota",
        "allowed_ids": "Dozwolone identyfikatory Telegrama (po przecinku)",
        "public_url": "Publiczny adres HTTPS aplikacji",
        "public_hint": "Telegram otwiera Mini App tylko po HTTPS z ważnym certyfikatem. Skonfiguruj w DSM odwrotny serwer proxy z tego adresu na localhost i port poniżej.",
        "port": "Port lokalny",
        "required": "Pole wymagane",
        "url_error": "Musi zaczynać się od https:// i nie kończyć ukośnikiem",
        "ids_error": "Tylko cyfry i przecinki",
        "port_error": "Tylko numer portu",
    },
    "trk": {
        "step_nas": "NAS erişimi",
        "step_telegram": "Telegram",
        "nas_intro": "Hizmet için yönetici yerine ayrı bir DSM kullanıcısı oluşturun: Denetim Masası → Kullanıcı ve Grup → Oluştur. Yalnızca Download Station ve File Station izni verin ve iki adımlı doğrulamayı açmayın.",
        "dsm_url": "DSM adresi",
        "dsm_user": "DSM kullanıcısı",
        "dsm_password": "DSM parolası",
        "telegram_intro": "@BotFather ile bir bot oluşturun, sayısal kimliğinizi @userinfobot'tan alın.",
        "bot_token": "Bot belirteci",
        "allowed_ids": "İzin verilen Telegram kimlikleri (virgülle)",
        "public_url": "Uygulamanın herkese açık HTTPS adresi",
        "public_hint": "Telegram, Mini App'i yalnızca geçerli sertifikalı HTTPS üzerinden açar. DSM'de bu adresten localhost'a ve aşağıdaki porta ters proxy kurun.",
        "port": "Yerel port",
        "required": "Bu alan zorunlu",
        "url_error": "https:// ile başlamalı ve sonunda eğik çizgi olmamalı",
        "ids_error": "Yalnızca rakam ve virgül",
        "port_error": "Yalnızca port numarası",
    },
}


def field(key, text_key, kind, default, t):
    """Одно поле мастера в том виде, в каком его понимает DSM."""
    validator = {"allowBlank": False, "errorText": t["required"]}
    if text_key == "public_url":
        validator["regex"] = {"expr": "/^https:\\/\\/[^\\/]+$/", "errorText": t["url_error"]}
    elif text_key == "allowed_ids":
        validator["regex"] = {"expr": "/^[0-9]+(,[0-9]+)*$/", "errorText": t["ids_error"]}
    elif text_key == "port":
        validator["regex"] = {"expr": "/^[0-9]{2,5}$/", "errorText": t["port_error"]}
    elif text_key == "dsm_url":
        validator["regex"] = {"expr": "/^https?:\\/\\/[^\\/]+$/", "errorText": t["url_error"]}

    return {
        "type": kind,
        "subitems": [{
            "key": key,
            "desc": t[text_key],
            "defaultValue": default,
            "validator": validator,
        }],
    }


def wizard(t):
    nas = [{"desc": t["nas_intro"]}]
    telegram = [{"desc": t["telegram_intro"]}]
    for key, text_key, kind, default in FIELDS:
        item = field(key, text_key, kind, default, t)
        if text_key.startswith("dsm_"):
            nas.append(item)
        else:
            if text_key == "port":
                telegram.append({"desc": t["public_hint"]})
            telegram.append(item)
    return [
        {"step_title": t["step_nas"], "items": nas},
        {"step_title": t["step_telegram"], "items": telegram},
    ]


OUT.mkdir(parents=True, exist_ok=True)
for lang, texts in TEXTS.items():
    name = "install_uifile" if lang is None else f"install_uifile_{lang}"
    (OUT / name).write_text(json.dumps(wizard(texts), ensure_ascii=False, indent=3) + "\n",
                            encoding="utf-8")
    print("готов", name)
