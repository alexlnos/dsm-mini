#!/usr/bin/env python3
"""Builds spk/ui/i18n.js, the dictionary of the settings screen inside DSM.

The screen is a plain page served by DSM's web server, with no build step and
no bundler, so its translations are generated here and committed — the same
arrangement as the installer wizard in tools/make-wizard.py. English is the
source of truth; a missing string falls back to it rather than showing a key.

    tools/make-dsmui-i18n.py
"""
import json
import pathlib

# Order matters only for reading the file: en first, then the rest as in
# web/src/i18n.
STRINGS = {
    "loading":        ["Loading…", "Загрузка…", "Завантаження…", "Wird geladen…", "Chargement…", "Cargando…", "Caricamento…", "Carregando…", "Wczytywanie…", "Yükleniyor…"],
    "connection":     ["Connection to the NAS", "Подключение к NAS", "Підключення до NAS", "Verbindung zum NAS", "Connexion au NAS", "Conexión al NAS", "Connessione al NAS", "Conexão ao NAS", "Połączenie z NAS", "NAS bağlantısı"],
    "connectionHint": ["The service signs in to DSM as an ordinary user you created for it. DSM 7 gives a package no access of its own.",
                       "Служба входит в DSM обычным пользователем, заведённым для неё. Своего доступа DSM 7 пакету не даёт.",
                       "Служба входить у DSM звичайним користувачем, створеним для неї. Власного доступу DSM 7 пакунку не дає.",
                       "Der Dienst meldet sich bei DSM als gewöhnlicher Benutzer an, den Sie für ihn angelegt haben. DSM 7 gibt einem Paket keinen eigenen Zugang.",
                       "Le service se connecte à DSM en utilisateur ordinaire que vous avez créé pour lui. DSM 7 ne donne aucun accès propre à un paquet.",
                       "El servicio entra en DSM como un usuario corriente que creaste para él. DSM 7 no da acceso propio a un paquete.",
                       "Il servizio accede a DSM come utente normale creato apposta. DSM 7 non dà accesso proprio a un pacchetto.",
                       "O serviço entra no DSM como um usuário comum criado para ele. O DSM 7 não dá acesso próprio a um pacote.",
                       "Usługa loguje się do DSM jako zwykły użytkownik założony dla niej. DSM 7 nie daje pakietowi własnego dostępu.",
                       "Servis, DSM'ye onun için açtığınız sıradan bir kullanıcı olarak bağlanır. DSM 7 bir pakete kendi erişimini vermez."],
    "fDsmUrl":        ["DSM address", "Адрес DSM", "Адреса DSM", "DSM-Adresse", "Adresse DSM", "Dirección de DSM", "Indirizzo DSM", "Endereço do DSM", "Adres DSM", "DSM adresi"],
    "hDsmUrl":        ["The service runs on this NAS, so this is normally the loopback address.",
                       "Служба работает на этом же NAS, так что обычно это локальный адрес.",
                       "Служба працює на цьому ж NAS, тож зазвичай це локальна адреса.",
                       "Der Dienst läuft auf diesem NAS, also ist das normalerweise die lokale Adresse.",
                       "Le service tourne sur ce NAS, donc c'est normalement l'adresse locale.",
                       "El servicio corre en este NAS, así que normalmente es la dirección local.",
                       "Il servizio gira su questo NAS, quindi di norma è l'indirizzo locale.",
                       "O serviço roda neste NAS, então normalmente é o endereço local.",
                       "Usługa działa na tym NAS-ie, więc zwykle jest to adres lokalny.",
                       "Servis bu NAS üzerinde çalışır, bu yüzden normalde yerel adrestir."],
    "fDsmUser":       ["DSM user", "Пользователь DSM", "Користувач DSM", "DSM-Benutzer", "Utilisateur DSM", "Usuario de DSM", "Utente DSM", "Usuário do DSM", "Użytkownik DSM", "DSM kullanıcısı"],
    "hDsmUser":       ["Give it access only to the packages it needs, and not to an administrator account.",
                       "Дайте ему доступ только к нужным пакетам и не берите администратора.",
                       "Дайте йому доступ лише до потрібних пакунків і не беріть адміністратора.",
                       "Geben Sie ihm nur Zugriff auf die nötigen Pakete, und kein Administratorkonto.",
                       "Ne lui donnez accès qu'aux paquets nécessaires, et pas un compte administrateur.",
                       "Dale acceso solo a los paquetes necesarios, y no una cuenta de administrador.",
                       "Dagli accesso solo ai pacchetti necessari, e non un account amministratore.",
                       "Dê acesso apenas aos pacotes necessários, e não uma conta de administrador.",
                       "Daj mu dostęp tylko do potrzebnych pakietów, i nie konto administratora.",
                       "Yalnızca gerekli paketlere erişim verin, yönetici hesabı değil."],
    "fDsmPassword":   ["DSM password", "Пароль DSM", "Пароль DSM", "DSM-Passwort", "Mot de passe DSM", "Contraseña de DSM", "Password DSM", "Senha do DSM", "Hasło DSM", "DSM parolası"],
    "fBotToken":      ["Bot token", "Токен бота", "Токен бота", "Bot-Token", "Jeton du bot", "Token del bot", "Token del bot", "Token do bot", "Token bota", "Bot jetonu"],
    "hBotToken":      ["The one @BotFather gave you.", "Тот, что выдал @BotFather.", "Той, що видав @BotFather.", "Der von @BotFather.", "Celui que @BotFather vous a donné.", "El que te dio @BotFather.", "Quello che ti ha dato @BotFather.", "O que o @BotFather lhe deu.", "Ten od @BotFather.", "@BotFather'ın verdiği."],
    "fAllowedIds":    ["Allowed Telegram ids", "Разрешённые id в Telegram", "Дозволені id у Telegram", "Erlaubte Telegram-IDs", "Identifiants Telegram autorisés", "Ids de Telegram permitidos", "Id Telegram consentiti", "Ids do Telegram permitidos", "Dozwolone identyfikatory Telegrama", "İzin verilen Telegram kimlikleri"],
    "hAllowedIds":    ["Numbers separated by commas. An empty list lets nobody in, not everybody.",
                       "Числа через запятую. Пустой список не пускает никого, а не всех.",
                       "Числа через кому. Порожній список не пускає нікого, а не всіх.",
                       "Zahlen durch Kommas getrennt. Eine leere Liste lässt niemanden herein, nicht jeden.",
                       "Des nombres séparés par des virgules. Une liste vide ne laisse entrer personne, pas tout le monde.",
                       "Números separados por comas. Una lista vacía no deja entrar a nadie, no a todos.",
                       "Numeri separati da virgole. Un elenco vuoto non fa entrare nessuno, non tutti.",
                       "Números separados por vírgulas. Uma lista vazia não deixa ninguém entrar, não todos.",
                       "Liczby oddzielone przecinkami. Pusta lista nie wpuszcza nikogo, a nie wszystkich.",
                       "Virgülle ayrılmış sayılar. Boş liste kimseyi almaz, herkesi değil."],
    "fListen":        ["Service port", "Порт службы", "Порт служби", "Dienst-Port", "Port du service", "Puerto del servicio", "Porta del servizio", "Porta do serviço", "Port usługi", "Servis portu"],
    "hListen":        ["Loopback only: the reverse proxy reaches the service through localhost.",
                       "Только локально: обратный прокси ходит к службе через localhost.",
                       "Лише локально: зворотний проксі ходить до служби через localhost.",
                       "Nur lokal: der Reverse-Proxy erreicht den Dienst über localhost.",
                       "En local uniquement : le proxy inverse joint le service via localhost.",
                       "Solo local: el proxy inverso llega al servicio por localhost.",
                       "Solo locale: il proxy inverso raggiunge il servizio via localhost.",
                       "Apenas local: o proxy reverso alcança o serviço por localhost.",
                       "Tylko lokalnie: odwrotne proxy sięga usługi przez localhost.",
                       "Yalnızca yerel: ters vekil servise localhost üzerinden ulaşır."],
    "secretSet":      ["Set. Leave empty to keep it.", "Задан. Оставьте пустым, чтобы не менять.", "Задано. Залиште порожнім, щоб не змінювати.", "Gesetzt. Leer lassen, um es zu behalten.", "Défini. Laissez vide pour le garder.", "Definido. Déjalo vacío para conservarlo.", "Impostato. Lascia vuoto per mantenerlo.", "Definido. Deixe vazio para manter.", "Ustawione. Zostaw puste, aby zachować.", "Ayarlı. Korumak için boş bırakın."],
    "secretUnset":    ["Not set yet.", "Ещё не задан.", "Ще не задано.", "Noch nicht gesetzt.", "Pas encore défini.", "Aún no definido.", "Non ancora impostato.", "Ainda não definido.", "Jeszcze nieustawione.", "Henüz ayarlanmadı."],
    "notify":         ["Notifications", "Уведомления", "Сповіщення", "Benachrichtigungen", "Notifications", "Notificaciones", "Notifiche", "Notificações", "Powiadomienia", "Bildirimler"],
    "notifyHint":     ["What the bot may write without being asked. Applies at once.", "Что бот может написать без спроса. Применяется сразу.", "Що бот може написати без запиту. Застосовується одразу.", "Was der Bot ungefragt schreiben darf. Gilt sofort.", "Ce que le bot peut écrire sans qu'on le lui demande. Prend effet aussitôt.", "Lo que el bot puede escribir sin que se lo pidan. Se aplica al instante.", "Cosa il bot può scrivere senza che glielo si chieda. Vale subito.", "O que o bot pode escrever sem ser perguntado. Vale de imediato.", "Co bot może napisać bez pytania. Działa od razu.", "Botun sormadan ne yazabileceği. Hemen geçerli olur."],
    "notifyOff":      ["Nothing", "Ничего", "Нічого", "Nichts", "Rien", "Nada", "Niente", "Nada", "Nic", "Hiçbir şey"],
    "notifyOffHint":  ["The bot answers, but never writes first", "Бот отвечает, но первым не пишет", "Бот відповідає, але першим не пише", "Der Bot antwortet, schreibt aber nie zuerst", "Le bot répond, mais n'écrit jamais le premier", "El bot responde, pero nunca escribe primero", "Il bot risponde, ma non scrive mai per primo", "O bot responde, mas nunca escreve primeiro", "Bot odpowiada, ale nigdy nie pisze pierwszy", "Bot yanıt verir ama ilk yazan olmaz"],
    "notifyDown":     ["Downloads only", "Только загрузки", "Лише завантаження", "Nur Downloads", "Téléchargements seulement", "Solo descargas", "Solo download", "Apenas downloads", "Tylko pobierania", "Yalnızca indirmeler"],
    "notifyDownHint": ["A message when a download finishes or fails", "Сообщение, когда загрузка завершилась или упала", "Повідомлення, коли завантаження завершилось або впало", "Eine Nachricht, wenn ein Download endet oder scheitert", "Un message quand un téléchargement se termine ou échoue", "Un mensaje cuando una descarga termina o falla", "Un messaggio quando un download finisce o fallisce", "Uma mensagem quando um download termina ou falha", "Wiadomość, gdy pobieranie się kończy lub nie udaje", "Bir indirme bittiğinde veya başarısız olduğunda mesaj"],
    "notifyAll":      ["Everything", "Всё", "Усе", "Alles", "Tout", "Todo", "Tutto", "Tudo", "Wszystko", "Her şey"],
    "notifyAllHint":  ["Also what the NAS announces itself: disks, security, updates", "И то, о чём сообщает сам NAS: диски, безопасность, обновления", "І те, про що повідомляє сам NAS: диски, безпека, оновлення", "Auch was das NAS selbst meldet: Platten, Sicherheit, Updates", "Aussi ce que le NAS annonce lui-même : disques, sécurité, mises à jour", "También lo que anuncia el propio NAS: discos, seguridad, actualizaciones", "Anche ciò che annuncia il NAS stesso: dischi, sicurezza, aggiornamenti", "Também o que o próprio NAS anuncia: discos, segurança, atualizações", "Także to, co zgłasza sam NAS: dyski, bezpieczeństwo, aktualizacje", "NAS'ın kendi bildirdikleri de: diskler, güvenlik, güncellemeler"],
    "address":        ["Public address", "Публичный адрес", "Публічна адреса", "Öffentliche Adresse", "Adresse publique", "Dirección pública", "Indirizzo pubblico", "Endereço público", "Adres publiczny", "Genel adres"],
    "addressHint":    ["Telegram opens a Mini App only over a public HTTPS address with a real certificate.", "Telegram открывает Mini App только по публичному адресу HTTPS с настоящим сертификатом.", "Telegram відкриває Mini App лише за публічною адресою HTTPS зі справжнім сертифікатом.", "Telegram öffnet eine Mini App nur über eine öffentliche HTTPS-Adresse mit echtem Zertifikat.", "Telegram n'ouvre une Mini App que sur une adresse HTTPS publique avec un vrai certificat.", "Telegram abre una Mini App solo por una dirección HTTPS pública con un certificado real.", "Telegram apre una Mini App solo su un indirizzo HTTPS pubblico con un certificato vero.", "O Telegram abre um Mini App apenas por um endereço HTTPS público com certificado real.", "Telegram otwiera Mini App tylko pod publicznym adresem HTTPS z prawdziwym certyfikatem.", "Telegram bir Mini App'i yalnızca gerçek sertifikalı genel bir HTTPS adresinden açar."],
    "addrCurrent":    ["Configured now", "Сейчас настроен", "Зараз налаштовано", "Derzeit eingestellt", "Configurée actuellement", "Configurada ahora", "Configurato ora", "Configurado agora", "Obecnie ustawiony", "Şu an ayarlı"],
    "addrExternal":   ["External address of the NAS", "Внешний адрес NAS", "Зовнішня адреса NAS", "Externe Adresse des NAS", "Adresse externe du NAS", "Dirección externa del NAS", "Indirizzo esterno del NAS", "Endereço externo do NAS", "Zewnętrzny adres NAS-a", "NAS'ın dış adresi"],
    "addrNone":       ["not set", "не задан", "не задано", "nicht gesetzt", "non définie", "sin definir", "non impostato", "não definido", "nieustawiony", "ayarlı değil"],
    "addrMatched":    ["A reverse proxy rule already sends this name to the service.", "Правило обратного прокси уже направляет это имя в службу.", "Правило зворотного проксі вже спрямовує це ім'я до служби.", "Eine Reverse-Proxy-Regel schickt diesen Namen bereits an den Dienst.", "Une règle de proxy inverse envoie déjà ce nom au service.", "Una regla de proxy inverso ya envía este nombre al servicio.", "Una regola di proxy inverso manda già questo nome al servizio.", "Uma regra de proxy reverso já envia este nome ao serviço.", "Reguła odwrotnego proxy już kieruje tę nazwę do usługi.", "Bir ters vekil kuralı bu adı zaten servise yönlendiriyor."],
    "addrCreate":     ["Point a name at the service", "Направить имя в службу", "Спрямувати ім'я до служби", "Einen Namen auf den Dienst richten", "Diriger un nom vers le service", "Dirigir un nombre al servicio", "Puntare un nome al servizio", "Apontar um nome ao serviço", "Skieruj nazwę do usługi", "Bir adı servise yönlendir"],
    "addrFqdnHint":   ["The name has to resolve to the external address above, and ports 80 and 443 have to reach this NAS.", "Имя должно указывать на внешний адрес выше, а порты 80 и 443 — доходить до этого NAS.", "Ім'я має вказувати на зовнішню адресу вище, а порти 80 і 443 — досягати цього NAS.", "Der Name muss auf die obige externe Adresse zeigen, und die Ports 80 und 443 müssen dieses NAS erreichen.", "Le nom doit pointer vers l'adresse externe ci-dessus, et les ports 80 et 443 doivent atteindre ce NAS.", "El nombre debe apuntar a la dirección externa de arriba, y los puertos 80 y 443 deben llegar a este NAS.", "Il nome deve puntare all'indirizzo esterno qui sopra, e le porte 80 e 443 devono raggiungere questo NAS.", "O nome precisa apontar para o endereço externo acima, e as portas 80 e 443 precisam alcançar este NAS.", "Nazwa musi wskazywać na powyższy adres zewnętrzny, a porty 80 i 443 muszą docierać do tego NAS-a.", "Ad yukarıdaki dış adrese işaret etmeli ve 80 ile 443 portları bu NAS'a ulaşmalı."],
    "addrCreateBtn":  ["Create the rule", "Создать правило", "Створити правило", "Regel erstellen", "Créer la règle", "Crear la regla", "Crea la regola", "Criar a regra", "Utwórz regułę", "Kuralı oluştur"],
    "addrDdns":       ["DDNS names DSM keeps up to date", "Имена DDNS, которые обновляет DSM", "Імена DDNS, які оновлює DSM", "DDNS-Namen, die DSM aktuell hält", "Noms DDNS que DSM tient à jour", "Nombres DDNS que DSM mantiene al día", "Nomi DDNS che DSM tiene aggiornati", "Nomes DDNS que o DSM mantém em dia", "Nazwy DDNS, które DSM utrzymuje", "DSM'nin güncel tuttuğu DDNS adları"],
    "addrNoDdns":     ["None. Control Panel → External Access → DDNS has a free one from Synology.", "Ни одного. В Панели управления → Внешний доступ → DDNS есть бесплатное имя от Synology.", "Жодного. У Панелі керування → Зовнішній доступ → DDNS є безкоштовне ім'я від Synology.", "Keine. Systemsteuerung → Externer Zugriff → DDNS bietet einen kostenlosen von Synology.", "Aucun. Panneau de configuration → Accès externe → DDNS en propose un gratuit de Synology.", "Ninguno. Panel de control → Acceso externo → DDNS ofrece uno gratuito de Synology.", "Nessuno. Pannello di controllo → Accesso esterno → DDNS ne offre uno gratuito di Synology.", "Nenhum. Painel de controle → Acesso externo → DDNS tem um gratuito da Synology.", "Żadnych. Panel sterowania → Dostęp zewnętrzny → DDNS ma darmową od Synology.", "Yok. Denetim Masası → Dış Erişim → DDNS'te Synology'den ücretsiz bir tane var."],
    "save":           ["Save", "Сохранить", "Зберегти", "Speichern", "Enregistrer", "Guardar", "Salva", "Salvar", "Zapisz", "Kaydet"],
    "saving":         ["Saving…", "Сохранение…", "Збереження…", "Wird gespeichert…", "Enregistrement…", "Guardando…", "Salvataggio…", "Salvando…", "Zapisywanie…", "Kaydediliyor…"],
    "saved":          ["Saved.", "Сохранено.", "Збережено.", "Gespeichert.", "Enregistré.", "Guardado.", "Salvato.", "Salvo.", "Zapisano.", "Kaydedildi."],
    "restartNeeded":  ["Restart the package in Package Center for this to take effect.", "Чтобы это подействовало, перезапустите пакет в Центре пакетов.", "Щоб це подіяло, перезапустіть пакунок у Центрі пакунків.", "Starten Sie das Paket im Paket-Zentrum neu, damit es wirkt.", "Redémarrez le paquet dans le Centre de paquets pour que cela prenne effet.", "Reinicia el paquete en el Centro de paquetes para que surta efecto.", "Riavvia il pacchetto in Centro pacchetti perché abbia effetto.", "Reinicie o pacote no Centro de pacotes para isso valer.", "Uruchom pakiet ponownie w Centrum pakietów, aby to zadziałało.", "Etkili olması için paketi Paket Merkezi'nde yeniden başlatın."],
    "errGeneric":     ["It did not work. The details are in the service log.", "Не получилось. Подробности — в журнале службы.", "Не вийшло. Подробиці — у журналі служби.", "Es hat nicht geklappt. Die Einzelheiten stehen im Dienstprotokoll.", "Cela n'a pas marché. Les détails sont dans le journal du service.", "No funcionó. Los detalles están en el registro del servicio.", "Non ha funzionato. I dettagli sono nel registro del servizio.", "Não funcionou. Os detalhes estão no log do serviço.", "Nie udało się. Szczegóły są w dzienniku usługi.", "Olmadı. Ayrıntılar servis günlüğünde."],
    "errForbidden":   ["Only a DSM administrator can open this screen.", "Этот экран открывается только администратору DSM.", "Цей екран відкривається лише адміністратору DSM.", "Diesen Bildschirm kann nur ein DSM-Administrator öffnen.", "Seul un administrateur DSM peut ouvrir cet écran.", "Solo un administrador de DSM puede abrir esta pantalla.", "Solo un amministratore DSM può aprire questa schermata.", "Só um administrador do DSM pode abrir esta tela.", "Ten ekran może otworzyć tylko administrator DSM.", "Bu ekranı yalnızca bir DSM yöneticisi açabilir."],
}

LANGS = ["en", "ru", "uk", "de", "fr", "es", "it", "pt", "pl", "tr"]

out = {lang: {} for lang in LANGS}
for key, values in STRINGS.items():
    if len(values) != len(LANGS):
        raise SystemExit(f"{key}: {len(values)} translations, {len(LANGS)} languages expected")
    for lang, value in zip(LANGS, values):
        out[lang][key] = value

body = ["// Generated by tools/make-dsmui-i18n.py — do not edit by hand.",
        "//",
        "// The screen inside DSM is a plain page with no build step, so its",
        "// dictionary is generated and committed. English is the source of truth and",
        "// the fallback: a language without a string shows the English one, never a key.",
        "window.DSMMINI_I18N = " + json.dumps(out, ensure_ascii=False, indent=2) + ";", ""]

path = pathlib.Path(__file__).resolve().parent.parent / "spk" / "ui" / "i18n.js"
path.write_text("\n".join(body), encoding="utf-8")
print(f"{path}: {len(STRINGS)} strings x {len(LANGS)} languages")
