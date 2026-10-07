<div align="center">

[English](README.md) · **فارسی**

<img src="docs/media/banner.svg" alt="fdev: لانچر و نمایشگر لاگ برای Flutter، در ترمینال" width="100%">

<br>

**فلیور، استور و دستگاه را از یک منو انتخاب کنید. خروجی `flutter run` را به‌صورت لاگ‌هایی تمیز،
رنگی و قابل کلیک بخوانید؛ هر درخواست شبکه در یک خط.**

<br>

![status: beta](https://img.shields.io/badge/status-beta-F5A524?style=flat-square)
[![release](https://img.shields.io/github/v/release/NaderMozaffari/fdev?include_prereleases&sort=semver&style=flat-square&label=release&color=F780E2)](https://github.com/NaderMozaffari/fdev/releases)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?style=flat-square&logo=go&logoColor=white)
![Flutter](https://img.shields.io/badge/for-Flutter-02569B?style=flat-square&logo=flutter&logoColor=white)
![macOS · Linux · Windows](https://img.shields.io/badge/macOS%20·%20Linux%20·%20Windows-555?style=flat-square)
![Bubble Tea](https://img.shields.io/badge/built%20with-Bubble%20Tea-F780E2?style=flat-square)
![MIT](https://img.shields.io/badge/license-MIT-6B50FF?style=flat-square)

[نصب](#install) •
[استفاده](#use) •
[لانچر](#launcher) •
[نمایشگر لاگ](#viewer) •
[کلیدها](#keys) •
[تم‌ها](#look) •
[وای‌فای](#wifi) •
[`fdev_log`](#fdev-log)

<br>

<img src="docs/media/hero.gif" alt="fdev: از منو تا اجرای اپ، لاگ‌ها، باز کردن یک درخواست شبکه و فیلتر" width="100%">

</div>

> [!NOTE]
> نسخه‌ی فعلی fdev **بتا** است: هر روز استفاده می‌شود، اما ممکن است
> ایراد داشته باشد و تا `v1.0.0` تغییر کند. شماره‌ی نسخه هم این را نشان
> می‌دهد (`v0.2.0-beta.1`) و [نسخه‌های پایدار](#versions)
> جدا از آن منتشر می‌شوند. اگر مشکلی دیدید، [یک issue باز کنید](https://github.com/NaderMozaffari/fdev/issues).

<br>

ساخته‌شده با [Bubble Tea](https://github.com/charmbracelet/bubbletea)،
[Bubbles](https://github.com/charmbracelet/bubbles)،
[Huh](https://github.com/charmbracelet/huh) و
[Lip Gloss](https://github.com/charmbracelet/lipgloss) از Charm، با رنگ‌های
Charm؛ خط‌های لاگ شبیه [Log](https://github.com/charmbracelet/log) هستند.

## در یک نگاه

<table>
<tr>
<td width="50%" valign="top">

**انتخاب کنید، تایپ نکنید.** اول اجراهای اخیر، بعد Run، Build، Tools و
لاگ‌های ذخیره‌شده؛ بعد پلتفرم و فلیور، هر کدام با آیکونش.

<img src="docs/media/launcher.png" alt="منو: Recent، Run، Build، Tools">

</td>
<td width="50%" valign="top">

**بدانید چه چیزی را اجرا می‌کنید.** تارگتِ زیر نشانگر با آیکون اپ:
مشخصات فلیور، دستوری که اجرا می‌کند، سؤال‌هایی که می‌پرسد و آخرین اجرا.

<img src="docs/media/targets.png" alt="یک تارگت با آیکون اپ و مشخصات فلیورش">

</td>
</tr>
<tr>
<td valign="top">

**سؤال‌هایی با جواب‌های آماده.** نسخه‌ی استور، دستگاه؛ جواب دفعه‌ی قبل
از پیش انتخاب شده است.

<img src="docs/media/ask.png" alt="انتخاب نسخه‌ی استور">

</td>
<td valign="top">

**بیلدی که پیشرفتش را می‌بینید.** نوار پیشرفتی که با مدت همان مرحله
در دفعه‌ی قبل سنجیده می‌شود.

<img src="docs/media/progress.png" alt="نوار پیشرفت Gradle">

</td>
</tr>
<tr>
<td valign="top">

**هر درخواست شبکه در یک خط**، که با یک کلیک هدرها و بدنه‌اش باز می‌شود.
درخواستی که هنوز منتظر جواب است می‌چرخد.

<img src="docs/media/network.png" alt="یک درخواست شبکه که هدرها و بدنه‌اش باز شده">

</td>
<td valign="top">

**فیلتر مثل جست‌وجو.** `tag:Auth`، `status:4`، `method:post`، `-word`
و موارد دیگر.

<img src="docs/media/filter.png" alt="لاگ‌ها فیلترشده به درخواست‌های POST">

</td>
</tr>
<tr>
<td valign="top">

**انتخاب لاگ‌ها** برای کپی، کپی به‌صورت cURL، فیلتر، سنجاق، نشانک،
دنبال کردن، پنهان کردن یا ذخیره.

<img src="docs/media/select.png" alt="دو لاگ انتخاب‌شده و کارهایی که می‌شود با آن‌ها کرد">

</td>
<td valign="top">

**توکن‌ها دم دست.** آخرین access token، شناسه‌ی کاربر و مقادیر دیگر،
برای کپی در هر لحظه.

<img src="docs/media/values.png" alt="پنجره‌ی مقادیر">

</td>
</tr>
</table>

<div align="center">

**ده تم، سه چیدمان**: Charm، Dracula، Tokyo Night، Catppuccin Latte، Gruvbox، Nord و …

<img src="docs/media/themes.gif" alt="نمایشگر لاگ با تم‌های Charm، Dracula، Tokyo Night، Catppuccin Latte، Gruvbox و Nord، به‌صورت خطی، ستونی و جدولی" width="100%">

</div>

<a name="install"></a>

## نصب

فقط یک خط، بدون نیاز به نصب چیز دیگری: اسکریپت نسخه‌ی مناسب سیستم‌عامل و
پردازنده‌تان را انتخاب می‌کند، آن را با checksum‌های ریلیز بررسی می‌کند و
`fdev` را به PATH اضافه می‌کند. برای به‌روزرسانی دوباره اجرایش کنید یا
`fdev update` را بزنید.

**مک و لینوکس** (در ترمینال):

```sh
curl -fsSL https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.sh | sh
```

<sub>اگر curl ندارید: `wget -qO- https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.sh | sh`</sub>

**ویندوز، PowerShell** (اول خط فرمان `PS` نوشته شده):

```powershell
irm https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.ps1 | iex
```

**ویندوز، Command Prompt** (همان `cmd`، خط فرمان فقط `C:\>` است):

```bat
powershell -ExecutionPolicy Bypass -c "irm https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.ps1 | iex"
```

> [!TIP]
> خطای `'sh' is not recognized` یا `'irm' is not recognized` یعنی دستور
> برای shell دیگری است: در `cmd` از خط Command Prompt بالا استفاده کنید.

اسکریپت‌ها در ویندوز در `%LOCALAPPDATA%\Programs\fdev` و در بقیه‌ی
سیستم‌ها در `~/.fdev/bin` نصب می‌کنند (با `FDEV_INSTALL_DIR` قابل تغییر
است) و آن را به PATH اضافه می‌کنند: در ویندوز به PATH کاربر، و در بقیه
به فایل شروع هر shell که دارید (zsh، bash، fish، sh). یک ترمینال تازه باز
کنید و `fdev` را بزنید.

<details>
<summary><b>نسخه‌ی بتا یا یک نسخه‌ی مشخص</b></summary>

تا وقتی نسخه‌ی پایداری منتشر نشده، اسکریپت‌ها جدیدترین بتا را نصب
می‌کنند و این را اعلام می‌کنند. برای اینکه از آن به بعد هم بتاها را بگیرید:

```sh
curl -fsSL https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.sh | sh -s -- --beta
```

```powershell
$env:FDEV_CHANNEL = 'beta'; irm https://raw.githubusercontent.com/NaderMozaffari/fdev/main/scripts/install.ps1 | iex
```

برای یک نسخه‌ی مشخص: `sh -s -- v0.2.0-beta.1`، یا در PowerShell
`$env:FDEV_VERSION = 'v0.2.0-beta.1'`. در `cmd`، خط PowerShell را داخل
گیومه بعد از `powershell -c` بگذارید.

</details>

<details>
<summary><b>نصب دستی</b>، بدون اسکریپت</summary>

از صفحه‌ی [ریلیزها](https://github.com/NaderMozaffari/fdev/releases)
فایل مناسب کامپیوترتان را دانلود کنید:

| | |
|---|---|
| مک، Apple silicon (M1 به بعد) | `fdev_darwin_arm64.tar.gz` |
| مک، Intel | `fdev_darwin_amd64.tar.gz` |
| لینوکس | `fdev_linux_amd64.tar.gz`، یا روی ARM `fdev_linux_arm64.tar.gz` |
| ویندوز | `fdev_windows_amd64.zip`، یا روی ARM `fdev_windows_arm64.zip` |

آن را با `checksums.txt` بررسی کنید (`shasum -a 256 <file>`، یا در ویندوز
`Get-FileHash <file>`)، از حالت فشرده خارج کنید و `fdev` (یا `fdev.exe`)
را در پوشه‌ای که در PATH است بگذارید. در مک، فایلی که با مرورگر دانلود
شده قبل از اجرا به `xattr -d com.apple.quarantine fdev` نیاز دارد؛ در
ویندوز ممکن است SmartScreen بار اول بپرسد (**More info → Run anyway**).

اگر به `raw.githubusercontent.com` دسترسی ندارید، اسکریپت‌های نصب در هر
ریلیز هم هستند: `install.sh` یا `install.ps1` را از آنجا دانلود و اجرا
کنید (`sh install.sh`، یا `powershell -ExecutionPolicy Bypass -File install.ps1`).

</details>

<details>
<summary><b>با Go</b> (نسخه‌ی 1.27.1 یا جدیدتر، وگرنه Go خودش دانلودش می‌کند)</summary>

```sh
go install github.com/NaderMozaffari/fdev@latest
```

برای یک نسخه‌ی مشخص `@v0.2.0-beta.1`. Go آن را در `$(go env GOPATH)/bin` می‌گذارد.

</details>

<details>
<summary><b>تفاوت‌ها در ویندوز</b></summary>

نمایشگر لاگ دستورها را بدون pty اجرا می‌کند، از طریق `sh` در Git for
Windows (که Flutter به هر حال لازمش دارد)، یا اگر `sh` نباشد با `cmd`. به
همین دلیل بعضی ابزارها بدون رنگ چاپ می‌کنند و **Stop** (`ctrl+c`) دستور
را تمام می‌کند. تارگت‌های Makefile به `make` در PATH نیاز دارند
(`winget install ezwinports.make`).

</details>

<a name="versions"></a>

### نسخه‌ها: بتا و پایدار

نسخه‌های fdev [نسخه‌گذاری معنایی](https://semver.org/lang/fa/) دارند.
نسخه‌ای که بخش پیش‌انتشار دارد، مثل `v0.2.0-beta.1`، **بتا** است: در
گیت‌هاب به‌عنوان pre-release منتشر می‌شود، و `fdev version` (`fdev
v0.2.0-beta.1 (beta)`) و صفحه‌ی About هم آن را بتا نشان می‌دهند. نسخه‌ی
بدون آن، مثل `v1.0.0`، **پایدار** است. fdev تا `v1.0.0` بتا است.

```sh
fdev update             # جدیدترین نسخه‌ی کانال خودش: بتا به جدیدترین بتا
fdev update --stable    # فقط نسخه‌های پایدار
fdev update --beta      # بتاها هم
fdev update v0.1.3      # همان نسخه، برای برگشتن به آن

fdev channel            # این fdev روی کدام کانال است
fdev channel stable     # تغییر کانال: جدیدترین نسخه‌ی پایدار، حتی از یک بتای جدیدتر
fdev channel beta       # تغییر کانال: جدیدترین بتا
```

بعد از تغییر کانال، `fdev update` روی همان کانال می‌ماند، چون از fdev
نصب‌شده پیروی می‌کند. با `FDEV_CHANNEL=beta` (یا `stable`) کانال را برای
همیشه تعیین کنید، هر نسخه‌ای که نصب باشد.

<details>
<summary><b>حذف</b></summary>

پوشه‌ی `~/.fdev` را پاک کنید (و خط‌های `# fdev` را که اسکریپت به
`~/.zshrc`، `~/.bashrc`، `~/.bash_profile`، `~/.profile` یا
`~/.config/fish/conf.d/fdev.fish` اضافه کرده)؛ در ویندوز پوشه‌ی
`%LOCALAPPDATA%\Programs\fdev` را پاک کنید و آن را از PATH کاربر بردارید.
تنظیمات fdev در پوشه‌ی cache کاربر، در پوشه‌ای به اسم `fdev` است.

</details>

برای مشارکت در fdev (اجرا از روی کد، باز کردن pull request یا انتشار نسخه‌ی جدید)، [CONTRIBUTING.fa.md](CONTRIBUTING.fa.md) را ببینید.

<a name="use"></a>

## استفاده

دستور `fdev` را هر جای پروژه اجرا کنید. ریشه‌ی پروژه را با گشتن به
بالا پیدا می‌کند: اول `fdev.yaml`، بعد `Makefile` یا `pubspec.yaml`.

```sh
fdev                     # منو
fdev dev                 # اجرای مستقیم تارگت dev
fdev logs -- flutter run -d emulator-5554 --dart-define=FDEV_LOGS=true
fdev wifi                # دیباگ گوشی اندروید از طریق وای‌فای (یا Tools → wifi-debug)
fdev init                # یک Makefile برای این پروژه می‌نویسد تا تارگت‌ها را تغییر دهید
fdev help                # همه‌ی دستورها و تارگت‌های این پروژه
fdev help update         # گزینه‌های یک دستور
```

اگر دستور، گزینه یا تارگتی را اشتباه تایپ کنید، fdev نزدیک‌ترین‌ها را
پیشنهاد می‌دهد (`fdev versoin` ← *Did you mean? fdev version*).

> [!TIP]
> هیچ تنظیمی لازم نیست و چیزی به git اضافه نمی‌شود: fdev هر بار منو را از
> خود پروژه درمی‌آورد، پس هیچ‌وقت قدیمی نمی‌شود.

- **تارگت‌ها** همان قانون‌های Makefile هستند، با توضیحی از خط `make help`
  (`@echo "  dev    Run the test app"`)، یک `## text` بعد از قانون یا یک خط
  `# text` بالای آن. fdev هر دستور را مثل make باز می‌کند و از آن
  این‌ها را می‌خواند: گروه (`flutter run` می‌شود Run، `flutter build`
  می‌شود Build، بقیه Tools)، فلیور (`--flavor`، `-t lib/main_<flavor>.dart`)،
  پلتفرم (`-d chrome`، `build apk`، متغیرهایی با نام `ANDROID_…`/`IOS_…`،
  `ios/` در دستور یک ابزار) و سؤال‌ها: `DEVICE ?=` دستگاه را می‌پرسد، و هر
  `NAME ?=` خالی دیگری که دستور استفاده کند، یکی از مقدارهایی را می‌پرسد
  که Makefile با آن مقایسه‌اش می‌کند (`$(filter myket,$(STORE))`)، با نام
  و آیکون استورهای رایج (Google Play، کافه‌بازار، مایکت، AppGallery و …).
- **فلیورها** از `android/app/build.gradle(.kts)` می‌آیند (`productFlavors`؛
  `devMyket` نسخه‌ی مایکت `dev` است)، برچسب‌هایشان از `strings.xml`،
  از `ios/Runner.xcodeproj` (bundle id و نام نمایشی)، و از یک enum در Dart
  داخل `lib/` که برای هر فلیور یک مقدار دارد، مثل
  `dev(apiHost: 'test.example.com')`: رشته‌هایش (یک host یا URL می‌شود
  Backend) و توضیح مستندش.
- بدون Makefile: برای هر فلیور `flutter run`، بیلدها، `pub get`، `test` و …

آنچه پیدا کرده در `.fdev/fdev.yaml` نوشته می‌شود تا ببینید (fdev هیچ‌وقت
آن را نمی‌خواند)؛ داخل `.fdev/` یک `.gitignore` با `*` هست، پس git هیچ‌کدام
را نمی‌بیند.

برای کنترل کامل، یک `fdev.yaml` در ریشه‌ی پروژه بگذارید (از
`.fdev/fdev.yaml` شروع کنید یا [fdev.example.yaml](fdev.example.yaml) را
ببینید): از آن به بعد fdev همان را استفاده می‌کند.

برای تغییر تارگت‌ها یا اضافه کردن تارگت‌های خودتان، در پروژه‌ای که Makefile
ندارد `fdev init` را بزنید: یک Makefile مخصوص همان پروژه می‌نویسد (اجرا و
بیلد هر فلیور، یا خود اپ اگر فلیور ندارد)، و fdev یک بار هم موقع اجرا
پیشنهادش را می‌دهد. اینکه fdev چطور Makefile را می‌خواند، با نمونه:
[docs/CUSTOMIZE.fa.md](docs/CUSTOMIZE.fa.md).

<a name="launcher"></a>

## لانچر

در شروع، fdev قدم‌به‌قدم می‌پرسد: **بخش** (اول Recent، بعد Run، Build،
Tools و لاگ‌های ذخیره‌شده)، بعد **پلتفرم** و **فلیور** از میان
تارگت‌هایش. هر جواب آیکون خودش را دارد (آیکون‌های پیکسلی fdev، و برای
فلیور آیکون خود اپ، که با نیم‌بلوک کشیده می‌شود تا در هر ترمینال رنگی
دیده شود) و کارتی که محتوایش را نشان می‌دهد.

- جواب‌های دفعه‌ی قبل از پیش انتخاب شده‌اند؛ `→` یا `enter` جلو می‌رود،
  `←` یا `esc` برمی‌گردد، `1` تا `9` انتخاب می‌کند، و دکمه‌های پایین صفحه
  هم همین کار را می‌کنند؛ قدم‌هایی که فقط یک جواب دارند رد می‌شوند.
- بعد یک تارگت انتخاب کنید (از `fdev.yaml`، Makefile، یا دستورهای ساده‌ی
  `flutter`)، به سؤال‌هایش (نسخه‌ی استور، دستگاه و …) به همین شکل جواب
  بدهید، با آیکون (فیلدهای `icon` و `color` هر گزینه در `fdev.yaml`) و
  با `←` برای برگشت به سؤال قبل، و اجرایش کنید.
- یک پنل، تارگتِ زیر نشانگر را با آیکون بزرگ اپ نشان می‌دهد: مشخصات
  فلیور، دستوری که اجرا می‌کند و سؤال‌هایی که می‌پرسد، و آخرین اجرا.
  تارگتی که پلتفرم یا فلیور ندارد زیر همه نشان داده می‌شود.
- با رفتن ماوس روی گزینه انتخاب می‌شود و با کلیک اجرا. تنظیمات خود fdev
  پشت **⚙ settings** است (یا `s`؛ `t` برای تم)، و **⤢ full** (یا `z`)
  کلیدها و دکمه‌ها را پنهان می‌کند.

اسم تب ترمینال، چیزی است که در آن اجرا می‌شود، با یک دایره به رنگ فلیور:
`🟡 fdev v1.4.0 • Acme Shop • dev • SM-S918B • bazaar`، و بعد از
تمام شدن `✓` یا `✘`. VS Code (و Cursor) تب‌ها را به اسم پروسه نام‌گذاری
می‌کنند، مگر اینکه `terminal.integrated.tabs.title` شامل `${sequence}`
باشد؛ برای همین fdev اولین بار که در ترمینال آن‌ها اجرا شود این را به
تنظیمات کاربر اضافه می‌کند؛ مقداری که خودتان گذاشته باشید دست‌نخورده
می‌ماند. برنامه‌ها راهی برای تغییر آیکون یا رنگ تب در آن‌ها ندارند، پس
دایره جای هر دو را می‌گیرد.

<a name="viewer"></a>

## نمایشگر لاگ

<img src="docs/media/viewer.png" alt="نمایشگر لاگ: لاگ‌ها با زمان، سطح و تگ، درخواست‌های شبکه، یک Map در Dart به‌صورت JSON و یک خطا با stack trace" width="100%">

همان `flutter run`، بدون شلوغی `I/flutter (12345):`. هر لاگ یک خط است با
زمان، سطح و تگش، و `↗` خطی از کد را که آن را لاگ کرده باز می‌کند.

- **درخواست‌های شبکه** یک خط هستند (`RESP GET /v1/me status=200 took=142ms`)
  که به هدرها و بدنه باز می‌شوند. پارامترهای query جدا و هر کدام در یک خط
  نشان داده می‌شوند (`? type = UPDATE_INFORMATION`)، نه چسبیده به مسیر.
- درخواستی که هنوز منتظر جواب است **می‌چرخد** و زمان سپری‌شده را نشان
  می‌دهد؛ وقتی جواب برسد، خود درخواست هم نتیجه را می‌گوید (`→ 200 142ms`)،
  و هدر بالای صفحه تعداد درخواست‌های در جریان را می‌شمارد.
- **دسته‌ها** را در حین اجرای اپ روشن و خاموش کنید (`1` تا `9`) و با متن
  **فیلتر** کنید.
- زیر لاگ‌ها، کلیدهای دستوری که در حال اجراست **دکمه** هستند، هر کدام با
  کلیدش کنارش: کلیدهای flutter (`r` Reload، `R` Restart، `v` DevTools،
  `i` Inspector، `s` Screenshot، `q` Quit، و بقیه پشت **more**)، یا برای
  ابزارهای دیگر، کلیدهایی که خودشان چاپ می‌کنند (`press r + enter to restart`،
  `› Press a │ open Android`، یا فهرستی بعد از خط «key commands»)؛ و بعد
  **Stop** (`ctrl+c`) در حین اجرا و **Back** (`enter`) بعد از آن.
- وقتی flutter روی یک مرحله کار می‌کند (`Running Gradle task 'assembleDevDebug'...`،
  `Installing ...apk...`)، یک **نوار پیشرفت** نشان می‌دهد چقدر جلو رفته، در
  مقایسه با مدتی که همان مرحله دفعه‌ی قبل طول کشید؛ بار اول، که چیزی برای
  مقایسه نیست، یک نوار بارگذاری است که به‌جای پر شدن، رفت‌وبرگشت می‌کند.
  ترمینال‌هایی که پشتیبانی کنند پیشرفت را روی تب هم نشان می‌دهند.
- **بیلدها و دستورهای دیگر** هم در همین نمایشگر اجرا می‌شوند، پس خروجی‌شان
  قابل اسکرول و فیلتر است و خطاها قرمز.

<a name="keys"></a>

### کلیدهای نمایشگر لاگ

| کلید | |
|---|---|
| `1` تا `9` | info، success، warning، error، debug، شبکه، جزئیات شبکه، لاگ‌های native اندروید، خروجی خام |
| `,` `.` `;` | نمایش یا پنهان کردن زمان، برچسب سطح و تگ |
| `/` | فیلتر: کلمه‌هایی که لاگ باید همه را داشته باشد؛ `tag:Billing`، `level:error`، `url:/v1/me`، `status:4`، `method:post`، `name:getProfile`، `is:bookmarked`، `is:pinned`، `is:tracked`، `is:hidden`، `is:waiting`، `"two words"`، و `-` قبل از کلمه برای حذف آن‌ها؛ `esc` پاکش می‌کند |
| `?` | همه‌ی کلیدها |
| `k`، **k values** | توکن‌ها و مقادیر دیگر، جدیدترینِ هر کدام، برای دیدن و کپی در هر لحظه: پایین‌تر را ببینید |
| `l` | چیدمان، برچسب‌ها، فاصله، خط بین گروه‌ها، زمان، ذخیره، میانبرهای خاموش |
| `ctrl+s` | ذخیره‌ی کل لاگ در `.fdev/logs` |
| **⤓ save** | ذخیره‌ی بخشی از لاگ‌ها با موضوع و تگ: انتخاب‌شده‌ها، نمایش‌داده‌شده‌ها، نشانک‌دارها یا همه، به‌صورت متن (که دوباره در fdev باز می‌شود)، Markdown یا JSON |
| `ctrl+l`، **⌫ clear** | پاک کردن صفحه، نه لاگ: آنچه قبلاً بوده ذخیره می‌شود (اگر ذخیره خاموش بود روشن می‌شود) و فایل جلسه بعد از خط `──── cleared ────` ادامه پیدا می‌کند؛ کلید `c` در flutter هم همین کار را می‌کند |
| `↑` `↓` `pgup` `pgdn` `home` `end`، چرخ ماوس | اسکرول نرم (چرخ ماوس با ادامه‌ی چرخیدن سریع‌تر می‌شود)؛ `end` دوباره خروجی جدید را دنبال می‌کند |
| کلیک روی `↗` | باز کردن کدی که آن خط را لاگ کرده |
| کلیک روی خطی با `▸` | باز کردن هدرها و بدنه، یا کل stack trace: باز می‌شوند و لحظه‌ای علامت می‌خورند |
| `x`، **⊞ open all** | باز کردن همه‌ی درخواست‌های شبکه (و آن‌هایی که بعداً می‌آیند)؛ دوباره برای بستن |
| `e`، کلیک روی **… more lines · ⤢ open** | لاگ بلند (بدنه‌ی بزرگ یک پاسخ، یک print طولانی) فقط ردیف‌های اولش را نشان می‌دهد؛ این کلید کلش را در یک پنجره باز می‌کند: `↑` `↓` `pgup` `pgdn` `home` `end` اسکرول، `c` کپی بدنه، `esc` بستن |
| `tab`، کلیک روی یک لاگ | انتخابش؛ `↑` `↓` جابه‌جا می‌کنند، `shift+↑` `shift+↓` (یا shift+کلیک) یک بازه را انتخاب می‌کنند. نوار پایین آن وقت کارهایی است که می‌شود با آن‌ها کرد: `c` کپی، `j` کپی بدنه، `u` کپی URL، `C` کپی درخواست به‌صورت cURL، `f` فقط لاگ‌های مشابه، `F` پنهان کردن لاگ‌های مشابه، `p` سنجاق به بالا، `b` نشانک، `t` دنبال کردن، `s` ذخیره، `o` باز کردن در پنجره، `g` باز کردن کدش؛ `esc` تمام |
| `[` `]` | نشانک قبلی یا بعدی |
| `m` | روشن/خاموش کردن ماوس؛ خاموش (یا با نگه داشتن ⌥/shift) برای انتخاب متن |
| `z`، **⤢ full** | تمام‌صفحه: فقط عنوان و لاگ‌ها؛ `z`، `esc` یا **⤡ exit** در گوشه نوارها را برمی‌گرداند |
| `h` (با لاگ‌های انتخاب‌شده) | پنهان کردنشان: جای هر کدام یک ردیف خالی با **◌ show** می‌ماند که برش می‌گرداند، تا لاگ‌های اطراف جابه‌جا نشوند |
| هر کلید دیگر | به flutter می‌رود: `r` reload، `R` restart، `q` quit و … |

عددها فقط وقتی دکمه‌ی روشن/خاموش می‌شوند که flutter کلیدهایش را فهرست
کرده باشد، تا اگر flutter چیزی بپرسد («choose a device: 1, 2, ...») عدد
به خودش برسد. وضعیت روشن/خاموش‌ها برای هر پروژه به خاطر سپرده می‌شود.

لاگ **سنجاق‌شده** هر قدر هم اسکرول کنید بالای لاگ‌ها می‌ماند. لاگ
**دنبال‌شده** هم همان‌جا می‌ماند، به‌صورت جدیدترینِ لاگ‌های شبیه خودش
(همان درخواست، یا همان پیام با هر عددی)، با تعداد دفعاتی که آمده:
`◉ 12:04:51 RESP GET ← getProfile /v1/me 200 142ms ×7`. با کلیک روی ردیف
خود لاگ نشان داده می‌شود؛ `✕` آن سنجاق یا دنبال کردن را برمی‌دارد.

### مقادیر دم دست

با `k` مقادیری که از لاگ‌ها نگه داشته شده‌اند باز می‌شوند: جدیدترینِ هر
اسم، برای دیدن کامل و کپی (`enter`، یا `1` تا `9`)، و `g` لاگی را که در
آن آمده نشان می‌دهد. این مقادیر فقط تا پایان جلسه نگه داشته می‌شوند و از
این‌ها می‌آیند:

- توکن‌ها در هدرها و بدنه‌ی درخواست‌ها: `authorization`، `access_token`،
  `refresh_token`، `id_token`، `token`، `jwt`، `session_id`، `api_key` و …
  (وقتی اپ آن‌ها را نپوشانده باشد؛ `fdev_log` به‌طور پیش‌فرض می‌پوشاند)؛
- آنچه اپ خودش می‌دهد: `FdevLog.value('accessToken', token)`، که پوشانده
  نمی‌شود؛
- کلیدهایی که در `fdev.yaml` نام می‌برید: `logs: {values: [userId, deviceId]}`.

<a name="look"></a>

### ظاهر و ذخیره لاگ‌ها

<table>
<tr>
<td width="50%" valign="top">
<img src="docs/media/settings.png" alt="تنظیمات: تم، که با رفتن نشانگر رویش امتحان می‌شود">
</td>
<td width="50%" valign="top">
<img src="docs/media/table.png" alt="چیدمان جدولی با برچسب‌های رنگی، در تم Tokyo Night">
</td>
</tr>
</table>

با `l` در نمایشگر (یا **⚙ settings** / `s` در منو، `t` برای تم) تنظیمات
باز می‌شود. اول صفحه، برای همه‌ی پروژه‌ها:

- **تم**: *Auto* (رنگ‌های Charm، تیره یا روشن مثل ترمینال)، یا Charm تیره
  یا روشن، Dracula، Nord، Tokyo Night، Catppuccin (Mocha و Latte)، Gruvbox
  یا Solarized Light، که تا وقتی fdev اجراست پس‌زمینه‌ی ترمینال را هم
  رنگ می‌کنند (اگر ترمینال اجازه بدهد). هر تم با رفتن نشانگر رویش امتحان
  می‌شود؛ `esc` تم قبلی را برمی‌گرداند.
- **اندازه‌ی منوها و آیتم‌هایشان**: *Auto* بزرگ‌ترین اندازه‌ای را که در
  پنجره جا شود می‌گیرد؛ *Small* هر آیتم یک خط، بدون خط خالی، و نوارهای
  جمع‌وجور در نمایشگر (بدون عنوان بخش‌ها، یک ردیف دکمه‌ی دستور)، برای
  صفحه‌های کوچک؛ *Medium* دو خط؛ *Large* آیکون‌های بزرگ.

حالت تمام‌صفحه (`z`، یا **⤢ full** بالای منوها و در نوار نمایشگر) کلیدها،
راهنماها و دکمه‌ها را پنهان می‌کند: در نمایشگر فقط عنوان بالای لاگ‌ها
می‌ماند. در هر شروع خاموش است.

بعد لاگ‌ها، برای همین پروژه:

- **چیدمان**: *lines* (فشرده، مثل charmbracelet/log)، *columns* (یک ستون
  برای تگ؛ درخواست‌های شبکه در ستون‌های `METHOD STATUS URL TOOK SIZE`) یا
  *table* (همان ستون‌ها با حاشیه و سرستون).
- **برچسب‌ها**: متن رنگی، یا نشان با پس‌زمینه‌ی رنگی.
- **فاصله** بین لاگ‌ها، و **زمان** با یا بدون میلی‌ثانیه.
- **خط بین لاگ‌های مرتبط**: یک خط چین `╌╌ +2.4s ╌╌` بعد از مکث یک ثانیه
  یا بیشتر، تا هر دسته (آنچه یک لمس لاگ می‌کند) یک گروه باشد و پاسخ هر
  قدر هم طول بکشد کنار درخواستش بماند؛ یا یک خط هر جا تگ عوض شود
  (`╌╌ AuthRepo ╌╌`)؛ یا هیچ.
- **متن فارسی و عربی**: بیشتر ترمینال‌ها (ترمینال VS Code، iTerm2، Ghostty،
  kitty، Windows Terminal) آن را جدا جدا و از چپ به راست می‌کشند؛ fdev
  حروف را به هم می‌چسباند و متن را راست‌به‌چپ می‌چیند (عددهای داخلش
  چپ‌به‌راست می‌مانند). *Auto* این کار را به ترمینال‌هایی که خودشان
  انجامش می‌دهند می‌سپارد (Terminal.app، GNOME Terminal و بقیه‌ی
  ترمینال‌های VTE، Konsole). فقط نمایش عوض می‌شود: لاگ‌های ذخیره‌شده و
  کپی‌ها همان متن اصلی را دارند. فیلتر حروف هم‌صدای فارسی و عربی را یکی
  می‌گیرد (`ي` `ی`، `ك` `ک`)، با یا بدون نیم‌فاصله، و ارقام فارسی را
  مثل 0 تا 9.
- **ذخیره‌ی همه‌ی جلسه‌ها.**
- **میانبرهای خاموش**: `ctrl+c`، `ctrl+l`، `ctrl+s`، `ctrl+d`، `ctrl+z`
  که اینجا انتخاب شوند با زدنشان کاری نمی‌کنند (نمایشگر همین را می‌گوید)؛
  دکمه‌های Stop و clear همچنان کار می‌کنند.

با `ctrl+s` هر لحظه کل لاگ ذخیره می‌شود: همه‌ی خط‌ها، حتی پنهان‌ها، با
stack trace‌ها، هدرها و بدنه‌ها. لاگ‌ها در `.fdev/logs/` داخل پروژه
می‌روند (`<time>_<target>.log`، و هنگام ضبط `.raw.log` با خروجی اصلی، هر
خط بعد از زمان رسیدنش)؛ fdev در آن پوشه یک `.gitignore` با `*` می‌گذارد،
پس git چیزی آنجا نمی‌بیند و هیچ فایل ردیابی‌شده‌ای تغییر نمی‌کند. سی
جلسه‌ی آخر و همه‌ی ستاره‌دارها نگه داشته می‌شوند.

با **⤓ save** (یا `s` وقتی لاگ‌هایی انتخاب شده) بخشی از لاگ‌ها با
**موضوع** و **تگ** ذخیره می‌شوند: لاگ‌های انتخاب‌شده، نمایش‌داده‌شده‌ها (با
فیلترهای فعلی)، نشانک‌دارها یا همه؛ به‌صورت متن، که دوباره در fdev باز
می‌شود و زیر موضوع و تگ‌هایش در Saved logs می‌آید (ستاره‌دار، تا هیچ‌وقت
پاک نشود)، به‌صورت Markdown برای یک issue، یا JSON.

بخش **Saved logs** در منو آن‌ها را فهرست می‌کند، اول ستاره‌دارها: `→`
هر کدام را همان‌طور که بود در نمایشگر باز می‌کند (دسته‌ها، فیلتر، باز
کردن و `↗` کار می‌کنند)، `space` ستاره می‌زند، دو بار `x` حذف می‌کند، و
`o` فایل را در ادیتور باز می‌کند.

تنظیمات شما برای هر پروژه در cache کاربر نگه داشته می‌شود؛ بخش `logs:`
در `fdev.yaml` پیش‌فرض‌ها را برای همه تعیین می‌کند.

### باز کردن کد

برای باز کردن کد، fdev خودش ادیتور را اجرا می‌کند و فایل و شماره‌ی خط را به‌عنوان آرگومان
می‌دهد (بدون shell، بدون URL handler)، و فقط برای فایل‌هایی که داخل پروژه
وجود دارند. در ترمینال VS Code یا Cursor از همان‌ها استفاده می‌کند، در
ترمینال Android Studio یا IntelliJ از آن‌ها، و بعد `code`، و بعد
`$VISUAL` / `$EDITOR`. برای انتخاب خودتان:

```sh
export FDEV_EDITOR='zed {file}:{line}:{col}'      # یا editor: در fdev.yaml
```

<a name="wifi"></a>

## دیباگ از طریق وای‌فای

دستور `fdev wifi` (یا **wifi-debug** زیر Tools، در پروژه‌ای که اپ اندروید
دارد) گوشی را از طریق وای‌فای به adb وصل می‌کند. اول گوشی‌هایی را که قبلاً
pair شده‌اند و خودشان را اعلام می‌کنند وصل می‌کند. در غیر این صورت یک
QR code در ترمینال نشان می‌دهد برای **Wireless debugging → Pair device
with QR code** در گوشی. اگر **Pair device with pairing code** را انتخاب
کنید، fdev گوشی را پیدا می‌کند و کد ۶ رقمی را می‌پرسد؛ اگر شبکه گوشی را
پنهان می‌کند، IP و پورتی را که پنجره نشان می‌دهد تایپ کنید. `fdev wifi pair`
مستقیم سراغ pair کردن می‌رود.

وقتی به گوشی دسترسی نباشد، به‌جای `protocol fault` در adb، دلیلش را
می‌گوید:

- **فیلترشکن (VPN)**: ترافیک گوشی به‌جای وای‌فای وارد تونل (`utun4`)
  می‌شود. fdev اپ‌های VPN در حال اجرا را نام می‌برد و می‌گوید در هر کدام
  چطور شبکه‌ی محلی را مستثنا کنید، یا خط `sudo route` را چاپ می‌کند که
  فقط گوشی را از VPN رد می‌کند؛
- **شبکه‌ی دیگر**: گوشی و کامپیوتر به وای‌فای‌های متفاوتی وصل‌اند؛
- **پورت بسته یا بی‌جواب**: پنجره بسته شده، صفحه قفل شده، یا وای‌فای
  دستگاه‌ها را از هم جدا نگه می‌دارد (شبکه‌های مهمان، AP isolation).

<a name="fdev-log"></a>

## لاگ ساختاریافته در اپ

نمایشگر با هر اپی کار می‌کند: پیشوندهای logcat را حذف می‌کند، بر اساس
سطح رنگ می‌کند، و لاگ‌های یک‌خطی مثل `INFO [Tag] message` را می‌فهمد.
برای تجربه‌ی کامل (تگ‌ها، JSON چندخطی، خط‌های شبکه، لینک‌های `↗`) لاگ‌ها را
با [فرمت رکورد](docs/PROTOCOL.md) fdev چاپ کنید. پکیج
[`fdev_log`](dart/fdev_log) همین کار را می‌کند:

```yaml
dependencies:
  fdev_log:
    git: {url: https://github.com/NaderMozaffari/fdev, path: dart/fdev_log}
```

```dart
import 'package:fdev_log/fdev_log.dart';

void main() {
  FdevLog.appPackage = 'my_app';        // its frames are what ↗ links to
  FdevLog.skip = ['core/logger/'];      // your own logging helpers, if any
  FdevLog.info('connected', tag: 'Billing');
  runApp(const App());
}
```

مپ‌ها و لیست‌ها به‌صورت JSON مرتب نشان داده می‌شوند، چه در یک رکورد چه در
یک `print` ساده: `FdevLog.info(user)` (یک `Map`، یا مدلی با `toJson()`)، و
حتی آنچه Dart برای یک map چاپ می‌کند،
`print('state: $map')` → `state: {id: 7, tags: [a, b]}`، که fdev دوباره به
JSON تبدیلش می‌کند.

با `FdevLog.value(name, value)` یک مقدار در نمایشگر دم دست می‌ماند (`k`).

رکوردها فقط وقتی چاپ می‌شوند که اپ با `--dart-define=FDEV_LOGS=true`
بیلد شده باشد؛ fdev این را برای تارگت‌های `logs: true` در
`FDEV_DART_DEFINES` می‌گذارد، پس در Makefile به flutter بدهیدش:

```make
RUN_ARGS ?= $(FDEV_DART_DEFINES)

dev:
	flutter run --flavor dev $(RUN_ARGS)
```

در غیر این صورت لاگ‌ها متن یک‌خطی ساده‌اند، و در بیلد release هیچ.

<details>
<summary><b>یک interceptor برای Dio</b></summary>

```dart
class FdevDioLogger extends Interceptor {
  static const _start = 'fdev_start';

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    options.extra[_start] = DateTime.now();
    FdevLog.http(FdevHttp(
        phase: FdevHttpPhase.request, method: options.method, url: '${options.uri}',
        headers: options.headers, body: options.data, caller: options.extra['fdev_caller'] as StackTrace?));
    handler.next(options);
  }

  @override
  void onResponse(Response response, ResponseInterceptorHandler handler) {
    final o = response.requestOptions;
    FdevLog.http(FdevHttp(
        phase: FdevHttpPhase.response, method: o.method, url: '${response.realUri}',
        status: response.statusCode, duration: _took(o), body: response.data,
        caller: o.extra['fdev_caller'] as StackTrace?));
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final o = err.requestOptions;
    FdevLog.http(FdevHttp(
        phase: FdevHttpPhase.error, method: o.method, url: '${o.uri}',
        status: err.response?.statusCode, duration: _took(o), body: err.response?.data,
        error: err.response == null ? err.type.name : null,
        caller: o.extra['fdev_caller'] as StackTrace?));
    handler.next(err);
  }

  Duration? _took(RequestOptions o) =>
      o.extra[_start] is DateTime ? DateTime.now().difference(o.extra[_start] as DateTime) : null;
}
```

برای جفت کردن درخواست‌های هم‌زمان به یک URL، `id: o.hashCode` را بدهید
(که در درخواست و پاسخش یکی است).

برای اینکه خط‌های شبکه به کدی لینک شوند که درخواست را فرستاده، نه به
خود Dio، جایی که Dio را صدا می‌زنید (در API client، قبل از اولین `await`)
`Options(extra: {'fdev_caller': StackTrace.current})` را بدهید.

</details>

## حریم خصوصی و امنیت

- برنامه‌ی fdev چیزی جز دستورهای Makefile / `fdev.yaml` شما اجرا نمی‌کند (پروژه را
  می‌خواند، برای پیدا کردن تارگت‌ها اجرایش نمی‌کند)، و جواب‌ها به‌صورت
  متغیر محیطی به دستورها می‌رسند، هرگز وسط دستور گذاشته نمی‌شوند.
- هر چیزی که از دستگاه می‌آید به‌صورت متن نشان داده می‌شود: escape
  sequence‌ها، کاراکترهای کنترلی و bidi override‌ها حذف می‌شوند، تا یک خط
  لاگ نتواند ترمینال شما را تغییر دهد (عنوان، کلیپ‌بورد، لینک‌ها).
- کلید `↗` فقط فایل‌های داخل پروژه را باز می‌کند.
- پکیج `fdev_log` توکن‌ها، رمزها، کوکی‌ها و کلیدهای مشابه را در هدرها،
  بدنه‌ها و URL‌ها می‌پوشاند (`FdevLog.redactKeys`، `FdevLog.redact`)، و
  فقط در بیلدهای debug لاگ می‌کند.
- پنجره‌ی مقادیر توکن‌ها را فقط در حافظه و تا پایان جلسه نگه می‌دارد؛
  جایی نوشته نمی‌شوند جز همان‌جا که لاگ از قبل بود.
- برنامه‌ی fdev اجراهای اخیر، دسته‌ها و تم شما را در پوشه‌ی cache کاربر نگه
  می‌دارد، نه در پروژه. هیچ چیزی از کامپیوتر شما خارج نمی‌شود.

## تصاویر این صفحه

گیف‌ها و اسکرین‌شات‌ها ضبط شده‌اند، نه طراحی: `scripts/demo/record.sh`
برنامه‌ی fdev را در یک پروژه‌ی نمونه (`scripts/demo/project`) اجرا می‌کند
که `flutter` و `adb` آن نسخه‌های ساختگی‌اند و لاگ‌های یک اپ فرضی را چاپ
می‌کنند، کلیدهای `scripts/demo/scenes` را تایپ می‌کند و نتیجه را با
[agg](https://github.com/asciinema/agg) به تصویر تبدیل می‌کند. بعد از هر
تغییر در صفحه‌ها دوباره اجرایش کنید.

## مجوز

MIT
