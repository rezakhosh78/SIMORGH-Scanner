![Ping Logo](ping_logo.png)

# RKh-CFS v0.2.0 | @pingplas_channel ⚡

اسکنر **Clean-IP کلادفلر** برای کانفیگ‌های **VLESS**.

RKh-CFS برای تست IP، CIDR، رنج‌های دستی و لیست‌های آماده ISP ساخته شده و نتیجه‌ها را به‌صورت رتبه‌بندی‌شده خروجی می‌دهد.  
در خط نسخه v0.2.0، پروژه حالا شامل **Web UI مدرن**، نسخه **Windows Single EXE WebUI** و نسخه بهینه‌شده **Android APK** است.

---

## 📦 نسخه‌ها

| نسخه | فایل / پروژه اصلی | محیط اجرا |
|---|---|---|
| Windows Web UI Single EXE | `RKh-CFS-win-v0.2.0.exe` | ویندوز + Python 3 نصب‌شده |
| Windows Web UI package | `web_ui.py` / `run_webui.bat` | ویندوز + Python 3 + فایل‌های Xray |
| Windows Python scanner | `RKh-CFS-v0.2.0.py` | ویندوز + Python 3 + `xray.exe` |
| Android APK | `RKh-CFS-Android-v0.2.0.apk` | APK اندروید با backend بومی Go |
| Android / Termux legacy | پکیج Termux | اندروید + Termux + باینری Android/Linux `xray` |


---

## 📥 فایل‌های نهایی ریلیز

اسم فایل‌های نهایی ریلیز:

```text
Windows Web UI: RKh-CFS-win-v0.2.0.exe
Android APK:    RKh-CFS-Android-v0.2.0.apk
Termux ZIP:     RKh-CFS-Termux-v0.2.0.zip
```

---

## ✨ تغییرات مهم v0.2.0

- اضافه شدن **Web UI** 
- پشتیبانی از زبان فارسی و انگلیسی؛ زبان پیش‌فرض انگلیسی است.
- اضافه شدن نسخه Android APK.
- نسخه **Windows Single EXE WebUI** اضافه شد.
- پنل Live Ranking با ستون‌های:
  - IP
  - Stage
  - Latency
  - Avg Latency
  - Pass
  - Speed
- امکان sort کردن Live Ranking با کلیک روی عنوان ستون‌ها.
- اولویت پیش‌فرض مرتب‌سازی Live Ranking بر اساس latency است.
- Scan Progress حالا مراحل scan، re-check و speed-test را دنبال می‌کند.
- انتخاب ISP list با checkbox، چندانتخابی، Check all و Clear.
- دکمه‌های Stop و Continue در Web UI.
- دانلود فایل‌های خروجی از داخل مرورگر.
- خروجی Best Configs و Final Ranked Configs.
- بهینه‌سازی Android Go برای موبایل:
- جدول Live Ranking فشرده‌تر و مناسب گوشی

---

## 🚀 قابلیت‌ها

- ورود کانفیگ VLESS
- ورود دستی تارگت:
  - تک IP
  - CIDR
  - Range
  - Paste چندخطی
- لیست آماده ISPها
- دسته‌بندی ایران و International
- انتخاب چند ISP برای اسکن
- تنظیم concurrency / worker
- Re-check اختیاری برای latency
- Speed-test اختیاری
- نمایش وضعیت زنده در Web UI
- Live Ranking با latency، میانگین latency، تعداد pass و speed
- Scan Progress با مرحله و درصد
- خروجی‌های تمیز TXT
- خروجی Best Configs و final ranked configs
- حالت CLI برای نسخه Python scanner

---

## 🖥️ نسخه Windows Web UI Single EXE

راحت‌ترین نسخه ویندوز، Single EXE است:

```text
RKh-CFS-win-v0.2.0.exe
```

فقط روی فایل دابل‌کلیک کن.

```

بعد Web UI را اجرا می‌کند و این آدرس را باز می‌کند:

```text
http://127.0.0.1:18080
```

### پیش‌نیاز

Single EXE فایل‌های پروژه را داخل خودش دارد، اما برای اجرای Web UI هنوز به **Python 3 نصب‌شده روی ویندوز** نیاز دارد؛ چون backend وب‌یوآی نسخه ویندوز با Python اجرا می‌شود.

بررسی نصب بودن Python:

```powershell
python --version
```

یا:

```powershell
py --version
```

اگر Python نصب نیست، Python 3 را نصب کن و دوباره EXE را اجرا کن.

---

## 🪟 نسخه Windows Web UI package

اگر نسخه Extract شده Web UI را استفاده می‌کنی، ساختار پوشه باید این‌طور باشد:

```text
RKh-CFS-v0.2.0/
├─ web_ui.py
├─ RKh-CFS-v0.2.0.py
├─ run_webui.bat
├─ run_windows.bat
├─ requirements.txt
├─ xray.exe
├─ geoip.dat
├─ geosite.dat
├─ ip-ranges/
├─ web_runtime/
└─ results/
```

نصب پیش‌نیازها:

```powershell
py -m pip install -r requirements.txt
```

اجرای Web UI:

```powershell
py web_ui.py
```

یا دابل‌کلیک روی:

```text
run_webui.bat
```

آدرس پیش‌فرض نسخه عادی Web UI:

```text
http://127.0.0.1:8080
```

---

## 🤖 نسخه Android APK v0.2.0

اسم فایل نهایی APK نسخه Android این است:

```text
RKh-CFS-Android-v0.2.0.apk
```

وضعیت نسخه:

```text
versionName: 0.2.0-android
```


### قابلیت‌های Android APK

- UI کامل Web UI حفظ شده است.
- backend بومی Go دارد.
- Chaquopy/Python backend حذف شده است.
- UI برای موبایل بهینه شده است.
- جدول Live Ranking برای گوشی فشرده‌تر شده است.
- Logs به‌صورت Native Dialog نمایش داده می‌شود.

---

## 🧭 مسیر انتخاب تارگت در Web UI

در Web UI یکی از این دو حالت را انتخاب کن:

```text
Manual targets
ISP list
```

Manual targets این مدل‌ها را قبول می‌کند:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

می‌توانی چند خط را یکجا Paste کنی:

```text
104.16.0.0/24
172.64.0.0/24
188.114.96.0-188.114.99.255
```

در حالت ISP list دسته‌ها این‌ها هستند:

```text
Iran
International
```

می‌توانی چند ISP را با checkbox انتخاب کنی یا از این گزینه‌ها استفاده کنی:

```text
Check all
Clear
```

---

## 🎯 Maximum targets

Maximum targets مشخص می‌کند چند تارگت لود/اسکن شود.

```text
0 = بدون محدودیت / همه تارگت‌ها
```

برای محدود کردن رنج‌های بزرگ، عدد وارد کن:

```text
5000
```

---

## 📊 Live Ranking

Live Ranking یک جدول زنده نشان می‌دهد:

```text
Rank | IP | Stage | Latency | Avg Latency | Pass | Speed
```

مرتب‌سازی پیش‌فرض:

```text
Latency
```

با کلیک روی این ستون‌ها می‌توانی ترتیب را تغییر بدهی:

```text
IP
Latency
Avg Latency
Pass
Speed
```

---

## 📁 فایل‌های خروجی

فایل‌ها از بخش Result Files در Web UI قابل دانلود هستند.

خروجی‌های رایج:

```text
clean_ips.txt
clean_ips_rechecked.txt
clean_ips_speed_tested.txt
best_configs.txt
final_ranked_configs.txt
selected_clean_ips.txt
selected_rechecked_ips.txt
selected_speed_tested_ips.txt
```

نسخه Python scanner ممکن است برای سازگاری CSV هم بسازد، اما مسیر اصلی دانلود Web UI روی خروجی‌های TXT تمیز تمرکز دارد.

---

# 📱 RKh-CFS Termux v0.2.0 - نسخه Android / Termux

این نسخه برای اجرای مستقیم داخل Termux آماده شده است.

## نصب و اجرا در Termux

دستورهای زیر را دقیقاً داخل Termux اجرا کنید:

```bash
pkg update -y
pkg install -y unzip
pkg install python -y
pkg install wget unzip -y
mkdir -p RKh-CFS-Termux-v0.2.0
cd RKh-CFS-Termux-v0.2.0
wget https://github.com/rezakhosh78/RKh-CF-Scanner/releases/download/v0.2.0/RKh-CFS-Termux-v0.2.0.zip
unzip RKh-CFS-Termux-v0.2.0.zip
pip install -r requirements.txt
chmod +x run.sh
./run.sh
```

## فایل‌هایی که باید دستی اضافه کنید

این پکیج عمداً فایل‌های زیر را ندارد و باید خودتان آن‌ها را کنار `run.sh` و فایل Python قرار دهید:

```text
xray
geoip.dat
geosite.dat
```

ساختار پوشه بعد از Extract و اضافه کردن فایل‌ها:

```text
RKh-CFS-Termux-v0.2.0/
├── RKh-CFS-Termux-v0.2.0.py
├── run.sh
├── requirements.txt
├── xray
├── geoip.dat
├── geosite.dat
├── ip-ranges/
├── configs/
└── results/
```

بعد از قرار دادن فایل `xray`، این دستور را هم بزنید:

```bash
chmod +x xray
```

> اندروید نمی‌تواند `xray.exe` ویندوز را اجرا کند. برای Termux باید باینری Android/Linux با نام `xray` استفاده شود.

---

## 🧪 نمونه‌های CLI

نمایش لیست ISPها:

```bash
python RKh-CFS-v0.2.0.py --list-isps
```

اسکن همه ISPهای ایران:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." --isp-category iran --isp all --max-hosts 0
```

اسکن چند ISP از دسته International:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." --isp-category international --isp Fastly Nocix --max-hosts 3000
```

اسکن دستی:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." -t 104.16.0.0/24 172.64.0.0/24 --concurrency 20
```

---

## ⚙️ نکات کاربردی

- نسخه Windows Web UI Single EXE از پورت `18080` استفاده می‌کند.
- نسخه عادی Windows Web UI از پورت `8080` استفاده می‌کند.
- concurrency را خیلی بالا نگذار.
- concurrency پیشنهادی ویندوز: `10` تا `30`
- concurrency پیشنهادی Android Go: `15` تا `30`
- برای speed-test روی موبایل، speed workers را `2` یا `3` بگذار.
- اگر runtime نسخه Single EXE خراب شد، پوشه مربوطه را از `%LOCALAPPDATA%\RKh-CFS` پاک کن و EXE را دوباره اجرا کن.

---

## ⚠️ Donate

USDT BEP20:

```text
0x304B5D9e118732C98FA60c473A763aD5076FFfb0
```

---

## ⚠️ مسئولیت استفاده

RKh-CFS برای تست کانفیگ شخصی و رنج‌های مجاز ساخته شده است. مسئولیت استفاده از ابزار با خود کاربر است.

کانال: `@pingplas_channel`
