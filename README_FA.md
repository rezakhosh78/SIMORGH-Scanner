# RKh-CFS v0.1.2 | @pingplas_channel

اسکنر Clean IP کلادفلر برای کانفیگ‌های **VLESS** با تست واقعی از طریق **Xray Core**.

این ابزار IP یا رنج IP را با همان کانفیگی که وارد می‌کنید تست می‌کند. برای هر IP، فقط آدرس سرور در outbound تغییر می‌کند و موارد مهم مثل `UUID`، `SNI`، `Host`، `Path`، نوع transport و TLS/Reality از کانفیگ اصلی حفظ می‌شوند. سپس Xray اجرا می‌شود و یک درخواست واقعی از داخل تونل عبور داده می‌شود؛ بنابراین خروجی صرفاً Ping یا TCP Check نیست.

> فقط روی IPها و رنج‌هایی استفاده کنید که اجازه تست آن‌ها را دارید. تعداد Worker را منطقی انتخاب کنید.

---

## نسخه‌ها

این پروژه دو نسخه دارد:

| نسخه | فایل اصلی | محیط اجرا |
|---|---|---|
| Windows / Python | `RKh-CFS-v0.1_2.py` | ویندوز با Python و `xray.exe` |
| Android / Termux | `rkh_cfs_termux.py` | اندروید با Termux و باینری `xray` مخصوص Android |

---

## قابلیت‌ها

- UI رنگی و مرحله‌ای داخل ترمینال
- دریافت کانفیگ VLESS از کاربر
- دریافت IP، CIDR یا Range
- انتخاب تعداد Worker قبل از اسکن
- تست واقعی با Xray Core
- ذخیره IPهای سالم همراه latency
- مرتب‌سازی خروجی بر اساس latency
- Re-check latency برای IPهای سالم بعد از اسکن
- انتخاب تعداد تست در مرحله Re-check، پیش‌فرض ۵ بار
- خروجی TXT و CSV

---

## خروجی‌ها

نتایج در پوشه `results` ذخیره می‌شوند:

```text
results/clean_ips.txt
results/clean_ips.csv
```

---

# نصب و اجرا در ویندوز

## ساختار پوشه ویندوز

بعد از خارج کردن فایل ZIP، ساختار پیشنهادی:

```text
RKh-CFS-v0.1_2/
├─ RKh-CFS-v0.1_2.py
├─ run_windows.bat
├─ requirements.txt
├─ xray.exe
├─ geoip.dat
├─ geosite.dat
├─ configs/temp/
└─ results/
```

فایل‌های زیر داخل پکیج قرار داده نشده‌اند و باید کنار اسکریپت باشند:

```text
xray.exe
geoip.dat
geosite.dat
```

## نصب پیش‌نیازها

داخل پوشه برنامه PowerShell یا CMD باز کنید و بزنید:

```powershell
py -m pip install -r requirements.txt
```

## اجرا

```powershell
py RKh-CFS-v0.1_2.py
```

یا روی فایل زیر دابل‌کلیک کنید:

```text
run_windows.bat
```

---

# نصب و اجرا در اندروید / Termux

## ساختار پوشه Termux

```text
RKh-CFS-Termux-v0.1.2/
├─ rkh_cfs_termux.py
├─ install_termux.sh
├─ run.sh
├─ requirements.txt
└─ results/
```

## نصب و اجرا

فایل ZIP نسخه Termux را به حافظه گوشی منتقل کنید، سپس در Termux:

```bash
pkg update -y
pkg install -y unzip
unzip RKh-CFS-Termux-v0.1.2.zip
cd RKh-CFS-Termux-v0.1.2
chmod +x install_termux.sh run.sh
./install_termux.sh
./run.sh
```

اسکریپت `install_termux.sh` وابستگی‌های لازم را نصب می‌کند و اگر فایل `xray` وجود نداشته باشد، باینری مناسب Android را بر اساس CPU دستگاه دانلود می‌کند.

> اندروید نمی‌تواند `xray.exe` ویندوز را اجرا کند. نسخه Termux از باینری `xray` مخصوص Android/Linux استفاده می‌کند.

---

## فرمت ورودی IP

می‌توانید یک IP، رنج یا CIDR وارد کنید:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

---

## Re-check latency

بعد از اسکن اولیه، اگر IP سالم پیدا شود، برنامه می‌پرسد آیا می‌خواهید latency دوباره بررسی شود یا نه.

در صورت تأیید:

1. تعداد تست برای هر IP پرسیده می‌شود.
2. مقدار پیش‌فرض ۵ بار است.
3. برای هر IP سالم چند تست واقعی با Xray انجام می‌شود.
4. نتیجه نهایی دوباره مرتب و ذخیره می‌شود.

---

## حالت CLI در نسخه ویندوز

نسخه Python ویندوز علاوه بر UI، حالت CLI هم دارد:

```powershell
py RKh-CFS-v0.1_2.py -c "vless://..." -t 104.16.0.0/24 --concurrency 10
```

اجرای Re-check از CLI:

```powershell
py RKh-CFS-v0.1_2.py -c "vless://..." -t 104.16.0.0/24 --concurrency 20 --recheck --recheck-samples 5 --recheck-workers 10
```

---

## نکات مهم

- برای تست واقعی، Xray باید کنار برنامه باشد یا در PATH سیستم شناسایی شود.
- در ویندوز نام فایل باید معمولاً `xray.exe` باشد.
- در Termux نام فایل باید `xray` باشد و permission اجرا داشته باشد.
- اگر کانفیگ شما Cloudflare است، مقدارهای SNI و Host باید درست باشند.
- افزایش Worker سرعت را بیشتر می‌کند، اما روی گوشی یا اینترنت ضعیف می‌تواند باعث خطا یا داغ شدن دستگاه شود.
- مقدار پیشنهادی Worker در ویندوز: ۱۰ تا ۳۰
- مقدار پیشنهادی Worker در Termux: ۵ تا ۲۰

---

## مسئولیت استفاده

این ابزار برای بررسی اتصال کانفیگ شخصی و تست IPهای مجاز طراحی شده است. استفاده از آن روی شبکه‌ها یا رنج‌هایی که اجازه تست ندارید بر عهده خود کاربر است.
