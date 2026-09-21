# Рецепт для SynoCommunity

SynoCommunity — народный каталог пакетов Synology: его источник уже добавлен
у множества людей, и попасть туда значит попасть «в магазин» без партнёрской
программы Synology.

Собирают они всё сами, из исходников, своим тулчейном — поэтому наш
`tools/build-spk.sh` им не подходит и рядом живёт вот этот рецепт.

## Что здесь

```
cross/dsm-mini/     сборка бинарника: качает наш тег с GitHub и собирает Go
spk/dsm-mini/       сам пакет: описание, мастер установки, запуск службы
```

Собранный интерфейс Mini App лежит в репозитории и попадает в бинарник через
`go:embed`, поэтому Node для сборки не нужен — только Go. Это заметно
упрощает жизнь их сборочной ферме.

## Как проверить сборку

```bash
git clone https://github.com/SynoCommunity/spksrc
cd spksrc
cp -r /путь/к/dsm-mini/contrib/spksrc/cross/dsm-mini cross/
cp -r /путь/к/dsm-mini/contrib/spksrc/spk/dsm-mini spk/
make -C spk/dsm-mini arch-x64-7.1
```

Первая сборка тянет тулчейн и занимает время. Результат — в `packages/`.

## Как отправить

1. Форкнуть [SynoCommunity/spksrc](https://github.com/SynoCommunity/spksrc).
2. Положить обе папки, как выше.
3. Прогнать сборку хотя бы под одну архитектуру.
4. Открыть pull request; их требования — в
   [документации](https://docs.synocommunity.com/developer-guide/).

## При выпуске новой версии

Обновить в обоих `Makefile` номер версии (`PKG_VERS`, `SPK_VERS`), поднять
`SPK_REV` и пересчитать контрольные суммы исходников:

```bash
curl -sL https://github.com/alexlnos/dsm-mini/archive/refs/tags/vX.Y.Z.tar.gz -o s.tar.gz
for a in sha1 sha256 md5; do
  echo "dsm-mini-X.Y.Z.tar.gz ${a^^} $(openssl dgst -$a s.tar.gz | awk '{print $2}')"
done
```
