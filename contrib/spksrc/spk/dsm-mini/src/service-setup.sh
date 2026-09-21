# Служба dsm-mini.
#
# Настройки живут отдельным файлом в var пакета: мастер установки пишет его
# один раз, дальше его можно править руками, не переустанавливая пакет.

DSM_MINI="${SYNOPKG_PKGDEST}/bin/dsm-mini"
CONFIG="${SYNOPKG_PKGVAR}/config.env"

SERVICE_COMMAND="${DSM_MINI}"
SVC_BACKGROUND=y
SVC_WRITE_PID=y

# Значение уходит в файл, который читается через `.`, поэтому кавычки
# обязательны: пароль с пробелом или знаком доллара иначе развалит запуск.
put ()
{
    escaped=$(printf '%s' "$2" | sed "s/'/'\\\\''/g")
    printf "%s='%s'\n" "$1" "$escaped" >> "${CONFIG}"
}

service_postinst ()
{
    if [ "${SYNOPKG_PKG_STATUS}" != "INSTALL" ]; then
        return 0
    fi

    umask 077
    : > "${CONFIG}"
    put DSM_URL "${wizard_dsm_url:-https://localhost:5001}"
    put DSM_USER "${wizard_dsm_user}"
    put DSM_PASSWORD "${wizard_dsm_password}"
    put DSM_INSECURE_TLS "true"
    put TELEGRAM_BOT_TOKEN "${wizard_bot_token}"
    put ALLOWED_USER_IDS "${wizard_allowed_ids}"
    put PUBLIC_URL "${wizard_public_url}"
    put LISTEN_ADDR ":${wizard_port:-${SERVICE_PORT}}"
    put LOG_LEVEL "info"
    chmod 600 "${CONFIG}"
}

service_prestart ()
{
    if [ ! -r "${CONFIG}" ]; then
        echo "Configuration not found: ${CONFIG}"
        return 1
    fi

    set -a
    . "${CONFIG}"
    set +a

    # База и состояние наблюдателя переживают обновление пакета.
    STATE_DIR="${SYNOPKG_PKGVAR}"
    export STATE_DIR
}
