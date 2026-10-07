# مشارکت در fdev

[English](CONTRIBUTING.md) · **فارسی**

ممنون که می‌خواهید fdev را بهتر کنید. این راهنما همه‌ی مراحل را، از کلون
کردن کد تا انتشار نسخه‌ی جدید، قدم‌به‌قدم توضیح می‌دهد:

1. [چه چیزهایی لازم است](#need)
2. [گرفتن کد](#clone)
3. [ساختن و اجرا کردن](#run)
4. [اجرای تست‌ها و بررسی‌ها](#checks)
5. [برنچ‌ها چطور کار می‌کنند: اول beta، بعد main](#branches)
6. [ایجاد یک تغییر، قدم‌به‌قدم](#change)
7. [انتشار نسخه (برای نگهدارنده‌ها)](#release): **دستورهایی که باید زد**
8. [چه چیزی کجاست](#map)

---

<a name="need"></a>

## ۱. چه چیزهایی لازم است

| ابزار | برای چه | بررسی |
|---|---|---|
| [Go](https://go.dev/dl/) نسخه‌ی **1.27.1** یا جدیدتر | fdev با Go نوشته شده | `go version` |
| Git | | `git --version` |
| [Dart](https://dart.dev/get-dart) یا Flutter SDK | فقط اگر `dart/fdev_log` را تغییر می‌دهید | `dart --version` |
| [GitHub CLI](https://cli.github.com) (`gh`) | اختیاری: باز کردن PR و دنبال کردن ریلیز | `gh --version` |

برای امتحان کردن fdev **نیازی** به Flutter، گوشی یا امولاتور ندارید.
پروژه یک پروژه‌ی دمو با `flutter` و `adb` ساختگی دارد ([مرحله‌ی ۳](#run)).

<a name="clone"></a>

## ۲. گرفتن کد

اگر دسترسی push به این مخزن ندارید، اول در گیت‌هاب آن را **Fork** کنید
(دکمه‌ی Fork بالا سمت راست) و بعد fork خودتان را کلون کنید:

```sh
git clone https://github.com/<your-username>/fdev.git
cd fdev
git remote add upstream https://github.com/NaderMozaffari/fdev.git
git checkout beta
```

اگر دسترسی push دارید، مستقیم کلون کنید:

```sh
git clone https://github.com/NaderMozaffari/fdev.git
cd fdev
git checkout beta
```

> [!IMPORTANT]
> کار از برنچ **`beta`** شروع می‌شود، نه `main`. دلیلش در [بخش ۵](#branches) آمده است.

<a name="run"></a>

## ۳. ساختن و اجرا کردن

برنامه را در ریشه‌ی پروژه بسازید (فایل `fdev` در `.gitignore` است و
کامیت نمی‌شود):

```sh
go build -o fdev .
./fdev version        # یک نسخه‌ی توسعه نشان می‌دهد، مثل v0.0.0-2026...-07c1a2b
```

### امتحان بدون Flutter (پروژه‌ی دمو)

`scripts/demo/project` یک پروژه‌ی کوچک Flutter است و در `scripts/demo/bin`
نسخه‌های ساختگیِ `flutter`، `dart` و `adb` هستند که خروجی واقعی‌نما چاپ
می‌کنند. آن‌ها را اول PATH بگذارید و fdev را آنجا اجرا کنید:

```sh
go build -o fdev .
cd scripts/demo/project
PATH="$PWD/../bin:$PATH" ../../../fdev
```

یک تارگت انتخاب کنید تا لانچر و نمایشگر لاگ را کامل ببینید، با
درخواست‌های شبکه و رنگ‌ها و بقیه‌چیز، بدون اینکه چیز واقعی‌ای اجرا شود.
با `q` یا `ctrl+c` خارج شوید.

### امتحان روی یک پروژه‌ی واقعی Flutter

نسخه‌ای را که ساخته‌اید با مسیر کاملش اجرا کنید تا با `fdev` نصب‌شده‌ی
قبلی اشتباه نشود:

```sh
cd ~/path/to/your/flutter_app
~/path/to/fdev/fdev
```

### تنظیمات به‌دردبخور هنگام توسعه

| متغیر | کارش |
|---|---|
| `FDEV_REPO=owner/name` | مخزنی که `fdev update` ریلیزها را از آن می‌گیرد (برای تست آپدیت با fork خودتان) |
| `FDEV_CHANNEL=beta` | `fdev update` و اسکریپت‌های نصب نسخه‌های بتا را هم بگیرند |
| `FDEV_EDITOR` | ادیتوری که با کلیک روی لینک لاگ باز می‌شود |
| `FDEV_SHOW=1` | همراه `go test`، صفحه‌هایی را که تست‌های لانچر می‌کشند چاپ می‌کند |

fdev وضعیتش (اجراهای اخیر، تنظیمات) را در پوشه‌ی cache کاربر نگه می‌دارد
(`~/Library/Caches/fdev` در مک، `~/.cache/fdev` در لینوکس). برای اینکه
مثل کاربری که اولین بار fdev را باز می‌کند شروع کنید، این پوشه را پاک کنید.

برای ساختن با یک شماره‌نسخه‌ی واقعی (مثلاً برای تست صفحه‌ی About یا
`fdev update`):

```sh
go build -ldflags "-X main.version=v0.2.0-beta.1" -o fdev .
```

<a name="checks"></a>

## ۴. اجرای تست‌ها و بررسی‌ها

CI همین دستورها را روی هر push و هر pull request اجرا می‌کند
([.github/workflows/ci.yml](.github/workflows/ci.yml)). قبل از push خودتان
اجرایشان کنید:

```sh
go vet ./...
go test ./...
go build .
GOOS=windows go vet ./... && GOOS=windows go build -o /dev/null .   # هنوز برای ویندوز ساخته می‌شود؟
```

اگر پکیج Dart را تغییر داده‌اید:

```sh
cd dart/fdev_log
dart pub get
dart analyze
dart test
```

اگر چیزی را تغییر داده‌اید که در عکس‌های README دیده می‌شود، دوباره
ضبطشان کنید (به [agg](https://github.com/asciinema/agg/releases) نیاز دارد):

```sh
scripts/demo/record.sh              # همه
scripts/demo/record.sh hero themes  # فقط این‌ها
```

<a name="branches"></a>

## ۵. برنچ‌ها چطور کار می‌کنند: اول beta، بعد main

```
 your-branch ──PR──▶  beta  ──تست شد و مشکلی نبود──▶  main
                       │                               │
                 v0.3.0-beta.1                       v0.3.0
                 (pre-release)                  (نسخه‌ی پایدار)
```

| برنچ | چه چیزی رویش است | به دست چه کسی می‌رسد |
|---|---|---|
| `beta` | جدیدترین کارها که هنوز در حال تست‌اند | کاربرهای بتا (`fdev update --beta`) |
| `main` | فقط چیزهایی که روی بتا تست شده‌اند | همه؛ اسکریپت‌های نصب README از `main` خوانده می‌شوند |

قانون‌ها:

1. **هر تغییر در برنچی جدا از `beta` شروع می‌شود** و با یک pull request
به `beta` برمی‌گردد. هیچ‌کس مستقیم روی `main` کامیت نمی‌کند.
2. از `beta` یک **نسخه‌ی بتا** (`v0.3.0-beta.1`) منتشر می‌شود و مدتی
استفاده می‌شود.
3. اگر ایرادی پیدا شد، اصلاحش هم به `beta` می‌رود و بتای بعدی
(`v0.3.0-beta.2`) منتشر می‌شود.
4. وقتی بتا مشکل شناخته‌شده‌ای نداشت، **`beta` در `main` مرج می‌شود** و
**نسخه‌ی پایدار** (`v0.3.0`) از `main` منتشر می‌شود.

چون `main` فقط چیزی را می‌گیرد که از قبل در `beta` هست، این دو برنچ هیچ‌وقت
از هم جدا نمی‌شوند.

<a name="change"></a>

## ۶. ایجاد یک تغییر، قدم‌به‌قدم

**۱. از جدیدترین `beta` شروع کنید:**

```sh
git checkout beta
git pull                       # اگر fork دارید: git pull upstream beta
git checkout -b fix/wifi-timeout
```

اسم برنچ را بر اساس کاری که می‌کند بگذارید: `feature/…` برای قابلیت جدید،
`fix/…` برای رفع باگ، `docs/…` برای مستندات.

**۲. کد را بنویسید**، بعد بسازید، امتحانش کنید ([مرحله‌ی ۳](#run)) و
بررسی‌ها را اجرا کنید ([مرحله‌ی ۴](#checks)).

**۳. کامیت کنید.** هر قابلیت یا اصلاح را در کامیت جدای خودش بگذارید و
مطمئن شوید هر کامیت به‌تنهایی build می‌شود. عنوان کامیت را یک جمله‌ی کوتاه
(به انگلیسی) درباره‌ی چیزی که برای کاربر عوض می‌شود بنویسید، چون
**یادداشت‌های ریلیز از عنوان کامیت‌ها ساخته می‌شوند**:

```sh
git add -A
git commit -m "wifi: give up pairing after 30 seconds"
```

**۴. push کنید و یک pull request به `beta` باز کنید:**

```sh
git push -u origin fix/wifi-timeout
gh pr create --base beta --fill
```

یا PR را در سایت گیت‌هاب باز کنید. در هر دو حالت **برنچ مقصد (base) باید
`beta` باشد**، نه `main`. در توضیحات بنویسید چه چیزی عوض شده، چرا، و چطور
تستش کرده‌اید (برای چیزهای دیدنی، یک عکس یا GIF خیلی کمک می‌کند).

**۵. بازبینی.** CI باید سبز شود. اگر تغییری خواسته شد، کامیت‌های بیشتری به
همان برنچ push کنید؛ PR خودش به‌روز می‌شود.

**۶. تمام.** بعد از مرج، تغییر شما در بتای بعدی منتشر می‌شود.

### به‌روز نگه داشتن برنچ

اگر وقتی کار می‌کردید `beta` جلو رفته است:

```sh
git fetch origin               # اگر fork دارید: git fetch upstream
git rebase origin/beta         # اگر fork دارید: git rebase upstream/beta
git push --force-with-lease
```

### گزارش باگ و ایده

یک [issue](https://github.com/NaderMozaffari/fdev/issues) باز کنید و
سیستم‌عامل، خروجی `fdev version`، کاری که کردید، چیزی که انتظار داشتید و
چیزی که اتفاق افتاد را بنویسید. برای یک قابلیت بزرگ، اول issue باز کنید
تا قبل از نوشتن کد روی روشش توافق کنیم.

---

<a name="release"></a>

## ۷. انتشار نسخه (برای نگهدارنده‌ها)

> **خلاصه:** انتشار نسخه یعنی فقط **یک تگ git که به گیت‌هاب push می‌شود**.
> بقیه‌اش با [Release workflow](.github/workflows/release.yml) است: تست‌ها
> را اجرا می‌کند، fdev را برای مک، لینوکس و ویندوز (amd64 و arm64) می‌سازد
> و ریلیز گیت‌هاب را با فایل‌ها، checksumها و اسکریپت‌های نصب منتشر می‌کند.
> یادداشت‌های ریلیز، عنوان کامیت‌های بعد از تگ قبلی است. جزئیات بیشتر در
> [docs/RELEASING.md](docs/RELEASING.md).

### انتخاب شماره‌ی نسخه

نسخه‌ها به شکل `vMAJOR.MINOR.PATCH` هستند ([semver](https://semver.org/lang/fa/)):

| چه چیزی عوض شده | مثال | نسخه‌ی بعدی |
|---|---|---|
| فقط رفع باگ | `v0.3.0` ← | `v0.3.1` |
| قابلیت جدید | `v0.3.0` ← | `v0.4.0` |
| چیزی که روش استفاده را می‌شکند (بعد از 1.0) | `v1.4.2` ← | `v2.0.0` |

بتا یک `-beta.N` به نسخه‌ای که به آن می‌رسد اضافه می‌کند: `v0.4.0-beta.1`،
`v0.4.0-beta.2`، … و در آخر `v0.4.0`. برای دیدن آخرین تگ:

```sh
git fetch --tags
git describe --tags --abbrev=0
```

### الف. انتشار نسخه‌ی بتا (از `beta`)

```sh
git checkout beta
git pull
go test ./...                          # یک بررسی آخر

git tag v0.4.0-beta.1
git push origin v0.4.0-beta.1
```

ساخته شدنش را دنبال کنید و نتیجه را ببینید:

```sh
gh run watch                           # اجرای «Release» را انتخاب کنید
gh release view v0.4.0-beta.1
```

روی سیستم خودتان نصبش کنید و با آن کار کنید:

```sh
fdev update v0.4.0-beta.1              # یا: fdev update --beta
fdev version                           # fdev v0.4.0-beta.1 (beta)
```

ایرادی پیدا شد؟ با یک PR به `beta` درستش کنید (بخش ۶) و بتای بعدی را
منتشر کنید: `v0.4.0-beta.2`.

### ب. انتشار نسخه‌ی پایدار (مرج `beta` در `main`)

وقتی آخرین بتا مدتی بدون مشکل کار کرد:

```sh
# ۱. main را به beta برسانید
git checkout main
git pull
git merge --ff-only origin/beta
git push origin main

# ۲. نسخه‌ی پایدار را روی main تگ بزنید
git tag v0.4.0
git push origin v0.4.0
```

`--ff-only` فقط `main` را جلو می‌برد تا جایی که `beta` هست. اگر git قبول
نکرد، یعنی `main` کامیتی دارد که `beta` ندارد (کسی مستقیم روی `main`
کامیت کرده). به‌جایش `git merge origin/beta` بزنید، push کنید، و بعد `beta`
را هم هم‌سطح کنید (`git checkout beta && git merge main && git push`).

اگر ترجیح می‌دهید با یک pull request در گیت‌هاب انجامش دهید:

```sh
gh pr create --base main --head beta --title "Release v0.4.0" --fill
# در گیت‌هاب مرجش کنید، بعد:
git checkout main && git pull
git tag v0.4.0 && git push origin v0.4.0
```

> [!NOTE]
> تا `v1.0.0`، fdev فقط نسخه‌ی بتا (`v0.x.y-beta.n`) منتشر می‌کند، پس
> معمولاً همان مرحله‌ی **الف** را انجام می‌دهید. ولی وقتی یک بتا خودش را ثابت
> کرد، باز هم `beta` را در `main` مرج کنید (قسمت ۱ مرحله‌ی ب)، چون
> اسکریپت‌های نصب README از `main` خوانده می‌شوند.

### ج. اصلاح فوری برای نسخه‌ی پایدار (hotfix)

وقتی نسخه‌ی پایدار باگی دارد که نمی‌شود تا بتای بعدی صبر کرد:

```sh
git checkout main && git pull
git checkout -b fix/crash-on-start
# اصلاح، کامیت، و یک PR به main:  gh pr create --base main --fill
# بعد از مرج:
git checkout main && git pull
git tag v0.4.1 && git push origin v0.4.1

# و اصلاح را به beta هم برگردانید
git checkout beta && git pull
git merge main
git push origin beta
```

### اگر انتشار خراب شد

اگر Release workflow شکست خورد یا تگ را روی کامیت اشتباه زدید، تگ (و اگر
ریلیزی ساخته شده، ریلیز) را پاک کنید، مشکل را درست کنید و دوباره تگ بزنید:

```sh
gh release delete v0.4.0-beta.1 --yes --cleanup-tag   # اگر ریلیز منتشر شده
git tag -d v0.4.0-beta.1                              # تگ محلی
git push origin :refs/tags/v0.4.0-beta.1              # تگ روی گیت‌هاب، اگر هنوز هست
```

این کار را فقط برای ریلیزی بکنید که هنوز کسی نصبش نکرده. در غیر این صورت
نسخه‌ی بعدی را منتشر کنید.

برای عوض کردن کانال یک ریلیز بعد از انتشار:

```sh
gh release edit v0.3.2 --prerelease                     # بتا شود
gh release edit v1.0.0 --prerelease=false --latest      # پایدار شود
```

### خلاصه‌ی دستورها

| می‌خواهم… | دستورها |
|---|---|
| بتا منتشر کنم | `git checkout beta && git pull` ← `git tag vX.Y.Z-beta.N` ← `git push origin vX.Y.Z-beta.N` |
| نسخه‌ی پایدار منتشر کنم | `git checkout main && git pull && git merge --ff-only origin/beta && git push origin main` ← `git tag vX.Y.Z` ← `git push origin vX.Y.Z` |
| آخرین نسخه را ببینم | `git fetch --tags && git describe --tags --abbrev=0` |
| ساخته شدن ریلیز را دنبال کنم | `gh run watch` |
| همه‌ی ریلیزها را ببینم | `gh release list` |
| فایل‌ها را محلی بسازم، بدون انتشار | `scripts/release.sh vX.Y.Z-beta.N` (در `dist/` ساخته می‌شوند) |

---

<a name="map"></a>

## ۸. چه چیزی کجاست

| مسیر | محتوا |
|---|---|
| `main.go` | دستورها (`fdev`، `fdev logs`، `fdev wifi`، `fdev update`، `fdev version`) |
| `internal/launcher` | منو: تارگت‌ها، فلیورها، سؤال‌ها، نوار پیشرفت |
| `internal/logview` | نمایشگر لاگ: خواندن خروجی `flutter run`، فیلترها، نمای شبکه |
| `internal/config` | خواندن `fdev.yaml` و Makefile |
| `internal/theme`، `internal/sprite`، `internal/icon` | رنگ‌ها و تم‌ها، انیمیشن‌ها، آیکون اپ‌ها |
| `internal/devices`، `internal/wifi` | پیدا کردن دستگاه‌ها، دیباگ با وای‌فای |
| `internal/update`، `internal/version` | `fdev update` و منطق نسخه و کانال |
| `internal/state` | چیزهایی که fdev بین اجراها به خاطر می‌سپارد |
| `internal/editor`، `internal/vscode`، `internal/keys` | باز کردن لینک لاگ در ادیتور، یکپارچگی با VS Code، کلیدها |
| `dart/fdev_log` | پکیج Dart که اپ‌ها با آن لاگ می‌نویسند ([فرمت](docs/PROTOCOL.md)) |
| `scripts/install.sh`، `scripts/install.ps1` | اسکریپت‌های نصب |
| `scripts/release.sh` | ساخت و انتشار ریلیز |
| `scripts/demo` | پروژه‌ی دمو، ابزارهای ساختگی و ضبط تصاویر README |
| `fdev.example.yaml` | یک نمونه‌ی تنظیمات با توضیح |

با مشارکت، می‌پذیرید که کارتان تحت [مجوز MIT](LICENSE) پروژه منتشر شود.

