# RKh-CFS v0.1.4

اسکنر Clean-IP کلادفلر برای کانفیگ‌های VLESS با استفاده از Xray Core.

## تغییرات نسخه v0.1.4

- تبدیل منوی تعاملی به TUI رنگی‌تر و کاربرپسندتر با Rich.
- اضافه شدن انتخاب منبع اسکن در دو حالت اصلی:
  1. اسکن دستی IP / CIDR / Rangeهایی که کاربر وارد می‌کند.
  2. اسکن از روی لیست آماده ISPها.
- دسته‌بندی لیست‌های آماده ISP به دو بخش:
  - `Iranian ISPs`
  - `International ISPs`
- امکان انتخاب یک یا چند ISP هم با کلیدهای جهت‌دار و Space/Enter، هم با فرمت‌های عددی `1`، `1,3,5`، `2-6` یا `all`.
- اضافه شدن دکمه/کلید بازگشت در منوهای انتخاب منبع، دسته ISP و لیست ISPها (`B` یا `Esc`).
- پاک‌سازی صفحه در هر مرحله؛ وقتی به مرحله بعد می‌روی، مرحله قبل دیگر نمایش داده نمی‌شود.
- اضافه شدن پوشه `ip-ranges/iran` برای ISPهای ایرانی.
- اضافه شدن پوشه `ip-ranges/international` برای ISPهای غیرایرانی.
- پشتیبانی بهتر از فایل‌های IP Range دارای فاصله، تب، کامنت یا علامت `*` در انتهای خط.
- برای Rangeهای خیلی بزرگ، امکان تعیین سقف تعداد Target و نمونه‌برداری یکنواخت از کل بازه‌ها.
- اضافه شدن گزینه CLI برای دیدن لیست ISPها: `--list-isps`.
- اضافه شدن اسکن CLI از روی ISPها با `--isp-category` و `--isp`.
- انتخاب URL تست latency داخل TUI با کلیدهای بالا/پایین؛ پیش‌فرض روی `https://www.gstatic.com/generate_204` است.
- اضافه شدن Re-checking Latency بعد از اسکن اولیه؛ به‌صورت پیش‌فرض هر IP سالم با `5 real Xray test(s) per IP` دوباره تست می‌شود.
- اضافه شدن تست سرعت دانلود در انتهای اسکن و ذخیره نتیجه در `results/clean_ips_speed_tested.csv`.

## ساختار انتخاب اسکن در TUI

بعد از وارد کردن کانفیگ VLESS، برنامه این دو مسیر را نشان می‌دهد:

```text
1) Manual IP ranges
2) ISP range list
```

اگر گزینه `ISP range list` را انتخاب کنی، برنامه دوباره دو دسته نشان می‌دهد:

```text
1) Iran
2) International
```

بعد از ورود به هر دسته، لیست ISPهای همان دسته نمایش داده می‌شود. با کلیدهای بالا/پایین بین آیتم‌ها جابه‌جا شو، با `Space` چند ISP را انتخاب کن و با `Enter` تأیید کن. انتخاب عددی قبلی هم هنوز فعال است، مثل `1,3,5` یا `all`. برای برگشت هم `B` یا `Esc` را بزن.

## پیش‌نیازها

این فایل‌ها را کنار اسکریپت قرار بده:

- `xray.exe` در ویندوز یا `xray` در لینوکس/مک
- `geoip.dat`
- `geosite.dat`

نصب وابستگی‌ها:

```bash
pip install -r requirements.txt
```

## اجرا در ویندوز

```powershell
py RKh-CFS-v0.1.4.py
```

یا روی فایل زیر دابل‌کلیک کن:

```text
run_windows.bat
```

## نمونه اجرای CLI

نمایش لیست ISPها:

```bash
python RKh-CFS-v0.1.4.py --list-isps
```

اسکن همه ISPهای ایرانی از روی CLI:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category iran --isp all --max-hosts 0
```

اسکن چند ISP خاص از دسته بین‌المللی:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category international --isp Fastly Nocix --max-hosts 3000
```

اسکن دستی و نمونه‌برداری از Rangeهای بزرگ:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." -t 104.16.0.0/16 172.64.0.0/13 --sample-large-ranges --max-hosts 5000
```

## URL تست Latency

در بخش تنظیمات اسکن، می‌توانید آدرس تست latency را انتخاب کنید. با کلیدهای بالا/پایین جابه‌جا شوید و با Enter تأیید کنید. انتخاب عددی هم مثل قبل کار می‌کند.

پیش‌فرض برنامه:

```text
https://www.gstatic.com/generate_204
```

گزینه‌های داخل برنامه:

```text
https://www.gstatic.com/generate_204
https://cp.cloudflare.com/generate_204
https://edge.microsoft.com/captiveportal/generate_204
https://connectivitycheck.gstatic.com/generate_204
```

در حالت CLI هم می‌توانید همین مقدار را با `--url` دستی تنظیم کنید.

## بررسی دوباره Latency و تست سرعت دانلود

بعد از اسکن اولیه، اگر IP سالم پیدا شود، برنامه می‌تواند یک مرحله بررسی دوباره latency انجام دهد. در این مرحله، هر IP سالم چند بار با اتصال واقعی Xray تست می‌شود؛ مثلاً:

```text
Re-checking Latency
5 real Xray test(s) per IP
```

یعنی برنامه برای هر Clean IP دوباره Xray را اجرا می‌کند، درخواست واقعی را از داخل تونل می‌فرستد، به‌صورت پیش‌فرض هر IP را ۵ بار تست می‌کند و بعد میانگین latency پایدارتر را ذخیره می‌کند:

```text
results/clean_ips_rechecked.txt
```

در پایان اسکن، امکان تست سرعت دانلود هم وجود دارد. این مرحله کمک می‌کند IPهایی که فقط latency خوبی دارند از IPهایی که واقعاً برای استفاده بهتر هستند جدا شوند. نتیجه تست سرعت دانلود در این فایل ذخیره می‌شود:

```text
results/clean_ips_speed_tested.csv
```

## خروجی

نتایج در پوشه زیر ذخیره می‌شود:

```text
results/
```

فایل‌های اصلی خروجی:

```text
results/clean_ips.txt
results/clean_ips.csv
results/clean_ips_rechecked.txt
results/clean_ips_speed_tested.csv
```

> فقط روی IPها و Rangeهایی اسکن انجام بده که مالک آن‌ها هستی یا اجازه تست آن‌ها را داری.

کانال: `@pingplas_channel`

## تغییرات تکمیلی نسخه 0.1.4

- انتخاب‌ها هم با عدد و هم با کلیدهای جهت بالا/پایین قابل انجام هستند.
- در منوهای چندانتخابی از Space برای انتخاب/حذف انتخاب و Enter برای تأیید استفاده کنید.
- برگشت به مرحله قبل با B یا Esc امکان‌پذیر است.

## نسخه Termux / Android

یک پکیج جداگانه برای Termux هم آماده شده است. فایل‌های `xray`، `geoip.dat` و `geosite.dat` داخل پکیج قرار داده نشده‌اند. بعد از استخراج پکیج Termux، این سه فایل را دستی کنار فایل Python قرار دهید:

```bash
RKh-CFS-Termux-v0.1.4/
├── RKh-CFS-Termux-v0.1.4.py
├── run.sh
├── requirements.txt
├── xray
├── geoip.dat
├── geosite.dat
├── ip-ranges/
├── configs/
└── results/
```

سپس در Termux:

```bash
pkg update -y
pkg install -y unzip
pkg install python -y
pkg install wget unzip -y
mkdir -p RKh-CFS-Termux-v0.1.4
cd RKh-CFS-Termux-v0.1.4
wget https://github.com/rezakhosh78/RKh-CF-Scanner/releases/download/v0.1.4/RKh-CFS-Termux-v0.1.4.zip
unzip RKh-CFS-Termux-v0.1.4.zip
pip install -r requirements.txt
chmod +x run.sh
./run.sh
```


## نکته‌های نسخه 0.1.4

- مقدار پیش‌فرض Maximum targets برابر بی‌نهایت است؛ با Enter همه IPهای داخل رنج‌های انتخاب‌شده اسکن می‌شوند. برای محدود کردن، یک عدد وارد کنید.
- اگر هنگام scan / re-check / speed-test کلید Ctrl+C را بزنید، نتیجه‌های جمع‌شده تا همان لحظه در پوشه results ذخیره می‌شوند.


### v0.1.4 اصلاحی
- لوگوی بزرگ RKh CFS در صفحه شروع حفظ شده و قبل از ورود به مراحل با Enter ادامه می‌دهید.
- در مراحل اصلی TUI گزینه برگشت با `b/back` یا `Esc` یکدست شده است.
- پرسش `Maximum targets to load/scan` فقط یک‌بار بعد از انتخاب منبع Target نمایش داده می‌شود.


## تغییرات اصلاحی TUI در همین نسخه

- در مرحله Targets Loaded هم گزینه برگشت اضافه شد: `b` یا `back`.
- پرسش `Maximum targets to load/scan` فقط در یک مرحله مستقل بعد از انتخاب منبع Target نمایش داده می‌شود.

### اصلاح ورودی دستی در همین نسخه
- بخش Manual IP ranges دیگر آیتم‌ها را تک‌به‌تک نمی‌پرسد.
- می‌توانید چندین IP / CIDR / Range را یکجا Paste کنید.
- پایان ورود دستی با یک خط خالی است؛ یعنی بعد از آخرین خط، دوبار Enter بزنید تا برنامه به مرحله بعد برود.
- متن راهنمای کم‌رنگ زیر همان بخش اضافه شده است.
