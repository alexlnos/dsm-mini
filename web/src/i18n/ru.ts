/**
 * Русский словарь — источник истины.
 *
 * Остальные языки описаны как `Record<Key, Phrase>`, поэтому пропущенный
 * ключ не соберётся: забыть перевод молча нельзя.
 *
 * Множественное число задаётся объектом с формами `Intl.PluralRules`
 * (`one`, `few`, `many`, `other`) — в русском их три, в английском две.
 */
export const ru = {
  // ── Общее ───────────────────────────────────────────────────────────────
  'common.back': 'Назад',
  'common.close': 'Закрыть',
  'common.loading': 'Загружаю…',
  'common.delete': 'Удалить',
  'common.copy': 'Копировать',
  'common.move': 'Перенести',
  'common.rename': 'Переименовать',
  'common.upload': 'Загрузить',
  'common.newFolder': 'Новая папка',
  'common.add': 'Добавить',
  'common.failed': 'Не получилось',
  'common.outOf': '{value} из {total}',
  'common.pickOnNas': 'Выбрать на NAS',
  'common.dash': '—',

  // ── Вкладки ─────────────────────────────────────────────────────────────
  'tab.home': 'Главная',
  'tab.downloads': 'Загрузки',
  'tab.files': 'Файлы',

  // ── Единицы и время ─────────────────────────────────────────────────────
  'unit.bytes': 'Б',
  'unit.kb': 'КБ',
  'unit.mb': 'МБ',
  'unit.gb': 'ГБ',
  'unit.tb': 'ТБ',
  'unit.perSecond': '{size}/с',
  'unit.days': '{value} дн',
  'unit.hours': '{value} ч',
  'unit.minutes': '{value} мин',
  'unit.seconds': '{value} с',
  'unit.hoursMinutes': '{hours} ч {minutes} мин',
  'unit.daysHours': '{days} дн {hours} ч',

  // ── Состояния задач ─────────────────────────────────────────────────────
  'status.waiting': 'В очереди',
  'status.downloading': 'Загружается',
  'status.paused': 'На паузе',
  'status.finishing': 'Завершается',
  'status.finished': 'Готово',
  'status.hashChecking': 'Проверка',
  'status.seeding': 'Раздаётся',
  'status.extracting': 'Распаковка',
  'status.error': 'Ошибка',
  'status.unknown': 'Неизвестно',
  'status.fetchingInfo': 'Получение сведений…',

  // ── Журнал: дни ─────────────────────────────────────────────────────────
  'day.today': 'Сегодня',
  'day.yesterday': 'Вчера',
  'day.earlier': 'Ранее',

  // ── Главный экран ───────────────────────────────────────────────────────
  'home.offline': 'NAS недоступен',
  'home.online': 'в сети',
  'home.noLink': 'нет связи',
  'home.connecting': 'подключаюсь…',
  'home.apps': 'Приложения',
  'home.notInstalled': 'не установлен',
  'home.notRunning': 'не запущен',
  'home.storage': 'Хранилище',
  'home.healthy': 'исправно',
  'home.needsAttention': 'требует внимания',
  'home.volume': 'Том {number}',
  'home.free': 'свободно {size}',
  'home.tasksNote': {
    one: '{count} задача',
    few: '{count} задачи',
    many: '{count} задач',
  },
  'home.activeNote': '{active} активных · {done} готовы',
  'home.filesNote': 'обзор и загрузка',
  'home.vmsNote': 'управление',
  'home.storageNote': '{disks} · {pools}',
  'home.logNote': 'журнал событий NAS',
  'home.storageFallbackNote': 'диски и тома',
  'app.downloads': 'Загрузки',
  'app.files': 'Файлы',
  'app.vms': 'Виртуальные машины',
  'app.containers': 'Контейнеры',
  'app.storage': 'Хранилище',
  'app.notifications': 'Уведомления',

  // ── Доступ ──────────────────────────────────────────────────────────────
  'access.forbidden': 'Доступ закрыт: ваш Telegram ID не в списке разрешённых.',
  'access.unauthorized': 'Откройте приложение через бота — Telegram не подтвердил вход.',
  'access.unavailable': 'Сервис недоступен',
  'error.http': 'Ошибка {status}',
  'error.noService': 'Не получилось связаться с сервисом',

  // ── Загрузки ────────────────────────────────────────────────────────────
  'downloads.title': 'Загрузки',
  'downloads.activeBadge': '{count} активных',
  'downloads.rx': 'Приём',
  'downloads.tx': 'Отдача',
  'downloads.freeOf': 'свободно {free} из {total}',
  'downloads.tabActive': 'Активные',
  'downloads.tabDone': 'Готовы',
  'downloads.tabAll': 'Все',
  'downloads.emptyTitle': 'Здесь пусто',
  'downloads.emptyDone': 'Завершённые задачи появятся тут, когда что-нибудь докачается.',
  'downloads.emptyOther': 'Пришлите ссылку боту или добавьте её кнопкой ниже.',
  'downloads.addAria': 'Добавить загрузку',
  'downloads.detailsAria': 'Подробности задачи',
  'downloads.resume': 'Возобновить',
  'downloads.pause': 'Остановить',

  // ── Новая загрузка ──────────────────────────────────────────────────────
  'add.title': 'Новая загрузка',
  'add.linkLabel': 'Ссылка',
  'add.linkPlaceholder': 'magnet:?xt=urn:btih:… или https://…',
  'add.clear': 'Очистить',
  'add.fromClipboard': 'Из буфера',
  'add.clipboardUnavailable': 'Буфер обмена недоступен — вставьте ссылку вручную',
  'add.whereTo': 'Куда положить',
  'add.configure': 'Настроить',
  'add.openFolderAria': 'Открыть {folder} и выбрать вложенную папку',
  'add.pickOnNas': 'Выбрать папку на NAS',
  'add.submit': 'Поставить в очередь',
  'add.submitting': 'Ставлю…',
  'add.needLink': 'Вставьте ссылку',
  'add.failed': 'Не получилось поставить задачу',

  // ── Папки назначения ────────────────────────────────────────────────────
  'folders.title': 'Папки',
  'folders.hint': 'Появятся на экране добавления в этом порядке. Изменения сохраняются сразу.',
  'folders.pinned': 'Закреплённые',
  'folders.removeAria': 'Убрать {folder}',
  'folders.up': 'Выше',
  'folders.down': 'Ниже',
  'folders.empty': 'Ничего не закреплено — на экране добавления покажутся недавние папки.',
  'folders.addSection': 'Добавить',
  'folders.inUse': 'Используются сейчас',
  'folders.showRecent': 'Показывать недавние',
  'folders.showRecentHint': 'Папки последних задач появятся под закреплёнными',
  'folders.alreadyPinned': 'Такая папка уже закреплена',
  'folders.loadFailed': 'Не загрузить настройки',
  'folders.saveFailed': 'Не сохранить',

  // ── Файлы ───────────────────────────────────────────────────────────────
  'files.shares': 'Общие папки',
  'files.objects': {
    one: '{count} объект',
    few: '{count} объекта',
    many: '{count} объектов',
  },
  'files.up': 'На уровень выше',
  'files.readFailed': 'Не прочитать папку',
  'files.confirmDelete': 'Удалить безвозвратно: {names}?',
  'files.newName': 'Новое имя',
  'files.newFolderName': 'Имя новой папки',
  'files.rootUpload': 'Выберите папку: в корень загружать нельзя',
  'files.someSkipped': 'Часть файлов пропущена: в папке назначения уже есть файлы с такими именами.',
  'files.stillRunning': 'Операция ещё идёт на NAS — обновите папку позже.',
  'files.transferring': 'Переношу…',
  'files.openFolderFirst': 'Откройте папку',
  'files.copyHere': 'Скопировать сюда · {count}',
  'files.moveHere': 'Перенести сюда · {count}',
  'files.pickHere': 'Выбрать {folder}',
  'files.folderKind': 'папка',

  // ── Виртуальные машины ──────────────────────────────────────────────────
  'vms.title': 'Машины',
  'vms.freeRam': 'Свободно RAM',
  'vms.usedVcpu': 'Занято vCPU',
  'vms.specs': '{vcpu} vCPU · {ram} · диск {disk}',
  'vms.start': 'Запустить',
  'vms.stop': 'Выключить',
  'vms.running': 'Работает',
  'vms.stopped': 'Выключена',
  'vms.autorun': 'автозапуск',
  'vms.empty': 'Машин нет.',
  'vms.listFailed': 'Не получить список машин',
  'vms.confirmShutdown':
    'Выключить «{name}»? Гостевой системе будет отправлена команда завершения работы.',

  // ── Контейнеры ──────────────────────────────────────────────────────────
  'containers.title': 'Контейнеры',
  'containers.emptyTitle': 'Контейнеров нет',
  'containers.emptyText': 'Container Manager работает, но ни один контейнер не создан.',
  'containers.start': 'Запустить',
  'containers.stop': 'Остановить',
  'containers.running': 'Работает',
  'containers.stopped': 'Остановлен',
  'containers.restart': 'перезапустить',
  'containers.listFailed': 'Не получить список контейнеров',

  // ── Хранилище ───────────────────────────────────────────────────────────
  'storage.title': 'Хранилище',
  'storage.healthy': 'исправно',
  'storage.attention': 'внимание',
  'storage.disksCount': {
    one: '{count} диск',
    few: '{count} диска',
    many: '{count} дисков',
  },
  'storage.poolsCount': {
    one: '{count} пул',
    few: '{count} пула',
    many: '{count} пулов',
  },
  'storage.volumesCount': {
    one: '{count} том',
    few: '{count} тома',
    many: '{count} томов',
  },
  'storage.baysSata': 'Отсеки SATA',
  'storage.slotsM2': 'Слоты M.2',
  'storage.volumes': 'Тома',
  'storage.pools': 'Пулы',
  'storage.disks': 'Диски',
  'storage.volumeOk': 'исправен',
  'storage.pool': 'Пул {number}',
  'storage.failed': 'Не получить состояние хранилища',

  // ── Журнал событий ──────────────────────────────────────────────────────
  'log.title': 'События',
  'log.subtitle': 'журнал NAS',
  'log.records': 'Записей',
  'log.problems': 'Проблемных',
  'log.tabAll': 'Все',
  'log.tabProblems': 'Важные',
  'log.emptyProblems': 'Ошибок и предупреждений нет.',
  'log.empty': 'Журнал пуст.',
  'log.failed': 'Не получить журнал',

  // ── Экран задачи ────────────────────────────────────────────────────────
  'task.priorityLow': 'Низкий',
  'task.priorityNormal': 'Обычный',
  'task.priorityHigh': 'Высокий',
  'task.confirmDelete': 'Удалить задачу? Это необратимо.',
  'task.etaLeft': ' · осталось {eta}',
  'task.resume': 'Возобновить',
  'task.pause': 'Пауза',
  'task.seedsPeers': 'Сиды / пиры',
  'task.uploaded': 'Роздано',
  'task.folder': 'Папка',
  'task.collapse': 'Свернуть',
  'task.moveTo': 'Перенести',
  'task.moveHint': 'перенести',
  'task.files': 'Файлы',
  'task.filesClosed':
    'Состав раздачи виден, только пока задача качается или раздаётся: NAS закрывает её при остановке. Возобновите задачу, чтобы увидеть файлы.',
  'task.filesLoading': 'Получаю список файлов… NAS отдаёт его не сразу после запуска.',
  'task.filesEmpty': 'Список файлов пуст.',
  'task.moveFailed': 'Не сменить папку',
  'task.trackers': 'Трекеры',
  'task.trackerLine': '{status} · сидов {seeds} · пиров {peers}',

  // ── Просмотр файла ──────────────────────────────────────────────────────
  'standalone.title': 'Откройте через Telegram',
  'standalone.text': 'Приложение работает внутри бота: Telegram передаёт подпись, по которой сервис узнаёт, кто вы. По прямой ссылке в браузере оно не откроется.',

  'preview.failed': 'Не удалось открыть файл',
  'preview.unsupported': 'Просмотр для таких файлов не предусмотрен.',
}

/** Ключ словаря: перечисление всех строк интерфейса. */
export type Key = keyof typeof ru

/** Строка или набор форм множественного числа. */
export type Phrase = string | Partial<Record<Intl.LDMLPluralRule, string>>
