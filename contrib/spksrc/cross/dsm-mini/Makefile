PKG_NAME = dsm-mini
PKG_VERS = 0.1.0
PKG_EXT = tar.gz
PKG_DIST_NAME = v$(PKG_VERS).$(PKG_EXT)
PKG_DIST_SITE = https://github.com/alexlnos/dsm-mini/archive/refs/tags
PKG_DIST_FILE = $(PKG_NAME)-$(PKG_VERS).$(PKG_EXT)
PKG_DIR = $(PKG_NAME)-$(PKG_VERS)

# Go не собирает под PowerPC и ARMv5.
UNSUPPORTED_ARCHS = $(PPC_ARCHS) $(ARMv5_ARCHS)

include ../../mk/spksrc.common.mk

BUILD_DEPENDS = native/go
NATIVE_GO = native/go

HOMEPAGE = https://github.com/alexlnos/dsm-mini
COMMENT  = Telegram bot with a Mini App to manage your Synology NAS.
LICENSE  = MIT

COMPILE_TARGET = dsm_mini_compile
INSTALL_TARGET = dsm_mini_install

include ../../mk/spksrc.cross-go.mk

# Интерфейс Mini App лежит в репозитории собранным и попадает в бинарник
# через go:embed — сборщик Node здесь не нужен.
.PHONY: dsm_mini_compile
dsm_mini_compile:
	@$(MSG) - Compile dsm-mini
	@cd $(WORK_DIR)/$(PKG_DIR) && env $(ENV) go build -trimpath \
		-ldflags "-s -w -X main.version=$(PKG_VERS)" \
		-o dsm-mini ./cmd/dsm-mini

.PHONY: dsm_mini_install
dsm_mini_install:
	@$(MSG) - Install dsm-mini
	@install -m 755 -d $(STAGING_INSTALL_PREFIX)/bin
	@install -m 755 $(WORK_DIR)/$(PKG_DIR)/dsm-mini $(STAGING_INSTALL_PREFIX)/bin/
