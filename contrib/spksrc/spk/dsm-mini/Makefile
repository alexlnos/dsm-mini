SPK_NAME = dsm-mini
SPK_VERS = 1.0.1
SPK_REV = 1
SPK_ICON = src/dsm-mini.png

DEPENDS = cross/dsm-mini

# Go does not build for PowerPC and ARMv5.
UNSUPPORTED_ARCHS = $(PPC_ARCHS) $(ARMv5_ARCHS)

MAINTAINER = alexlnos
MAINTAINER_URL = https://github.com/alexlnos
DESCRIPTION = Telegram bot with a Mini App to manage your Synology NAS: Download Station, File Station, disks, virtual machines and containers.
DESCRIPTION_RUS = Телеграм-бот с Mini App для управления Synology: загрузки, файлы, диски, виртуальные машины и контейнеры.
DESCRIPTION_GER = Telegram-Bot mit Mini App zur Verwaltung Ihres Synology NAS: Downloads, Dateien, Festplatten, virtuelle Maschinen und Container.
DESCRIPTION_FRE = Bot Telegram avec Mini App pour gérer votre NAS Synology : téléchargements, fichiers, disques, machines virtuelles et conteneurs.
DISPLAY_NAME = dsm-mini
CHANGELOG = "1. First release."
HOMEPAGE = https://github.com/alexlnos/dsm-mini
LICENSE = MIT
STARTABLE = yes

# The service listens locally only: the DSM reverse proxy exposes it, because
# Telegram opens a Mini App over HTTPS alone.
SERVICE_PORT = 8080
SERVICE_PORT_TITLE = dsm-mini

SERVICE_USER = auto
SERVICE_SETUP = src/service-setup.sh
WIZARDS_DIR = src/wizard/

include ../../mk/spksrc.spk.mk
