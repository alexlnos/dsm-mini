<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · **Español** · [Português](README_PT.md) · [Deutsch](README_DE.md) · [Français](README_FR.md) · [Italiano](README_IT.md) · [Türkçe](README_TR.md) · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Comprobaciones](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![Licencia MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Un bot de Telegram con Mini App para gobernar un NAS Synology doméstico:
descargas de Download Station y archivos de File Station directamente desde la
mensajería, sin VPN y sin la interfaz web de DSM.

Suelta un enlace magnet en el chat: el bot pregunta con botones dónde ponerlo y
lo pone en cola. Abre la aplicación y verás qué se está descargando, cuánto
falta, qué hay en los discos y cómo se encuentra el NAS.

<p align="center">
  <img src="docs/screenshots/es-home.webp" width="19%" alt="Resumen del NAS">
  <img src="docs/screenshots/es-downloads.webp" width="19%" alt="Descargas">
  <img src="docs/screenshots/es-task.webp" width="19%" alt="Una tarea">
  <img src="docs/screenshots/es-files.webp" width="19%" alt="Archivos">
  <img src="docs/screenshots/es-storage.webp" width="19%" alt="Almacenamiento">
</p>

> Funcionando: el bot, la Mini App y las notificaciones. Requiere DSM 7 o
> posterior: en DSM 6 el paquete no se instalará y allí nadie lo ha probado.
> Probado en DSM 7.2.2 con Download Station 4.1.2 y File Station 1.4.4.

## Qué sabe hacer

**Descargas**

- La lista de tareas: progreso, velocidad, tiempo restante, semillas y pares
- Pausar, reanudar, eliminar
- Añadir por enlace magnet, por enlace directo y por archivo `.torrent`
- Elegir la carpeta de destino, con una lista de acceso rápido configurable
- Elegir los archivos dentro de un torrent y su prioridad
- Un mensaje en el chat cuando una tarea termina o falla

**Archivos**

- Explorar carpetas, vistas previas, subir al NAS
- Renombrar, copiar, mover, eliminar

**Estado del NAS**

- Carga de CPU y memoria, tiempo encendido, red
- Discos, grupos y volúmenes: temperatura, espacio usado, salud
- Máquinas virtuales y contenedores: arrancar y parar
- El registro de eventos de DSM

**Idioma**

La aplicación y el bot hablan el idioma elegido en Telegram: inglés, ruso,
español, portugués, alemán, francés, italiano, turco, ucraniano, polaco. Un
idioma desconocido recibe inglés.

---

## Instalación

Media hora, y la mayor parte se va en el certificado, no en el servicio. Hace
falta un NAS Synology con DSM 7 y Download Station; nada más hay que instalar
de antemano.

> **Por qué hace falta una dirección pública.** Telegram abre una Mini App solo
> por `https://` con un certificado de verdad: un `192.168.…` local o uno
> autofirmado no abrirán. El bot en sí funciona sin dirección, solo que sin el
> botón.

**1. Crear el bot.** [@BotFather](https://t.me/BotFather) → `/newbot` → un
nombre y un usuario que termine en `bot`. Responde con un token; guárdalo, es
la contraseña de tu bot.

**2. Averiguar tu número.** [@userinfobot](https://t.me/userinfobot) → Start.
Responde con la línea `Id`.

**3. Crear un usuario de DSM para el servicio.** Panel de control → Usuario y
grupo → Crear. Acceso solo a Download Station y File Station, sin verificación
en dos pasos: un código de un solo uso no puede salir de un archivo de
configuración. No le des un administrador: el servicio puede borrar archivos.

**4. Conseguir dirección y certificado.** Sáltalo si el NAS ya tiene un dominio
con certificado válido.

- Panel de control → Acceso externo → DDNS → Añadir, proveedor `Synology`: sale
  algo como `alex-nas.synology.me`.
- En el router, redirige al NAS los puertos **80** y **443**. Sin el 80 no se
  emitirá el certificado, sin el 443 no abrirá la aplicación.
- Panel de control → Seguridad → Certificado → Añadir → de Let's Encrypt, para
  ese mismo nombre.

Compruébalo desde el móvil con datos: `https://alex-nas.synology.me:5001` debe
abrir DSM sin avisos.

**5. Instalar el paquete.** Centro de paquetes → Configuración → Orígenes de
paquetes → Añadir, nombre `dsm-mini` y la dirección de tu arquitectura:

- Intel y AMD, la mayoría de modelos: `https://alexlnos.github.io/dsm-mini/amd64.json`
- ARM, modelos básicos: `https://alexlnos.github.io/dsm-mini/arm64.json`

Luego Configuración → General → Nivel de confianza → **Cualquier editor**, e
instala **DSM mini (Telegram Mini App)** desde la sección **Comunidad**. ¿Dudas
con la arquitectura? Prueba `amd64`: un paquete que no encaja simplemente se
rechaza.

El instalador pide siete valores y explica cada uno mientras los pide: todo
para ellos se reunió en los pasos de arriba. O instala el `.spk` de las
[versiones](https://github.com/alexlnos/dsm-mini/releases) a mano, por Centro
de paquetes → Instalación manual.

**6. Apuntar la dirección al servicio.** Panel de control → Portal de inicio de
sesión → Avanzado → Proxy inverso → Crear. Origen: `HTTPS`, tu nombre, puerto
`443`. Destino: `HTTP`, `localhost`, puerto `8080`.

> No hagas proxy del puerto **80** para este nombre: por ahí DSM renueva el
> certificado, e interceptarlo rompe la renovación tres meses después.

**7. Comprobar.** `https://tu-direccion/healthz` en un navegador debe responder
`{"status":"ok"}`. Sin firma de Telegram no se entrega nada.

**8. Abrir la aplicación.** Busca el bot por su usuario, pulsa Start y junto al
campo de escritura aparece un botón **Descargas**. Mándale cualquier enlace
magnet: te ofrecerá carpetas con botones.

## Si algo ha salido mal

| Qué ves | De qué se trata | Qué hacer |
|---|---|---|
| El bot calla ante `/start` | Token incorrecto, o el paquete no está en marcha | Centro de paquetes → **DSM mini (Telegram Mini App)** → el registro |
| «El acceso a este bot está cerrado» | Tu ID no está en la lista | Pon el número del paso 2 en los IDs permitidos (ver «Cambiar los ajustes» abajo) |
| No hay botón de la aplicación | La dirección pública está vacía o no es `https://` | En el mismo sitio: el archivo de ajustes, luego reinicia el paquete |
| El botón está, la aplicación no abre | El proxy inverso o el certificado no funcionan | Abre `https://tu-direccion/healthz` en un navegador |
| «Abre la aplicación desde el bot» | La aplicación se abrió por enlace directo en un navegador | Es lo previsto: ábrela desde el bot |
| «Acceso denegado: tu ID de Telegram…» | El servicio no te reconoció | En los IDs permitidos solo dígitos, separados por comas |
| `authentication with DSM failed` en el registro | La contraseña, la 2FA o los permisos del usuario | Paso 3: contraseña sin erratas, 2FA apagada, aplicaciones permitidas |
| `Could not get the task list` en el registro | Download Station no está instalado o está denegado al usuario | Centro de paquetes y los permisos del paso 3 |
| El paquete se detiene nada más arrancar | Un ajuste está mal: el registro dice cuál | El motivo llega también al centro de notificaciones de DSM |

El registro del paquete es la fuente de verdad principal: dice exactamente qué
falta. Está en `/var/packages/dsm-mini/var/dsm-mini.log` y se abre desde el
Centro de paquetes. El registro va en inglés, la interfaz y los mensajes del bot
en tu idioma.

### Cambiar los ajustes

Todo lo que preguntó el asistente vive en un archivo del NAS,
`/var/packages/dsm-mini/var/config.env`, con permisos `600`.

Instalar el paquete sobre sí mismo **no** vuelve a preguntar: el asistente
corre al instalar, y una actualización deja el archivo a propósito — por eso
los ajustes sobreviven. Quedan dos caminos:

- **Editar el archivo por SSH** (Panel de control → Terminal y SNMP → activar
  SSH) y reiniciar el paquete en el Centro de paquetes. Así se conserva todo
  lo demás:

  ```bash
  sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='tu-nuevo-token'|" /var/packages/dsm-mini/var/config.env
  sudo synopkg restart dsm-mini
  ```

- **Desinstalar e instalar de nuevo**: el asistente lo pregunta todo otra vez.
  Junto con los ajustes se borra la base que está al lado, o sea las carpetas
  fijadas, el idioma y los últimos estados conocidos de las tareas.

## Actualizar

Si añadiste el origen de paquetes, el Centro de paquetes muestra la
actualización solo. Sin origen, descarga el `.spk` nuevo de las
[versiones](https://github.com/alexlnos/dsm-mini/releases) e instálalo encima.

Los ajustes y la base de datos se quedan en ambos casos: viven en el directorio
`var` del paquete, que una actualización no toca.

## Dónde se guardan los datos

Todo está en `/var/packages/dsm-mini/var/`:

- `config.env` — lo que preguntó el asistente, permisos `600`;
- `dsm-mini.db` — una base SQLite: las carpetas fijadas, el idioma y los últimos
  estados conocidos de las tareas, con los que el servicio sabe de qué ya ha
  avisado;
- `dsm-mini.log` — el registro.

Actualizar el paquete conserva los tres. Desinstalarlo los borra.

## Seguridad

El servicio está expuesto a internet y puede borrar archivos del NAS, así que:

- **Un usuario de DSM aparte**, no un administrador (paso 3).
- **Sin verificación en dos pasos** en esa cuenta: un código de un solo uso no
  puede funcionar desde un archivo de configuración.
- **Los IDs permitidos son una lista de admitidos.** Una lista vacía cierra el
  acceso a todos, no lo abre.
- Cada petición a `/api/` se comprueba dos veces: la firma de `initData` con una
  clave derivada del token del bot, y el identificador contra la lista. La firma
  solo demuestra que alguien abrió el bot, y abrirlo puede cualquiera.
- Hacia fuera va una frase genérica y los detalles al registro: un mensaje que
  dijera qué no cuadró exactamente en la firma sería una pista para falsificarla.
- El token del bot y la contraseña de DSM viven solo en `config.env`, en el
  propio NAS, con permisos `600`, y nunca llegan al registro: en un rechazo se
  escribe el motivo, nunca el valor.
- El servicio escucha solo en `127.0.0.1`, es decir desde el propio NAS. Todo lo
  que viene de fuera pasa por el proxy inverso de DSM, que además termina el TLS.

## Desarrollo

```bash
cd web && npm install && npm run build   # la Mini App acaba en internal/web/dist
cd .. && go build ./cmd/dsm-mini         # un binario con la aplicación incrustada
go test ./...
```

El frontend por separado, con recarga automática:

```bash
cd web && npm run dev    # habla con el backend en localhost:8080
```

Ejecutar el backend en local lee las mismas variables que el asistente del
paquete escribe en `config.env`. Va bien tenerlas en un archivo:

```bash
cp .env.example .env     # rellénalo
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

La interfaz compilada está en el repositorio (`internal/web/dist`), de donde la
recoge `go:embed`. Tras tocar el frontend, recompila y haz commit del resultado o
las comprobaciones no pasarán.

Las pruebas de integración van contra un NAS real y se saltan por defecto:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

Las pruebas que cambian el estado del NAS necesitan permiso aparte:
`DSM_TEST_MUTATIONS=1`, y para las operaciones de archivos también
`DSM_TEST_FOLDER`, la carpeta dentro de la cual se pueden crear archivos
temporales. Borran todo lo que crean.

Un idioma nuevo de la interfaz es un archivo de diccionario en cada lado:
`web/src/i18n/<código>.ts` e `internal/i18n/<código>.go`. No se puede olvidar una
cadena: en el frontend lo vigila el tipo, en el servidor una prueba.

El código, los comentarios y el registro del proyecto van en inglés; los demás
idiomas viven solo en los diccionarios. Los detalles, en [CLAUDE.md](CLAUDE.md).

## Particularidades de la API de Synology

La Web API de Synology se comporta a ratos de forma distinta a su documentación:
una misma acción responde en tres formatos diferentes, `limit = -1` tumba File
Station y el `_sid` al subir un archivo hay que pasarlo de otra manera que en el
resto. Todo lo averiguado en un NAS real está recogido en
[docs/synology-api.md](docs/synology-api.md).

## Licencia

[MIT](LICENSE)
