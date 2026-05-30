# RKh-CFS v0.1.4 | @pingplas_channel ⚡

A clean and friendly **Cloudflare Clean-IP Scanner** for **VLESS** configs, powered by the real **Xray Core** tunnel.

RKh-CFS tests every IP with the same VLESS config you give it. For each target, only the outbound server address is replaced; important fields like `UUID`, `SNI`, `Host`, `Path`, transport type, TLS and Reality settings stay untouched. Xray is then started and a real request is sent through the tunnel, so the result is more useful than a simple ping or TCP check.

---

## 📦 Editions

| Edition | Main file | Runtime |
|---|---|---|
| Windows / Python | `RKh-CFS-v0.1.4.py` | Windows with Python and `xray.exe` |
| Android / Termux | `RKh-CFS-Termux-v0.1.4.py` | Android with Termux and the Android/Linux `xray` binary |

---

## ✨ What’s new in v0.1.4

- New colorful TUI based on Rich.
- Cleaner screen flow: each step clears the previous one.
- Back navigation is available in the main stages.
- Menus support both:
  - numeric selection like `1`, `1,3,5`, `2-6`, `all`
  - arrow-key navigation with `↑ / ↓`, `Space`, and `Enter`
- Two target modes:
  1. Manual IP / CIDR / range input
  2. Built-in ISP range lists
- ISP lists are grouped into:
  - Iranian ISPs
  - International ISPs
- Manual target input supports multi-line paste.
- Default maximum targets is unlimited. Press Enter to scan all loaded IPs.
- If you press `Ctrl+C` during scan, re-check, or speed-test, partial results are saved.
- Re-checking Latency shows `5 real Xray test(s) per IP` during the optional latency re-check.
- Termux package now uses `run.sh` and does not include a separate install script.

---

## 🚀 Main features

- Real VLESS testing through Xray Core
- Manual input for IP, CIDR and IP ranges
- Paste many manual targets at once
- Built-in ISP range lists
- Iranian / International ISP grouping
- Multi-select ISP scanner
- Worker-count selection before scanning
- Optional latency re-check
- Optional speed-test flow
- TXT and CSV output
- CLI mode for automation

---

## 🧭 TUI target flow

After entering your VLESS config, choose the target source:

```text
1) Manual IP ranges
2) ISP range list
```

If you choose `ISP range list`, choose a category:

```text
1) Iran
2) International
```

Then select one or more ISP files.

Controls:

```text
↑ / ↓      move
Space      toggle selection
Enter      confirm
B / Esc    back
```

Numeric input still works too:

```text
1
1,3,5
2-6
all
```

---

## 📝 Manual target input

Manual mode accepts:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

You can paste many lines at once:

```text
104.16.0.0/24
172.64.0.0/24
188.114.96.0-188.114.99.255
```

After the last line, press Enter on an empty line to continue. In practice, after pasting your list, this usually means pressing **Enter twice**.

---

## 🎯 Maximum targets

When the program asks:

```text
Maximum targets to load/scan (default ∞)
```

Press Enter to scan every loaded target.

Enter a number only if you want to limit very large ranges:

```text
5000
```

With a limit, large ranges are sampled evenly instead of only taking the first IPs.

---

## 📁 Output

Results are saved in the `results` folder:

```text
results/clean_ips.txt
results/clean_ips.csv
results/clean_ips_rechecked.txt
results/clean_ips_speed_tested.csv
```

If the scan is interrupted with `Ctrl+C`, partial output is saved with an interrupted filename, for example:

```text
results/clean_ips_interrupted.txt
```

---

# 🪟 Windows / Python setup

## Folder layout

After extracting the Windows package, keep this layout:

```text
RKh-CFS-v0.1.4/
├─ RKh-CFS-v0.1.4.py
├─ run_windows.bat
├─ requirements.txt
├─ xray.exe
├─ geoip.dat
├─ geosite.dat
├─ ip-ranges/
├─ configs/temp/
└─ results/
```

These files are not bundled and must be placed next to the Python script:

```text
xray.exe
geoip.dat
geosite.dat
```

## Install dependencies

Open PowerShell inside the project folder:

```powershell
py -m pip install -r requirements.txt
```

## Run

```powershell
py RKh-CFS-v0.1.4.py
```

Or double-click:

```text
run_windows.bat
```

---

# 🤖 Android / Termux setup

The Termux package is separate and is named:

```text
RKh-CFS-Termux-v0.1.4.zip
```

The package does **not** include these files:

```text
xray
geoip.dat
geosite.dat
```

Place them manually next to `run.sh` after extracting the ZIP.

## Termux folder layout

```text
RKh-CFS-Termux-v0.1.4/
├─ RKh-CFS-Termux-v0.1.4.py
├─ run.sh
├─ requirements.txt
├─ xray
├─ geoip.dat
├─ geosite.dat
├─ ip-ranges/
├─ configs/temp/
└─ results/
```

## Install and run in Termux

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

Before running, make sure `xray`, `geoip.dat`, and `geosite.dat` are in the same folder as `run.sh`.

If needed, also make `xray` executable:

```bash
chmod +x xray
```

> Android cannot run Windows `xray.exe`. Use the Android/Linux `xray` binary for Termux.

---

## 🧪 CLI examples

List bundled ISP files:

```bash
python RKh-CFS-v0.1.4.py --list-isps
```

Scan all Iranian ISP ranges:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category iran --isp all --max-hosts 0
```

Scan selected international ISP ranges:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category international --isp Fastly Nocix --max-hosts 3000
```

Manual scan:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." -t 104.16.0.0/24 172.64.0.0/24 --concurrency 20
```

---

## ⚙️ Useful notes

- On Windows, the Xray binary is usually named `xray.exe`.
- On Termux, the binary must be named `xray`.
- Keep `geoip.dat` and `geosite.dat` beside the script.
- For Cloudflare-based configs, make sure `SNI` and `Host` are correct.
- More workers can be faster, but too many workers may cause errors, throttling, overheating, or unstable results.
- Suggested workers on Windows: `10` to `30`
- Suggested workers on Termux: `5` to `20`

---
## ⚠️ Donate:
USDT BEB20 : 0x304B5D9e118732C98FA60c473A763aD5076FFfb0

---

## ⚠️ Disclaimer

RKh-CFS is made for testing your own configs and authorized IP ranges. You are responsible for how you use it.

Channel: `@pingplas_channel`
