# تنظیم fdev برای پروژه‌ی شما

[English](CUSTOMIZE.md) · **فارسی**

fdev در هر پروژه‌ی Flutter بدون هیچ تنظیمی کار می‌کند. این راهنما برای
وقتی است که می‌خواهید خودتان تعیین کنید منوی fdev چه چیزهایی نشان دهد:
تارگت‌های خودتان، توضیح هر کدام، و سؤال‌هایی که قبل از اجرا می‌پرسند.

- [fdev تارگت‌ها را از کجا می‌آورد](#where)
- [شروع با `fdev init`](#init)
- [ویرایش Makefile: بدون نیاز به برنامه‌نویسی](#edit)
- [Makefile: چه چیزهایی را fdev می‌خواند](#makefile)
- [نمونه‌ها](#examples)
- [fdev.yaml: کنترل کامل](#yaml)
- [ببینید fdev چه چیزی خوانده](#check)

<a name="where"></a>

## fdev تارگت‌ها را از کجا می‌آورد

fdev در پوشه‌ای که در آن اجرا می‌شود و بعد پوشه‌های بالاتر می‌گردد و اولین
مورد از این‌ها را که پیدا کند استفاده می‌کند:

| | کاری که fdev می‌کند |
|---|---|
| **`fdev.yaml`** | دقیقاً همان را استفاده می‌کند ([پایین‌تر](#yaml)) |
| **`Makefile`** | تارگت‌هایش را می‌خواند و گروه، فلیور، پلتفرم و سؤال‌های هر کدام را تشخیص می‌دهد |
| **هیچ‌کدام**، در پروژه‌ی Flutter | خودش برای هر فلیور `flutter run` می‌سازد، به‌علاوه‌ی بیلدهای معمول، `pub get`، `test` و … |

اگر در یک پروژه‌ی Flutter هیچ‌کدام از این دو فایل نباشد، fdev یک بار
پیشنهاد می‌دهد که برایتان Makefile بسازد. Makefile در هر نوع پروژه‌ای (Go،
Node، یک پکیج Dart و …) کار می‌کند: fdev تارگت‌هایش را نشان می‌دهد، همه زیر
**Tools** مگر اینکه `flutter run` یا `flutter build` اجرا کنند.

<a name="init"></a>

## شروع با `fdev init`

```sh
cd path/to/your_app
fdev init
```

این دستور یک `Makefile` مخصوص پروژه‌ی شما می‌نویسد:

- **با فلیور** (`productFlavors` اندروید، یا فایل‌های
  `lib/main_<flavor>.dart`): `dev`، `prod` و … برای اجرای هر فلیور،
  `ios-dev` اگر iOS هم آن فلیور را دارد، `web-dev` اگر پوشه‌ی `web/` هست،
  و `build-apk-dev`، `build-aab-dev`، `build-ipa-dev`، `build-web-dev` و …
- **بدون فلیور**: `run`، `web`، `build-apk`، `build-aab`، `build-ipa`،
  `build-web`.
- **ابزارها**، در هر دو حالت: `devices`، `get`، `test`، `analyze`، `clean`،
  و `build-runner` اگر پروژه از آن استفاده می‌کند.

`--flavor` را فقط جایی می‌گذارد که پروژه‌تان آن فلیور را دارد
(`productFlavors` اندروید، schemeهای iOS)، پس تارگت‌ها همان‌طور که هستند
کار می‌کنند. هر طور خواستید ویرایششان کنید: Makefile مال شماست و
`make dev` بدون fdev هم کار می‌کند.

| | |
|---|---|
| `fdev init --print` | به‌جای نوشتن، چاپش می‌کند تا با Makefile فعلی‌تان مقایسه کنید |
| `fdev init --force` | Makefile موجود را جایگزین می‌کند |

`make` به‌تنهایی همه‌ی دستورها را فهرست می‌کند.

<a name="edit"></a>

## ویرایش Makefile: بدون نیاز به برنامه‌نویسی

Makefile که `fdev init` می‌نویسد، از بالا به پایین چهار بخش دارد:

1. **یک توضیح** درباره‌ی شکل هر دستور، و لینک همین راهنما.
2. **تنظیمات**: گزینه‌هایی که به همه‌ی اجراها یا همه‌ی بیلدها اضافه می‌شوند.
3. **دستورها**، زیر *Run*، *Build* و *Tools*.
4. **بخش fdev**، که لازم نیست به آن دست بزنید.

**هر دستور** دو خط است: یک اسم، دونقطه، `##` و متنی که منو نشان می‌دهد؛
بعد، پس از یک **TAB** (کلید Tab، نه فاصله)، فرمانی که اجرا می‌کند:

```make
dev: ## Run the dev flavor
	flutter run --flavor dev -t lib/main_dev.dart $(RUN)
```

- **تغییر اسم یا متن**: خط اول را عوض کنید. دفعه‌ی بعد که fdev اجرا شود،
  منو تغییر را نشان می‌دهد.
- **اضافه کردن**: یک دستور (هر دو خطش) را کپی کنید و اسم، متن و فرمانش را
  عوض کنید. `$(RUN)` را آخر خط‌های `flutter run` نگه دارید: دستگاهی که fdev
  می‌پرسد، تنظیمات شما و لاگ‌های fdev را اضافه می‌کند.
- **حذف کردن**: هر دو خط را پاک کنید.

**تنظیمات** روی همه‌ی دستورها اعمال می‌شوند، پس فقط یک بار عوضشان می‌کنید:

```make
# Added to every run below.
RUN_OPTIONS = --dart-define=API_URL=https://test.example.com

# Added to every build below.
BUILD_OPTIONS = --obfuscate --split-debug-info=build/symbols
```

بیلدها نسخه‌ی release هستند، همان چیزی که `flutter build` به‌طور پیش‌فرض
می‌سازد.

> [!TIP]
> اگر دستوری در منوی fdev نیست یا make خطای `missing separator` داد، خط
> دوم آن دستور به‌جای TAB با فاصله شروع شده است. بیشتر ادیتورها وقتی خط را انتخاب کنید فرقش را نشان
> می‌دهند.

<a name="makefile"></a>

## Makefile: چه چیزهایی را fdev می‌خواند

هر rule که دستوری زیرش دارد یک تارگت است. fdev دستور را همان‌طور که
make باز می‌کند می‌خواند (متغیرها، `$(if …)`، `$(filter …)` و …)، پس
می‌فهمد هر تارگت چه کاری می‌کند.

**توضیحی** که منو نشان می‌دهد، متن `##` بعد از تارگت است:

```make
dev: ## Run the test app
```

یا کامنتی که درست بالای آن است:

```make
# Run the test app
dev:
```

یا خط همان تارگت در یک rule به اسم `help`، که بر دو مورد قبلی مقدم است:

```make
help:
	@echo "  dev        Run the test app"
```

**گروه** از چیزی که دستور اجرا می‌کند معلوم می‌شود:

| دستور شامل | گروه | |
|---|---|---|
| `flutter run` | **Run** | در نمایشگر لاگ باز می‌شود |
| `flutter build` | **Build** | |
| هر چیز دیگر | **Tools** | |

**فلیور** از `--flavor dev` یا `-t lib/main_dev.dart` معلوم می‌شود. منو
آیکون و مشخصات آن فلیور را کنار تارگت نشان می‌دهد.

**پلتفرم** از `-d chrome` / `-d macos` / …، از `build apk` / `build ipa` /
`build web` / …، از متغیرهایی با اسم `ANDROID_…` یا `IOS_…`، یا از اسم
تارگت (`ios-dev`، `web-prod`) معلوم می‌شود. یک `flutter run` با `--flavor`
و بدون دستگاه، اندروید حساب می‌شود. تارگت‌های iOS و macOS فقط روی مک
نشان داده می‌شوند.

**پرسیدن دستگاه.** یک `DEVICE ?=` خالی تعریف کنید و در دستور از آن
استفاده کنید. fdev دستگاه‌های وصل را نشان می‌دهد (اگر تارگت پلتفرم دارد،
فقط اندرویدی‌ها یا فقط iOSها) و تارگت را با `DEVICE=<id>` اجرا می‌کند:

```make
DEVICE ?=

dev: ## Run the test app
	flutter run --flavor dev $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

**پرسیدن هر چیز دیگر.** هر `NAME ?=` خالی دیگری که دستوری از آن استفاده
کند یک سؤال می‌شود. جواب‌هایش مقدارهایی هستند که Makefile آن را با
`$(filter value,$(NAME))` یا `ifeq ($(NAME),value)` مقایسه می‌کند، به‌علاوه‌ی
*Default* (خالی). اگر اسمش *store*، *market*، *shop* یا *edition* داشته
باشد، عنوانش *Store edition* می‌شود، و استورهای شناخته‌شده (`bazaar`،
`myket`، `huawei`، `amazon`، `samsung` و …) اسم و آیکون خودشان را می‌گیرند.

**لاگ‌های ساخت‌یافته‌ی fdev.** fdev موقع اجرای تارگت، `FDEV_DART_DEFINES`
را برابر `--dart-define=FDEV_LOGS=true` می‌گذارد. `$(FDEV_DART_DEFINES)` را
در خط‌های `flutter run` بگذارید تا اپی که از [`fdev_log`](../dart/fdev_log)
استفاده می‌کند لاگ‌هایش را برای fdev بنویسد. بدون fdev خالی است و چیزی را
عوض نمی‌کند.

fdev rule به اسم `help` و ruleهایی را که خود fdev را اجرا می‌کنند نشان
نمی‌دهد.

> [!NOTE]
> در ویندوز، تارگت‌های Makefile به `make` در PATH نیاز دارند
> (`winget install ezwinports.make`).

<a name="examples"></a>

## نمونه‌ها

### یک فلیور یا نسخه‌ی دیگر

```make
staging: ## The store app's id on the test backend
	flutter run --flavor staging -t lib/main_staging.dart $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

### یک سؤال: کدام بک‌اند

```make
# Which backend; empty for the one the flavor uses.
API ?=
API_DEFINE = $(if $(filter local,$(API)),--dart-define=API_URL=http://localhost:8080) \
             $(if $(filter staging,$(API)),--dart-define=API_URL=https://staging.example.com)

dev: ## Run the test app
	flutter run --flavor dev $(API_DEFINE) $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

قبل از اجرای `dev`، fdev می‌پرسد **Api**، با گزینه‌های *Default*، *Local* و
*Staging*، و بعد دستگاه را می‌پرسد. `make dev API=local` همین کار را بدون
fdev می‌کند.

### نسخه‌ی استور

```make
STORE ?=
FLAVOR = dev$(if $(filter myket,$(STORE)),Myket)$(if $(filter bazaar,$(STORE)),Bazaar)

dev: ## Run the test app
	flutter run --flavor $(FLAVOR) $(if $(DEVICE),-d $(DEVICE)) $(FDEV_DART_DEFINES)
```

fdev می‌پرسد **Store edition**: *Google Play*، *Myket* یا *Cafe Bazaar*، هر
کدام با آیکونش. برای این کار باید فلیورهای اندرویدی با همین اسم‌ها
(`devMyket`، `devBazaar`) داشته باشید.

### یک ابزار

```make
icons: ## Make the launcher icons
	dart run flutter_launcher_icons
```

زیر **Tools** نشان داده می‌شود و خروجی‌اش به‌صورت متن ساده نمایش داده می‌شود.

<a name="yaml"></a>

## fdev.yaml: کنترل کامل

وقتی Makefile نمی‌تواند چیزی را که می‌خواهید بگوید، یک `fdev.yaml` در
ریشه‌ی پروژه بگذارید: تارگت‌هایی که rule در make نیستند، اسم و آیکون
دلخواه برای جواب‌ها، مشخصات پنل فلیور در منو، یا ظاهر پیش‌فرض نمایشگر
لاگ. وقتی این فایل باشد، fdev فقط از آن استفاده می‌کند.

ساده‌ترین شروع، همان چیزی است که fdev از پروژه‌تان خوانده: در
`.fdev/fdev.yaml` است (git آن را نادیده می‌گیرد). آن را به ریشه‌ی پروژه کپی
و ویرایش کنید:

```sh
cp .fdev/fdev.yaml fdev.yaml
```

همه‌ی فیلدها در [fdev.example.yaml](../fdev.example.yaml) توضیح داده شده‌اند.
یک نمونه‌ی کوتاه:

```yaml
title: Acme Shop

groups:
  - name: Run
    targets:
      - name: dev
        desc: Test app
        run: make dev          # هر دستور شل
        flavor: dev            # آیکون و مشخصات فلیور در منو
        logs: true             # نمایشگر لاگ را باز می‌کند؛ FDEV_DART_DEFINES را می‌گذارد
        ask: [store, "device:android"]

asks:
  store:
    title: Store edition
    env: STORE                 # جواب به‌صورت $STORE می‌رسد
    options:
      - {label: Google Play, value: ""}
      - {label: Myket, value: myket, icon: store, color: "#00A0E3"}

flavors:
  dev:
    info:
      App: Acme Shop Dev
      Backend: test.example.com
```

سؤال‌های دستگاه آماده‌اند: `device`، `device:android` و `device:ios`
دستگاه‌های وصل را نشان می‌دهند و `DEVICE` را می‌گذارند.

<a name="check"></a>

## ببینید fdev چه چیزی خوانده

```sh
fdev help              # تارگت‌های این پروژه را به تفکیک گروه نشان می‌دهد
cat .fdev/fdev.yaml    # هر چیزی که fdev تشخیص داده: تارگت‌ها، سؤال‌ها، فلیورها
```

`.fdev/fdev.yaml` در هر بار اجرا نوشته می‌شود، مگر اینکه پروژه `fdev.yaml`
خودش را داشته باشد، و fdev هیچ‌وقت آن را نمی‌خواند. اگر تارگتی در گروه
اشتباه افتاده یا سؤالش را ندارد، این فایل نشان می‌دهد fdev چه فهمیده است.
