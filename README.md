![Ping Logo](ping_logo.png)

# RKh-CFS v0.2.0 | @pingplas_channel ⚡

A **Cloudflare Clean-IP Scanner** for **VLESS** configs.

RKh-CFS is built to test IPs, CIDR ranges, manual ranges and built-in ISP lists, then export ranked results.  
In the v0.2.0 line, the project now includes a modern **Web UI**, a **Windows Single EXE WebUI** build, and an optimized **Android APK**.

---

## 📦 Editions

| Edition | Main file / project | Runtime |
|---|---|---|
| Windows Web UI Single EXE | `RKh-CFS-win-v0.2.0.exe` | Windows + installed Python 3 |
| Windows Web UI package | `web_ui.py` / `run_webui.bat` | Windows + Python 3 + Xray files |
| Windows Python scanner | `RKh-CFS-v0.2.0.py` | Windows + Python 3 + `xray.exe` |
| Android APK | `RKh-CFS-Android-v0.2.0.apk` | Android APK with native Go backend |
| Android / Termux legacy | Termux package | Android + Termux + Android/Linux `xray` binary |

---

## 📥 Final release files

Final release filenames:

```text
Windows Web UI: RKh-CFS-win-v0.2.0.exe
Android APK:    RKh-CFS-Android-v0.2.0.apk
Termux ZIP:     RKh-CFS-Termux-v0.2.0.zip
```

---

## ✨ Important changes in v0.2.0

- Added **Web UI**.
- Added Persian + English language support; English is the default language.
- Added Android APK edition.
- Added **Windows Single EXE WebUI** edition.
- Added Live Ranking panel with the following columns:
  - IP
  - Stage
  - Latency
  - Avg Latency
  - Pass
  - Speed
- Live Ranking can be sorted by clicking column headers.
- Default Live Ranking sort priority is latency.
- Scan Progress now tracks the scan, re-check and speed-test stages.
- ISP list selection now supports checkboxes, multi-select, Check all and Clear.
- Stop and Continue buttons were added to the Web UI.
- Output files can be downloaded directly from the browser.
- Added Best Configs and Final Ranked Configs outputs.
- Android Go/mobile optimization:
  - Live Ranking table is more compact and phone-friendly.

---

## 🚀 Features

- VLESS config input
- Manual target input:
  - single IP
  - CIDR
  - range
  - multi-line paste
- Built-in ISP lists
- Iran and International categories
- Multi-ISP selection for scanning
- Configurable concurrency / workers
- Optional latency re-check
- Optional speed-test
- Live status in Web UI
- Live Ranking with latency, average latency, pass count and speed
- Scan Progress with stage and percentage
- Clean TXT outputs
- Best Configs and final ranked configs output
- CLI mode for the Python scanner edition

---

## 🖥️ Windows Web UI Single EXE edition

The easiest Windows edition is the Single EXE build:

```text
RKh-CFS-win-v0.2.0.exe
```

Just double-click the file.

Then it starts the Web UI and opens this address:

```text
http://127.0.0.1:18080
```

### Requirement

The Single EXE contains the project files inside itself, but running the Web UI still requires **Python 3 installed on Windows**, because the Windows Web UI backend runs with Python.

Check whether Python is installed:

```powershell
python --version
```

or:

```powershell
py --version
```

If Python is not installed, install Python 3 and run the EXE again.

---

## 🪟 Windows Web UI package

If you use the extracted Web UI package, the folder structure should look like this:

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

Install requirements:

```powershell
py -m pip install -r requirements.txt
```

Run the Web UI:

```powershell
py web_ui.py
```

Or double-click:

```text
run_webui.bat
```

Default address for the normal Web UI package:

```text
http://127.0.0.1:8080
```

---

## 🤖 Android APK v0.2.0

The final Android APK filename is:

```text
RKh-CFS-Android-v0.2.0.apk
```

Version status:

```text
versionName: 0.2.0-android
```

### Android APK features

- Full Web UI layout is preserved.
- Uses a native Go backend.
- Chaquopy/Python backend has been removed.
- UI is optimized for mobile.
- Live Ranking table is more compact for phones.
- Logs are displayed in a Native Dialog.

---

## 🧭 Target selection flow in Web UI

In the Web UI, choose one of these two modes:

```text
Manual targets
ISP list
```

Manual targets supports these formats:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

You can paste multiple lines at once:

```text
104.16.0.0/24
172.64.0.0/24
188.114.96.0-188.114.99.255
```

In ISP list mode, the categories are:

```text
Iran
International
```

You can select multiple ISPs with checkboxes or use:

```text
Check all
Clear
```

---

## 🎯 Maximum targets

Maximum targets controls how many targets are loaded/scanned.

```text
0 = no limit / all targets
```

To limit very large ranges, enter a number:

```text
5000
```

---

## 📊 Live Ranking

Live Ranking shows a live table:

```text
Rank | IP | Stage | Latency | Avg Latency | Pass | Speed
```

Default sorting:

```text
Latency
```

You can change the order by clicking these columns:

```text
IP
Latency
Avg Latency
Pass
Speed
```

---

## 📁 Output files

Files are available for download from the Result Files section in the Web UI.

Common outputs:

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

The Python scanner may also create CSV files for compatibility, but the main Web UI download flow focuses on clean TXT outputs.

---

# 📱 RKh-CFS Termux v0.2.0 - Android / Termux edition

This edition is prepared for direct execution inside Termux.

## Install and run in Termux

Run the following commands exactly inside Termux:

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

## Files you must add manually

This package intentionally does not include the following files. You must place them next to `run.sh` and the Python file yourself:

```text
xray
geoip.dat
geosite.dat
```

Folder structure after extracting and adding the files:

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

After placing the `xray` file, also run this command:

```bash
chmod +x xray
```

> Android cannot run Windows `xray.exe`. For Termux, use the Android/Linux binary named `xray`.

---

## 🧪 CLI examples

List ISP files:

```bash
python RKh-CFS-v0.2.0.py --list-isps
```

Scan all Iranian ISPs:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." --isp-category iran --isp all --max-hosts 0
```

Scan selected ISPs from the International category:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." --isp-category international --isp Fastly Nocix --max-hosts 3000
```

Manual scan:

```bash
python RKh-CFS-v0.2.0.py -c "vless://..." -t 104.16.0.0/24 172.64.0.0/24 --concurrency 20
```

---

## ⚙️ Useful notes

- Windows Web UI Single EXE uses port `18080`.
- Normal Windows Web UI uses port `8080`.
- Do not set concurrency too high.
- Suggested Windows concurrency: `10` to `30`
- Suggested Android Go concurrency: `15` to `30`
- For speed-test on mobile, set speed workers to `2` or `3`.
---

## ⚠️ Donate

USDT BEP20:

```text
0x304B5D9e118732C98FA60c473A763aD5076FFfb0
```

---

## ⚠️ Disclaimer

RKh-CFS is made for testing personal configs and authorized ranges. You are responsible for how you use it.

Channel: `@pingplas_channel`
