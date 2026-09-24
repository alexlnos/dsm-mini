# dsm-mini service setup for the spksrc generic installer.
#
# The settings live in one file in the package var, written by the install
# wizard and read on every start. The var survives an upgrade, so the answers
# given once are kept; afterwards they are edited from the settings window in
# the DSM main menu rather than by hand.

DSM_MINI="${SYNOPKG_PKGDEST}/bin/dsm-mini"
CONFIG="${SYNOPKG_PKGVAR}/config.env"

# Loopback only. The DSM reverse proxy reaches the service through localhost
# and publishes it, because Telegram opens a Mini App over HTTPS alone;
# listening on every interface would put the app on the local network too.
# 58080 rather than 8080, which SABnzbd holds in the SynoCommunity port list.
DEFAULT_LISTEN="127.0.0.1:58080"

SERVICE_COMMAND="${DSM_MINI}"
SVC_BACKGROUND=y
SVC_WRITE_PID=y
# The log is kept across restarts: a refusal logged just before an upgrade is
# often the only trace of why the upgrade was needed.
SVC_KEEP_LOG=y

# Values go into a file that is read with `.`, so they are single-quoted: a
# password with a space or a dollar sign would otherwise break the start.
put ()
{
    escaped=$(printf '%s' "$2" | sed "s/'/'\\\\''/g")
    printf "%s='%s'\n" "$1" "${escaped}" >> "${CONFIG}"
}

service_postinst ()
{
    # An upgrade keeps the file: the wizard is not shown then, and the file
    # may have been changed from the settings window since.
    if [ "${SYNOPKG_PKG_STATUS}" != "INSTALL" ]; then
        return 0
    fi

    umask 077
    : > "${CONFIG}"
    # The service runs on this NAS, so DSM is always reached through the
    # machine itself; the settings window can change it if DSM listens on a
    # port of its own.
    put DSM_URL "https://localhost:5001"
    put DSM_USER "${wizard_dsm_user}"
    put DSM_PASSWORD "${wizard_dsm_password}"
    put DSM_INSECURE_TLS "true"
    put TELEGRAM_BOT_TOKEN "${wizard_bot_token}"
    put ALLOWED_USER_IDS "${wizard_allowed_ids}"
    put PUBLIC_URL "${wizard_public_url}"
    put LISTEN_ADDR "${DEFAULT_LISTEN}"
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

    # The database and the task watcher state live next to the settings.
    STATE_DIR="${SYNOPKG_PKGVAR}"
    export STATE_DIR

    # The settings window is served by DSM's own web server, under a user
    # that cannot read config.env (0600). Its CGI still has to know the port,
    # so the port — and only the port — is written next to it.
    port="${LISTEN_ADDR##*:}"
    case "${port}" in
        ''|*[!0-9]*) port="${DEFAULT_LISTEN##*:}" ;;
    esac
    printf 'PORT=%s\n' "${port}" > "${SYNOPKG_PKGDEST}/app/backend.conf"
}
