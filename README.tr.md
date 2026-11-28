<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/viva-lockup-dark.svg">
    <img src="website/src/assets/viva-lockup.svg" alt="Viva" width="240">
  </picture>
</h1>

<p align="center"><a href="README.md">English</a> | <a href="README.zh-CN.md">简体中文</a> | Türkçe</p>

<p align="center">
  <a href="https://docs.qq.com/doc/DVXZWdWFscGxjaWVP"><img src="https://img.shields.io/badge/Viva_交流群-扫码加入-07C160?style=for-the-badge&logo=wechat&logoColor=white" alt="Viva 交流群" /></a>
</p>

Viva, Go öncelikli ve GPU-yerel (GPU-native) bir masaüstü çatısıdır. Uygulamalar; WebView, tarayıcı motoru veya paketlenmiş bir çalışma zamanı olmadan doğrudan platformun kendi pencere ve GPU API'leri (Cocoa ve Metal, Win32 ve Direct3D 11, XCB ve Vulkan) üzerinden çizim yapar. Küçük ikili dosyalar, düşük boşta kaynak kullanımı ve yerel platform yeteneklerine doğrudan erişim hedeflenir. Web içeriğine ihtiyaç duyan bir uygulama, Viva'nın çiziminin yanında sistemin kendi web görünümünü (WebKitGTK, WKWebView, WebView2) barındırabilir: bu isteğe bağlıdır, ilk sayfa açıldığında yüklenir ve hiçbir şey paketlenmez.

Aşağıdaki durum listesi bugün çalışan özellikleri göstermektedir.

## Hızlı başlangıç

Viva, `github.com/vivaui/viva` adlı bir Go modülüdür. `viva` komutu uygulamaları derler, paketler ve canlı yeniden yükler; içindeki hiçbir şey dağıttığınız programa bağlanmaz.

```sh
brew install vivaui/tap/viva        # macOS, Linux
scoop bucket add vivaui https://github.com/vivaui/homebrew-tap && scoop install viva   # Windows
viva doctor                         # is this machine ready?
viva init hello
cd hello
viva dev .        # hot reload: save a file, the window updates and keeps its state
```

`v0.1.3` sürümünden itibaren her sürümde önceden derlenmiş arşivler (`SHA256SUMS` ile birlikte) bulunur; Go zaten kuruluysa `go install github.com/vivaui/viva/cmd/viva@latest` komutu da çalışır. Önceden Go kurulu olması gerekmez: `viva build`, `viva dev` ve `viva init`, Go yoksa resmi araç zincirini kullanıcı önbelleğinize bir kez indirir (go.dev'in SHA-256 değeriyle doğrulanır; `VIVA_GO_DOWNLOAD=0` bunu kapatır). `viva init` ile oluşturulan bir proje, `go.mod` dosyasında bir `tool` satırıyla komutu sabitler; böylece `go tool viva`, projenin derlendiği sürümü çalıştırır. Derleme ve paketleme rehberine bakın.

`viva init` çalıştırılabilir bir proje oluşturur: bir pencere, pencere açmadan onu süren bir test, `viva.toml` ve programlama modelini açıklayan bir `AGENTS.md` dosyası. `viva build` `build/` dizinine bir sürüm programı yazar, `viva package` ise platformun kendi paketini oluşturur.

Gereksinimler: Go 1.26 veya daha yenisi (eksik olduğunda viva bunu kendi indirir). macOS'ta Xcode komut satırı araçları gerekir, çünkü Viva Cocoa ve Metal'e cgo üzerinden erişir; Linux'ta sistem XCB, Pango ve Cairo kütüphaneleriyle X11 (isteğe bağlı GTK 3 kabuğu olan `-tags vivagtk`, yerel bir Wayland istemcisidir ve sistemin GTK 3'ünü gerektirir) ile GPU hızlandırması için bir Vulkan yükleyicisi ve sürücüsü gerekir (bunların olmadığı bir makinede çizim CPU üzerinde yapılır); Windows'ta ise Direct3D 11 ve DirectWrite ile Win32 gerekir. Viva sistem kütüphanelerini çalışma zamanında yükler ve hiçbirini paketlemez.

Güncel sürüm `v0.6.9`'dur; sabitlemek için açıkça belirtin (`@v0.6.9`). Rehberler, API referansı ve canlı önizlemeli bileşen kataloğu [`website/`](website/) dizininde yer alır; orada `npm run dev` ile çalıştırabilir veya [vivaui.dev](https://vivaui.dev/en/) adresinden okuyabilirsiniz ([Çince](https://vivaui.dev/zh/) olarak da mevcuttur).

Aşağıdaki her şey bu depoyu anlatır: ölçümlerini, kendi programlarını ve çatı ile değil, çatı üzerinde nasıl çalışılacağını.

## Kapsam

Viva; macOS, Windows ve Linux üzerinde yerel masaüstü uygulamalarını hedefler ve WebAssembly'yi desteklenen bir web dağıtım hedefi olarak ele alır; WebGL2 ile işlenir ve WebGL2 kullanılamadığında Canvas2D'ye geri döner. Her iki hedef de bileşenleri, girdiyi, metni, odağı ve erişilebilirlik semantiğini paylaşır; tarayıcının sahip olmadığı yetenekler hiçbir zaman sahte şekilde taklit edilmez, açık asenkron API'ler ve `Capability` aracılığıyla bildirilir. Web hedefi yerel derlemelere maliyet eklememelidir. iOS, iPadOS, Android, mobil paketleme ve dokunmatik öncelikli bir mobil çalışma zamanı açıkça kapsam dışındadır. Varsayımsal gelecekteki mobil uyumluluğu korumak adına masaüstü performansı ve yerel entegrasyon zayıflatılmayacaktır.

Her üç arka uç da WebView veya paketlenmiş bir işleme çalışma zamanı olmadan doğrudan işletim sisteminin pencere ve GPU API'lerini kullanır.

## Güncel Durum

- Yerel macOS penceresi ve olay döngüsü.
- Doğrudan Metal temizleme (clear) ve sunum (present).
- Kararlı düğüm kimliklerine sahip düz, nesilsel (generational) kalıcı sahne.
- Sıcak yolda (hot-path) sıfır bellek ayırması ile birleştirilmiş O(1) yerelleştirilmiş dikdörtgen güncellemeleri.
- Önceden derlenmiş Metal gölgelendirici ve örneklenmiş (instanced) dikdörtgen işleme.
- Olay güdümlü C'den Go'ya işaretçi yönlendirmesi ve 32 baytlık kısmi Metal yüklemeleri.
- Tıklama etkinleştirmesi ile boyama sırasına (paint-order) dayalı üzerine gelme (hover) ve basma etkileşim durumu; işaretçiyi tek bir bölge içinde hareket ettirmek işleyici (renderer) çalışması gerektirmez.
- Tab ile gezinme, Enter ve Boşluk ile etkinleştirme ve odak halkaları içeren klavye odağı; tek bir tuş olayı en fazla bir kez yeniden boyar ve erişilebilirlik güncellemelerini yalnızca odak gerçekten değiştiğinde yayınlar.
- Öğe başına kırpma (per-primitive clipping) ile olay güdümlü kaydırma: 1000 satırlık bir liste görünür penceresine sanallaştırılır, tek bir kaydırma hareketi tam olarak bir kez yeniden boyar ve bir kısmi GPU yüklemesi yapar; kırpılan içerik işleyiciye hiçbir zaman ulaşmaz.
- Üç yerel pencere modu: standart başlıklı, çerçevesiz (trafik ışıkları yok) ve özel başlık çubuğu (şeffaf başlık çubuğu, tam yükseklikte içerik, pencere sayfanın bildirdiği bölgelerden taşınır). Pencere kapatma uygulama tarafından yakalanıp durdurulabilir.
- `Window.SetFullscreen` üzerinden yerel tam ekran desteği; sistem geçişlerini bildiren ve çıkışta normal pencere yerleşimini geri yükleyen `Config.OnFullscreen` ile.
- `app.RunApplication` aracılığıyla açık çoklu pencere uygulamaları: her pencere bağımsız kalıcı boyama, odak, girdi, metin, geçersiz kılma ve kapatma durumuna sahiptir ve yerel döngü son pencere kapandıktan sonra sonlanır.
- Durum alanı simgeleri ve açılır paneller: `Application.AddStatusItem`, macOS menü çubuğuna, Windows bildirim alanına veya Linux StatusNotifier konağına bir simge yerleştirir; `StatusItemConfig.Menu` simgenin sistemin kendi menüsünü açmasını sağlar (NSMenu, bir Win32 açılır menüsü, `com.canonical.dbusmenu`); `WindowPanel`, yanında açılan ve odağı kaybettiğinde kapanan kenarlıksız açılır paneldir; `WindowNotification`, klavyeyi almayan ve odak başka yere gittiğinde açık kalan tıklanabilir bir açılır penceredir; `Window.Hide`/`Show` bir uygulamanın görünür bir pencere olmadan çalışmaya devam etmesini sağlar.
- `Application.SetDockBadge`, macOS'un uygulamanın Dock simgesinin köşesine çizdiği kırmızı rozete bir etiket yerleştirir; boş bir etiket ise rozeti kaldırır.
- `app.StoreSecret`/`LoadSecret`/`DeleteSecret`, küçük bir gizli veriyi sistemin kendi deposunda (Keychain, Credential Manager, Secret Service) saklar.
- `app.PreventSleep`, bir aktarım veya uzak oturum çalışırken makinenin boşta uykuya geçmesini engeller ve işi sistemin kendi güç tanılama araçlarında (`pmset`, `powercfg`, `systemd-inhibit`) adlandırır. Hiçbir zamanlayıcı veya goroutine tutmaz ve serbest bırakılması isteği sonlandırır.
- `Application.SetMenuBar`, macOS'ta sistemin uygulama menüsü ile Window menüsü arasına uygulamanın kendi File, Edit ve View menülerini tuş eşdeğerleri, etkinleştirme durumu, onay işaretleri ve alt menülerle yerleştirir. Bir öğenin eylemi, bir tuş işleyicisinin döndürdüğü `gfx.EventResult` değerinin aynısını döndürür ve çalışma zamanı bunun odağını, panosunu ve yeniden boyamasını etkin pencerede uygular; böylece menüden seçilen bir komut kısayoluyla aynı şekilde davranır. Diğer masaüstü hedefleri menülerini pencere içinde `components.Menubar` ile oluşturur.
- `Application.Post`, bir arka plan goroutine'inin tamamladığı işi pencere iş parçacığına aktarır: kabul edilen işlevler orada sırayla çalışır; ek bir goroutine, zamanlayıcı veya engelleyici bekleme eklenmez ve kilitlenmeye (deadlock) yol açabilecek "döngüyü çağır ve bekle" biçimi yoktur.
- `audio.Open`, sistemin varsayılan çıkışında (macOS'ta AudioUnit, Windows'ta WASAPI paylaşımlı mod, Linux'ta ALSA) aralıklı (interleaved) 16-bit PCM oynatır; arka plan çalışması yoktur ve bunu içe aktarmayan bir uygulamaya hiçbir arka uç bağlanmaz. Bkz. [docs/audio.md](docs/audio.md).
- `app/proxy.SystemProxy`, masaüstünün bir HTTP URL'si için seçtiği vekil sunucuyu (proxy) bildirir: macOS'ta CFNetwork, Windows'ta WinHTTP'nin otomatik vekil sunucusu (PAC, WPAD ve statik ayarlar) ve Linux'ta ortam değişkenleri. Platformun değerlendiremediği bir politika her zaman hata olarak döner, asla sessizce doğrudan bağlantıya geçmez. `SystemProxyContext`, bir isteğin macOS PAC indirmelerini ve değerlendirmesini iptal etmesine olanak tanır; her iki giriş noktası da PAC çalışmasını on saniye ile sınırlar.
- `gfx.MonoFamily`, kurulu herhangi bir yazı tipi ailesinde sabit genişlikli metin çizer ve `app.MonospaceFamilies` bunları listeler.
- `Config.OnFileDrop`, dosya yöneticisinden sürüklenen dosyaları işaretçinin altındaki isabet bilgisiyle birlikte alır.
- Yerel NSAccessibility öğelerine, Windows UI Automation sağlayıcılarına (Invoke, Toggle, Value, RangeValue, SelectionItem, ExpandCollapse ve Text kalıpları, olaylarla birlikte) ve Linux AT-SPI nesnelerine, tarayıcı derlemesinde ise ARIA'ya yansıtılan ayrı bir kalıcı semantik erişilebilirlik ağacı. Bir ekran okuyucunun basma, odaklanma ve değer ayarlama istekleri uygulamaya `platform.AccessibilityHandler` üzerinden ulaşır. Bkz. [docs/accessibility.md](docs/accessibility.md).
- Yeniden boyutlandırma başına tam olarak bir GPU gönderimi ve boyut değişmediğinde sıfır çalışma ile olay güdümlü pencere yeniden boyutlandırma boyaması.
- `Config.OnFrameChange`, `Config.OnMinimize` ve `Window.Minimized`, bir pencerenin nerede olduğunu ve pencere yöneticisinin onu simge durumuna küçültüp küçültmediğini bildirir.
- Değişmeyen düzenler için O(1) kontrollerine sahip kalıcı Row/Column/Stack/Flex düzeni.
- Boyama sırasına göre işaretçi yönlendirmesi, kararlı klavye odağı ve Tab ile gezinme.
- Gerçek Çince IME aday birleştirme, grafem duyarlı düzenleme ve UTF-16 eşlemesi ile yerel macOS `NSTextInputClient` entegrasyonu.
- macOS, Windows ve Linux üzerinde açık etkinleştir/değiştir/devre dışı bırak kare güncellemeleriyle odak güdümlü dinamik yerel metin düzenleyicileri. macOS ve Windows imleç zamanlayıcıları yalnızca düzenlenebilir bir alan odaktayken var olur. Linux, IBus protokolü (IBus ve Fcitx5'in IBus ön ucu) veya Fcitx 4'ün kendi protokolü üzerinden birleştirme yapar; XIM hâlâ açıktır.
- Ayrı bir kalıcı semantik erişilebilirlik ağacı.
- Doğrudan CoreText şekillendirme, sistem yazı tipi geri dönüşü (fallback), GPU ile birleştirilmiş metin dokuları ve sınırlı bir ölçüm önbelleği.
- Win32 olayları, D3D11 kalıcı işleme, gömülü DXBC gölgelendiricileri, DirectWrite ölçümü ve sınırlı GDI etiket dokularıyla deneysel Windows kabuğu; yerel derlemeler bir gölgelendirici derleyicisi yüklemez.
- Dinamik olarak yüklenen sistem XCB, Vulkan, Pango ve Cairo ile deneysel Linux/X11 kabuğu; kalıcı yuvarlatılmış dikdörtgenler ve üçgenler, katmanlı GPU ile birleştirilmiş sistem metni, işaretçi ve klavye girdisi, özel başlık çubuğu sürükleme, boşta yeniden çizimi olmayan engelleyici olay döngüsü ve donanım Vulkan aygıtı olmadığında pencereyi çizen bir CPU taramalı dönüştürücüsü (rasterizer).
- Boşta kare döngüsü olmayan olay güdümlü işleme.
- Tema belirteçleri (tokens) ve sabitlenmiş shadcn bileşen kataloğunun statik tipli, kaynağına sahip bir uyarlaması: formlar, katmanlar, menüler, takvimler, kaydırma tarifleri, geri bildirim yüzeyleri ve tarayıcı çalışma zamanı, yansıma (reflection) veya çalışma zamanı stil ayrıştırıcısı olmadan tipli dilimlerden (slices) çizilen alan, çizgi ve çubuk grafikleri içerir.
- Paylaşılan bir yerel/WASM bileşen tarayıcısı: kategoriye göre gruplanmış her bileşenin kataloğu ve her bileşen için tüm örneklerini yan yana canlı olarak (tıklama, üzerine gelme, klavye, sürükleme, tekerlek ve yerel metin düzenleme) çalıştıran, sürüklenebilir sayfa kaydırma çubuğuna ve sıcak yeniden yükleme durum korumasına sahip bir sayfa. Örnekler `internal/componentdemo` içindekilerdir ve `gfx.Canvas.Embed` ile sayfaya gömülürler; bu nedenle galeri hiçbir bileşenin kopyasını tutmaz.
- Yapılandırılmış tanılama: isteğe bağlı `diagnostics` paketi çatının bildirdiği olayları (yakalanan panic'ler, GPU aygıt kurtarma, web görünümü hataları, başlangıç ölçümleri) alır, `log/slog` ile çift yönlü uyum sağlar ve son olaylar ile hata bağlamının bir halkasını tutabilir; uygulama bir işleyici kurmadıkça hiçbir şey yazılmaz veya gönderilmez. Bkz. [docs/diagnostics.md](docs/diagnostics.md).
- İmzalı uygulama güncellemeleri: isteğe bağlı `update` paketi Ed25519 imzalı bir akışı (feed) okur, akışta listelenen arşivi indirir, uzunluğunu ve SHA-256 değerini doğrular ve platformun yükleyicisine teslim eder (macOS'ta `.app` değişimi, Windows'ta NSIS yükleyicisi, Linux'ta çalıştırılabilir dosya değişimi); `viva feed`, `viva package`'ın yazdığı arşivlerden bir akış imzalar. Bkz. [docs/updating.md](docs/updating.md).
- Pencere kapatıldığında veya uygulama sonlandırıldığında temiz olay döngüsü dönüşü.
- Tekrarlanabilir ikili dosya, başlangıç, CPU, bellek ve GPU gönderimi ölçümleri.

Windows arka ucu amd64 ve arm64 için çapraz derlenir ve Wine/DXVK altında platform testlerini ayrıca Gallery başlangıç, işaretçi ve klavye duman testlerini (smoke tests) geçer. Platform testleri, uyumluluk senaryoları ve girdi, bırakma ve aygıt kaybı yolları fiziksel bir Windows 11 makinesinde ve bir Debian KDE/X11 makinesinde (lavapipe) çalışır; gerçek donanım gerektiren senaryolar `docs/conformance.md` dosyasında kayıtlıdır. UI Automation gerçek bir Windows 11 makinesinde harici bir istemciden (bkz. docs/accessibility.md) ve AT-SPI ise KDE/X11 üzerinde libatspi'ye karşı kontrol edilir. Linux arka ucu; karışık Çince/Latin metin ve yerelleştirilmiş sahne güncellemeleri dahil olmak üzere Ubuntu arm64 Xvfb/llvmpipe M0 ve Debian amd64 X11/Lavapipe kalıcı sahne duman testlerini geçer. macOS çalışma zamanı testleri ve kıyaslamaları arm64 üzerinde çalışır.

Windows Mica, Mica Alt ve Acrylic, derleme komutları değiştirilmeden `app.Config.WindowBackdrop` aracılığıyla etkinleştirilir. Yapılandırılmamış pencereler sıradan işleyiciyi korur. Sürüm sınırları, geri dönüş davranışları ve fiilen doğrulanan platformlar için [Windows malzemeleri](docs/windows-materials.md) belgesine bakın.

Yerel etiket işleyicileri tek satırlı etiketler çizer. Tarayıcı hedefi aynı bileşenleri WebGL2 arka ucunda çalıştırır, geri dönüş olarak Canvas2D kullanır; yetenek matrisi ve barındırma gereksinimleri için [Web dağıtımı](docs/web-deployment.md) belgesine bakın.

## Depoyu çalıştırma

Bu komutlar, bu kopyadan çatının kendi programlarını derler ve çalıştırır.

```sh
go run ./cmd/showcase        # the native component showcase
go run ./cmd/component-preview -component button -example variants   # one example, natively
tools/build_web.sh           # viva build --target web, then serves http://127.0.0.1:8090
go run ./tools/docs -check   # the API and component data the website generates from
```

Tarayıcı derlemesi aynı Go bileşenlerini WebGL2 üzerinden boyar; WebGL2 kullanılamadığında veya bir bağlam kaybolduğunda Canvas2D WebAssembly arka ucuna geri döner. Bu yalnızca bir galeri değil, bir dağıtım hedefidir: dosya seçiciler, sürükle-bırak, pano, bildirimler, şifreli gizli veri depolama ve gömülü yazı tipleri tipli asenkron API'lere sahiptir. Görsel yönelimi ve ölçülen maliyetleri `docs/benchmarks/m3-gallery-style-darwin-arm64.md` dosyasında kayıtlıdır.

## Dokümantasyon web sitesi

[vivaui.dev](https://vivaui.dev/en/), İngilizce ve Çince dillerindeki halka açık sitedir: kod blokları `examples/guides/` altındaki dosyalardan oluşan ve `go test`'in başsız (headless) olarak derleyip sürdüğü rehberler, Go kaynak kodundan üretilen bir API referansı ve canlı Go/WASM önizlemesine sahip bileşen başına bir sayfa içerir. Astro + Svelte kaynak kodu [`website/`](website/) içindedir; yerel kullanıcı arayüzünün veya sürüm çalışma zamanının bir parçası değildir.

```sh
cd website
npm ci
npm run dev          # http://127.0.0.1:4321/zh/ and /en/
npm test && npm run build
```

Önizleme hattı, üretilen veriler ve `test:site` için [website/README.md](website/README.md) dosyasına bakın.

## Uygulama derleme ve dağıtma

`viva build`; `--target darwin`, `windows`, `linux` veya `web` için bir sürüm programı oluşturur (`-trimpath`, sembolleri temizlenmiş, sabit bir derleme kimliği; böylece aynı kaynaklar aynı baytları verir) ve boyutunu etrafındaki her şeyden ayrı olarak raporlar. `viva package` ise platformun kendi yükleyicisini ekler. Her ikisi de `go.mod` yanındaki isteğe bağlı `viva.toml` dosyasını okur:

```toml
[app]
name = "Viva Gallery"       # shown in the menu bar; names the .app
id = "dev.viva.gallery"     # bundle identifier
package = "./cmd/showcase"  # the main package to build

[mac]
min_system = "12.0"         # the macOS the program is compiled for; floor 12.0
```

```sh
go run ./cmd/viva build --target windows --arch amd64
go run ./cmd/viva package --zip
go run ./cmd/viva help package     # every command and flag
```

`viva.toml` katı bir şekilde çözülür: bilinmeyen bir tablo veya anahtar, yinelenen bir öğe ya da yanlış türde bir değer, satır numarasını belirten bir hataya yol açar. Bir macOS programı, derleyen makinenin macOS sürümü için değil `min_system` için derlenir ve `package`, programının gerektirdiğinden daha eski bir sistemi desteklediğini iddia eden bir paketi reddeder. İmzalama ve noter onayı (notarization) `--sign-identity` veya `APPLE_SIGNING_IDENTITY` artı notarytool kimlik bilgileri aracılığıyla isteğe bağlı olarak etkinleştirilir; `--no-sign` her ikisini de kapatır. `--arch amd64` her iki konaktan da diğer Darwin mimarisini derler: derleme cgo'yu ve Objective-C/Metal köprüsünü korur ve clang'ın diğer Mach-O dilimini üretmesine izin verir. [Derleme ve paketleme rehberi](https://vivaui.dev/en/docs/build-and-package/) her hedefi, yükleyici biçimini ve bayrağı kapsar.

Linux ve Windows derlemeleri varsayılan olarak cgo'yu devre dışı bırakır. Kendi C veya Rust kütüphanelerini bağlayan uygulamalar `viva build --cgo` veya `viva package --cgo` kullanabilir ve hedef C derleyicisini ile `CGO_CFLAGS`/`CGO_LDFLAGS` değişkenlerini sağlayabilir. Bu paketlerin destekledikleri sistem kütüphanelerine ve minimum işletim sistemine göre kontrol edilmesi gerekir; bu açık seçenek bir çapraz derleyici sağlamaz veya yerel bağımlılıkları taşınabilir hale getirmez. Tarayıcı derlemeleri `--cgo` seçeneğini reddeder; macOS ise sistem köprüsü için cgo'yu eskisi gibi etkin tutar.

Sürüm bağlantısı varsayılmaz, ölçülür. `go-toml/v2` yalnızca `viva` komutu tarafından ve `purego` yalnızca Linux arka ucu tarafından kullanılır; bu nedenle ikisi de bir macOS veya Windows programında yer almaz. Geliştirme yeniden yükleme protokolü, tek örnek soketleri (single-instance sockets), PNG simge kod çözme ve Go test çalışma zamanı sürüm programlarının dışında tutulur (yeniden yükleme, tek örnek ve PNG isteğe bağlı paketlerdir; test çalışma zamanı ise yoktur) ve sızıntı sayaçları pencere veya sürücü başına yalnızca iki atomik işlem maliyeti olduğu için kalır.

Herkese açık API aracılığıyla statik bir tuval (canvas) sunulabilir:

```go
theme := gfx.DarkTheme()
var canvas gfx.Canvas
components.Card(&canvas, theme, gfx.Rect{X: 24, Y: 24, Width: 320, Height: 180})
if err := app.RunCanvas(app.Config{Title: "My App"}, &canvas); err != nil {
    log.Fatal(err)
}
```

## Mühendislik Kuralları

Performans bütçeleri, bağımlılık politikası, mimari sınırlar ve tamamlanma tanımı, depoda yer alan kıyaslamalar ve testler tarafından zorunlu kılınır. Ölçümler kanıttır; "yerel hız", "boşta sıfır çalışma" ve "hafif" gibi terimler doğrulanmamış pazarlama iddiaları olarak kullanılmaz.

## Katkıda Bulunma

Hata raporları, platform bulguları ve dokümantasyon düzeltmeleri her zaman memnuniyetle karşılanır — bir değişikliğin geçmesi gereken kapılar ve çatı davranışı ile uygulama davranışı arasındaki çizginin nerede durduğu için [CONTRIBUTING.md](CONTRIBUTING.md) dosyasına bakın. Güvenlik sorunları hiçbir zaman herkese açık bir issue üzerinden değil, [SECURITY.md](SECURITY.md) üzerinden bildirilir.

## Katkıda Bulunanlar

<a href="https://github.com/vivaui/viva/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=vivaui/viva&max=300&columns=15" alt="Contributors" />
</a>

## Topluluk

<a href="https://linux.do"><img src="https://img.shields.io/badge/LINUX%20DO-Community-blue" alt="LINUX DO" /></a>

## Lisans

Viva, Apache License 2.0 altında lisanslanmıştır. Bkz. [LICENSE](LICENSE) ve [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
