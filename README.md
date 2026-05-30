# RKh-CFS v0.1.2 | @pingplas_channel

A user-friendly Cloudflare Clean-IP scanner for **VLESS** configurations using the real **Xray Core** tunnel.

RKh-CFS tests each IP or IP range with the exact VLESS configuration you provide. For every candidate IP, only the outbound server address is replaced. Important fields such as `UUID`, `SNI`, `Host`, `Path`, transport type, and TLS/Reality settings are preserved from the original config. Xray Core is then started and a real request is routed through the tunnel, so the result is not just a ping or TCP-port check.

> Use this tool only on IPs and ranges that you own or are authorized to test. Keep worker counts reasonable.

---

## Editions

This project includes two editions:

| Edition | Main file | Runtime |
|---|---|---|
| Windows / Python | `RKh-CFS-v0.1_2.py` | Windows with Python and `xray.exe` |
| Android / Termux | `rkh_cfs_termux.py` | Android with Termux and the Android `xray` binary |

---

## Features

- Colorful step-by-step terminal UI
- VLESS config input
- IP, CIDR, and range input
- Worker-count selection before scanning
- Real testing through Xray Core
- Working IPs saved with latency
- Results sorted by latency
- Optional latency re-check after the first scan
- Custom sample count for re-check, default 5 tests per IP
- TXT and CSV output

---

## Output

Results are saved in the `results` folder:

```text
results/clean_ips.txt
results/clean_ips.csv
```

---

# Windows / Python Setup

## Windows folder layout

After extracting the ZIP file, the recommended layout is:

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

These Xray files are not bundled and must be placed next to the script:

```text
xray.exe
geoip.dat
geosite.dat
```

## Install dependencies

Open PowerShell or CMD inside the project folder:

```powershell
py -m pip install -r requirements.txt
```

## Run

```powershell
py RKh-CFS-v0.1_2.py
```

Or double-click:

```text
run_windows.bat
```

---

# Android / Termux Setup

## Termux folder layout

```text
RKh-CFS-Termux-v0.1.2/
├─ rkh_cfs_termux.py
├─ run.sh
├─ xray
├─ geoip.dat
├─ geosite.dat
├─ requirements.txt
└─ results/
```

## Install and run

Copy the Termux ZIP file to your phone, then run:

```bash
pkg update -y
pkg install -y unzip
pkg install python -y
pkg install wget unzip -y
mkdir -p RKh-CFS-Termux-v0.1.2
cd RKh-CFS-Termux-v0.1.2
wget https://github.com/rezakhosh78/RKh-CF-Scanner/releases/download/v0.1.2/RKh-CFS-Termux-v0.1.2.zip
unzip RKh-CFS-Termux-v0.1.2.zip
pip install -r requirements.txt
chmod +x run.sh
./run.sh
```


> Android cannot run Windows `xray.exe`. The Termux edition uses the Android/Linux `xray` binary.

---

## Target input format

You can enter a single IP, a CIDR block, or an IP range:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

---

## Latency re-check

After the first scan, if working IPs are found, the program asks whether you want to run another latency check.

If accepted:

1. The program asks for the number of tests per IP.
2. The default is 5 tests per IP.
3. Every working IP is tested again through Xray Core.
4. Final results are sorted and saved again.

---

## CLI mode on Windows

The Windows/Python edition also supports CLI mode:

```powershell
py RKh-CFS-v0.1_2.py -c "vless://..." -t 104.16.0.0/24 --concurrency 10
```

Run a latency re-check from CLI:

```powershell
py RKh-CFS-v0.1_2.py -c "vless://..." -t 104.16.0.0/24 --concurrency 20 --recheck --recheck-samples 5 --recheck-workers 10
```

---

## Important notes

- Xray Core must be next to the program or available in your system PATH.
- On Windows, the binary is usually named `xray.exe`.
- On Termux, the binary must be named `xray` and must be executable.
- For Cloudflare-based configs, make sure SNI and Host values are correct.
- More workers can make scanning faster, but on phones or weak networks it may cause failures or overheating.
- Recommended workers on Windows: 10 to 30
- Recommended workers on Termux: 5 to 20

---

## Disclaimer

This tool is intended for testing your own configuration and authorized IP ranges. You are responsible for how you use it.
