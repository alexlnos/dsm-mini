<img src="docs/icon.png" width="88" alt="">

# dsm-mini

[English](README.md) · [Русский](README_RU.md) · [Español](README_ES.md) · [Português](README_PT.md) · [Deutsch](README_DE.md) · [Français](README_FR.md) · [Italiano](README_IT.md) · **Türkçe** · [Українська](README_UK.md) · [Polski](README_PL.md)

[![Denetimler](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml/badge.svg)](https://github.com/alexlnos/dsm-mini/actions/workflows/ci.yml)
[![MIT lisansı](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Ev tipi bir Synology NAS'ı yönetmek için Mini App'li bir Telegram botu: Download
Station indirmeleri ve File Station dosyaları doğrudan mesajlaşmadan, VPN ve DSM
web arayüzü olmadan.

Sohbete bir magnet bağlantısı atın — bot düğmelerle nereye indireceğini sorar ve
kuyruğa alır. Uygulamayı açın — ne indiğini, ne kadar kaldığını, disklerde ne
olduğunu ve NAS'ın durumunu görürsünüz.

<p align="center">
  <img src="docs/screenshots/tr-home.webp" width="19%" alt="NAS genel görünümü">
  <img src="docs/screenshots/tr-downloads.webp" width="19%" alt="İndirmeler">
  <img src="docs/screenshots/tr-task.webp" width="19%" alt="Bir görev">
  <img src="docs/screenshots/tr-files.webp" width="19%" alt="Dosyalar">
  <img src="docs/screenshots/tr-storage.webp" width="19%" alt="Depolama">
</p>

> Çalışıyor: bot, Mini App ve bildirimler. DSM 7 ya da daha yenisi gerekir —
> DSM 6'ya paket kurulmaz ve orada hiç denenmedi. DSM 7.2.2 üzerinde Download
> Station 4.1.2 ve File Station 1.4.4 ile denendi.

## Neler yapabiliyor

**İndirmeler**

- Görev listesi: ilerleme, hız, kalan süre, kaynaklar ve eşler
- Duraklatma, sürdürme, silme
- Magnet bağlantısı, doğrudan bağlantı ve `.torrent` dosyasıyla ekleme
- Hedef klasörü seçme, ayarlanabilir bir hızlı erişim listesiyle
- Torrent içindeki dosyaları ve önceliklerini seçme
- Bir görev bittiğinde ya da başarısız olduğunda sohbete mesaj

**Dosyalar**

- Klasörlere göz atma, önizleme, NAS'a yükleme
- Yeniden adlandırma, kopyalama, taşıma, silme

**NAS durumu**

- İşlemci ve bellek yükü, çalışma süresi, ağ
- Diskler, havuzlar ve birimler: sıcaklık, kullanılan alan, sağlık
- Sanal makineler ve kapsayıcılar: başlatma ve durdurma
- DSM olay günlüğü

**Bildirimler ve ayarlar**

- DSM'nin kendi bildirdikleri — güvenlik danışmanı, diskler, güncellemeler — sohbete ulaşır
- Botun ne gönderebileceğinin seçimi: hiçbir şey, yalnızca indirmeler, her şey
- DSM ana menüsünde kendi ekranı: her ayar, SSH ile dosya düzenlemeden

**Dil**

Uygulama ve bot, Telegram'da seçilen dili konuşur: İngilizce, Rusça, İspanyolca,
Portekizce, Almanca, Fransızca, İtalyanca, Türkçe, Ukraynaca, Lehçe. Bilinmeyen
bir dil İngilizce alır.

---

## Kurulum

Yarım saat, ve çoğu servise değil sertifikaya gidiyor. DSM 7'li bir Synology
NAS ve Download Station gerekiyor; başka hiçbir şeyi önceden kurmak gerekmez.

> **Genel bir adres neden gerekli.** Telegram bir Mini App'i yalnızca gerçek
> sertifikalı `https://` üzerinden açar: yerel bir `192.168.…` ya da kendinden
> imzalı bir sertifika açılmaz. Botun kendisi adressiz de çalışır, sadece
> düğmesiz.

**1. Botu oluşturmak.** [@BotFather](https://t.me/BotFather) → `/newbot` → bir
ad ve `bot` ile biten bir kullanıcı adı. Yanıt olarak bir belirteç gelir;
saklayın, o botunuzun parolasıdır.

**2. Kendi numaranızı öğrenmek.** [@userinfobot](https://t.me/userinfobot) →
Start. `Id` satırıyla yanıt verir.

**3. Servis için bir DSM kullanıcısı açmak.** Denetim Masası → Kullanıcı ve
Grup → Oluştur. Yalnızca Download Station ve File Station erişimi, iki aşamalı
doğrulama olmadan: tek kullanımlık kod bir yapılandırma dosyasından gelemez.
Yönetici vermeyin: servis dosya silebiliyor.

**4. Adres ve sertifika almak.** NAS'ta geçerli sertifikalı bir alan adı zaten
varsa atlayın.

- Denetim Masası → Harici Erişim → DDNS → Ekle, sağlayıcı `Synology`:
  `alex-nas.synology.me` gibi bir şey çıkar.
- Yönlendiricide **80** ve **443** numaralı bağlantı noktalarını NAS'a
  yönlendirin. 80 olmadan sertifika verilmez, 443 olmadan uygulama açılmaz.
- Denetim Masası → Güvenlik → Sertifika → Ekle → Let's Encrypt'ten, aynı ad
  için.

Telefondan mobil veriyle deneyin: `https://alex-nas.synology.me:5001` DSM'yi
uyarısız açmalı.

**5. Paketi kurmak.** Paket Merkezi → Ayarlar → Paket Kaynakları → Ekle, ad
`dsm-mini` ve mimarinize göre adres:

- Intel ve AMD, modellerin çoğu: `https://alexlnos.github.io/dsm-mini/amd64.json`
- ARM, giriş seviyesi modeller: `https://alexlnos.github.io/dsm-mini/arm64.json`

Sonra Ayarlar → Genel → Güven Düzeyi → **Herhangi bir yayıncı**, ve
**Topluluk** bölümünden **DSM mini (Telegram Mini App)** kurun. Mimariden emin
değil misiniz? `amd64` deneyin: uymayan bir paket zaten reddedilir.

Yükleyici beş değer sorar ve her birini sorarken açıklar — hepsi için gereken
yukarıdaki adımlarda toplandı. Ya da `.spk` dosyasını
[sürümlerden](https://github.com/alexlnos/dsm-mini/releases) elle kurun, Paket
Merkezi → Elle Yükleme ile.

**6. Adresi servise yönlendirmek.** Denetim Masası → Oturum Açma Portalı →
Gelişmiş → Ters Proxy → Oluştur. Kaynak: `HTTPS`, sizin adınız, bağlantı
noktası `443`. Hedef: `HTTP`, `localhost`, bağlantı noktası `8080`.

> Bu ad için **80** numaralı bağlantı noktasını proxy'lemeyin: DSM sertifikayı
> oradan yeniliyor, araya girmek yenilemeyi üç ay sonra bozar.

**7. Denemek.** Tarayıcıda `https://adresiniz/healthz` `{"status":"ok"}`
yanıtını vermeli. Telegram imzası olmadan dışarıya hiçbir şey verilmez.

**8. Uygulamayı açmak.** Botu kullanıcı adından bulun, Start'a basın — yazma
alanının yanında bir **İndirmeler** düğmesi belirir. Ona herhangi bir magnet
bağlantısı gönderin, klasörleri düğmelerle önerir.

## Bir şeyler ters gittiyse

| Gördüğünüz | Sorun ne | Ne yapmalı |
|---|---|---|
| Bot `/start` komutuna susuyor | Yanlış belirteç ya da paket çalışmıyor | Paket Merkezi → **DSM mini (Telegram Mini App)** → günlük |
| «Bu bota erişim kapalı» | Kimliğiniz listede değil | 2. adımdaki numarayı izin verilen kimliklere ekleyin (aşağıda «Ayarları değiştirmek») |
| Uygulama düğmesi yok | Genel adres boş ya da `https://` değil | Aynı yer: ayar dosyası, sonra paketi yeniden başlatın |
| Düğme var, uygulama açılmıyor | Ters proxy ya da sertifika çalışmıyor | Tarayıcıda `https://adresiniz/healthz` açın |
| «Uygulamayı bot üzerinden açın» | Uygulama tarayıcıda doğrudan bağlantıyla açıldı | Böyle olması gerekiyor: bottan açın |
| «Erişim reddedildi: Telegram kimliğiniz…» | Servis sizi tanımadı | İzin verilen kimliklerde yalnızca rakamlar, virgülle ayrılmış |
| Günlükte `authentication with DSM failed` | Parola, 2FA ya da kullanıcının yetkileri | 3. adım: parola yazım hatasız, 2FA kapalı, uygulamalara izinli |
| Günlükte `Could not get the task list` | Download Station kurulu değil ya da kullanıcıya yasak | Paket Merkezi ve 3. adımdaki yetkiler |
| Paket başlar başlamaz duruyor | Bir ayar yanlış — günlük hangisi olduğunu söyler | Neden DSM bildirim merkezine de düşer |

Paketin günlüğü ana doğruluk kaynağıdır: neyin eksik olduğunu açıkça söyler.
`/var/packages/dsm-mini/var/dsm-mini.log` dosyasında durur ve Paket
Merkezi'nden açılır. Günlük İngilizce tutulur, arayüz ve bot mesajları sizin
dilinizde olur.

### Ayarları değiştirmek

Sihirbazın sorduğu her şey NAS'ta tek bir dosyada:
`/var/packages/dsm-mini/var/config.env`, izinler `600`.

Paketi kendi üzerine kurmak yeniden **sormaz**: sihirbaz kurulumda çalışır ve
bir güncelleme dosyaya bilerek dokunmaz — ayarların güncellemeyi atlatmasının
nedeni budur. Geriye üç yol kalıyor:

- **DSM ana menüsünden DSM mini'yi açın** — ayarlar ekranı bunlardan
  herhangi birini değiştirir, parola ve jeton orada yalnızca yazılır: boş
  bırakılan alan eskisini korur. Sonra paketi yeniden başlatın; bildirim
  ayarı hemen geçerli olur.

- **Dosyayı SSH ile düzenlemek** (Denetim Masası → Terminal ve SNMP → SSH'i
  açın), sonra paketi Paket Merkezi'nden yeniden başlatmak. Böylece geri kalan
  her şey durur:

  ```bash
  sudo sed -i "s|^TELEGRAM_BOT_TOKEN=.*|TELEGRAM_BOT_TOKEN='yeni-belirteciniz'|" /var/packages/dsm-mini/var/config.env
  sudo synopkg restart dsm-mini
  ```

- **Kaldırıp yeniden kurmak** — sihirbaz her şeyi baştan sorar. Ayarlarla
  birlikte yanındaki veritabanı da gider: sabitlenen klasörler, dil ve
  görevlerin son bilinen durumları.

## Güncelleme

Paket kaynağı eklendiyse Paket Merkezi güncellemeyi kendi gösterir. Kaynak yoksa
yeni `.spk` dosyasını
[sürümlerden](https://github.com/alexlnos/dsm-mini/releases) indirip üzerine
kurun.

Ayarlar ve veritabanı iki durumda da kalır: paketin `var` dizininde duruyorlar,
güncelleme oraya dokunmuyor.

## Veriler nerede duruyor

Her şey `/var/packages/dsm-mini/var/` içinde:

- `config.env` — sihirbazın sorduğu şeyler, izinler `600`;
- `dsm-mini.db` — bir SQLite veritabanı: sabitlenen klasörler, dil ve görevlerin
  son bilinen durumları; servis neyi bildirdiğini bunlardan anlıyor;
- `dsm-mini.log` — günlük.

Paketin güncellenmesi üçünü de korur. Paketin kaldırılması siler.

## Güvenlik

Servis internete açık ve NAS'taki dosyaları silebiliyor, bu yüzden:

- **Ayrı bir DSM kullanıcısı**, yönetici değil (3. adım).
- Bu hesapta **iki aşamalı doğrulama olmadan**: tek kullanımlık bir kod bir
  yapılandırma dosyasından çalışamaz.
- **İzin verilen kimlikler bir beyaz listedir.** Boş liste erişimi herkese
  kapatır, açmaz.
- `/api/` altındaki her istek iki kez denetlenir: bot belirtecinden türetilen bir
  anahtarla `initData` imzası ve kimliğin listeye karşı denetimi. İmza yalnızca
  birinin botu açtığını kanıtlar — açmayı ise herkes yapabilir.
- Dışarıya genel bir ifade gider, ayrıntılar günlüğe: imzada tam olarak neyin
  tutmadığını söyleyen bir mesaj, onu nasıl taklit edeceğinin ipucu olurdu.
- Bot belirteci ve DSM parolası yalnızca NAS'ın kendisindeki `config.env`
  dosyasında, `600` izinleriyle durur ve hiçbir zaman günlüğe düşmez: bir
  reddedişte neden yazılır, değer değil.
- Servis yalnızca `127.0.0.1` üzerinde dinler — yani NAS'ın kendisinden.
  Dışarıdan gelen her şey, TLS'i de sonlandıran DSM ters proxy'sinden geçer.

## Geliştirme

```bash
cd web && npm install && npm run build   # Mini App internal/web/dist içine düşer
cd .. && go build ./cmd/dsm-mini         # uygulaması gömülü tek bir ikili dosya
go test ./...
```

Ön yüz ayrı, otomatik yenilemeyle:

```bash
cd web && npm run dev    # localhost:8080 üzerindeki arka uçla konuşur
```

Arka ucu yerelde çalıştırmak, paket sihirbazının `config.env` içine yazdığı
değişkenlerin aynısını okur. Bunları bir dosyada tutmak elverişli:

```bash
cp .env.example .env     # doldurun
set -a; . ./.env; set +a
go run ./cmd/dsm-mini
```

Derlenmiş arayüz depoda duruyor (`internal/web/dist`) — onu `go:embed` alıyor. Ön
yüzü değiştirdikten sonra yeniden derleyip sonucu işleyin, yoksa denetimler
geçmez.

Tümleştirme testleri gerçek bir NAS'a karşı çalışır ve öntanımlı olarak atlanır:

```bash
DSM_URL=https://192.168.1.10:5001 DSM_USER=... DSM_PASSWORD=... \
  DSM_INSECURE_TLS=true go test -tags=integration ./...
```

NAS'ın durumunu değiştiren testler ayrı izin ister: `DSM_TEST_MUTATIONS=1`, dosya
işlemleri için ayrıca `DSM_TEST_FOLDER` — içinde geçici dosya oluşturulabilecek
klasör. Oluşturdukları her şeyi kendileri siler.

Yeni bir arayüz dili, her iki tarafta birer sözlük dosyası demek:
`web/src/i18n/<kod>.ts` ve `internal/i18n/<kod>.go`. Bir metni atlamak mümkün
değil: ön yüzde bunu tür, sunucuda bir test denetliyor.

Projede kod, açıklamalar ve günlük İngilizce; öteki diller yalnızca sözlüklerde
yaşıyor. Ayrıntılar [CLAUDE.md](CLAUDE.md) dosyasında.

## Synology API'sinin tuhaflıkları

Synology Web API'si yer yer belgelerinde yazandan başka türlü davranıyor: aynı
eylem üç ayrı biçimde yanıt veriyor, `limit = -1` File Station'ı deviriyor ve
dosya yüklerken `_sid` her yerdekinden başka türlü verilmek zorunda. Gerçek bir
NAS üzerinde öğrenilenlerin tamamı
[docs/synology-api.md](docs/synology-api.md) dosyasında toplandı.

## Lisans

[MIT](LICENSE)
