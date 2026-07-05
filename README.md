# 🐦‍🔥 SIMORGH Scanner v0.3.0

[فارسی](README_FA.md)

A Cloudflare Clean-IP scanner for VLESS configurations. SIMORGH SCANNER can test single IPs, CIDR blocks, manual ranges, and built-in ISP lists, then export ranked clean-IP and configuration results.

Version **0.3.0** introduces a complete neon-retro Web UI redesign, a fully offline standalone Windows build, and a signed Android `arm64-v8a` APK with a native Go backend.

> Use this project only with your own configurations and authorized IP ranges.

---

## 📦 Editions

| Edition | Release file | Runtime |
|---|---|---|
| Windows Standalone | `SIMORGH-Scanner-v0.3.0-Windows-x64.exe` | Fully offline; Python installation is not required |
| Android APK | `SIMORGH-Scanner-v0.3.0-arm64-v8.apk` | Android 7.0+; native Go backend; `arm64-v8a` only |

The v0.3.0 release files are available from the repository **Releases** page.

---

## ✨ What is new in v0.3.0

### 🎨 User interface

- Completely redesigned the Web UI with a neon, retro, Windows-friendly appearance.
- Renamed the interface to **SIMORGH SCANNER**.
- Added a soft neon flicker effect to the application title.
- Added a persistent header containing:
  - `FA / EN` language selector
  - animated radar
  - current application status
- Improved responsive behavior for different resolutions and window sizes.
- Removed unnecessary visual framing around the radar.

### 📑 Page structure

The Web UI is divided into four independent pages:

- 🔍 **Scanner**
- 📊 **Results**
- 📁 **Files**
- 🧾 **Log**

Additional behavior:

- **Scanner** is the default landing page.
- Starting a scan automatically opens **Results**.
- **Live Work Log** is displayed on its own page.
- A persistent bottom navigation bar is available between pages.
- A scroll-to-top button is placed in the bottom-right corner.

### 🔍 Scanner controls

The Scanner page includes:

- ▶️ Start Scan
- ⏹️ Stop
- ⏯️ Continue
- 🧹 Clear
- ❌ Cancel

Other Scanner changes:

- The default ISP category is **International**.
- The animated radar is located in the main header.
- Custom `.txt` ISP lists can be imported into the scanner.
- Manual targets and built-in ISP lists remain supported.

### 📊 Results

- Scan results are displayed as independent neon cards.
- Each scanned IP has its own result card.
- Result cards show:
  - ⚡ Latency
  - 📈 Average Latency
  - ✅ Pass
  - 🚀 Speed
- Added hover and selection animations for result cards.
- Added a neon indicator beside the results title.
- Moved `SORT BY` into its own control bubble.
- Results can be sorted by:
  - Latency
  - Average Latency
  - Pass
  - Speed

### 📶 Scan progress

The old progress bar was replaced with three independent progress cards:

- 🔵 `SCAN`
- 🟣 `RE-CHECK`
- 🟪 `SPEED`

Each card displays:

- progress percentage
- completed target count
- remaining target count
- current stage status

The summary also shows:

- ✅ Clean
- ❌ Fail
- 🔄 Re-check
- 🚀 Speed

### 📌 Floating controls

- The scanner control bubble normally stays above `SORT BY`.
- It becomes floating when its original position leaves the visible area.
- It returns automatically when scrolling upward.
- Enter and exit animations are smooth and immediate.
- Button hover effects remain active while floating.

### 🌐 Local server

The Web UI uses one fixed local address:

```text
http://127.0.0.1:21301
```

Configuration:

```text
Local IP: 127.0.0.1
Port:     21301
Version:  v0.3.0
```

The local server is bound to the loopback interface and is not intended to be exposed directly to the public network.

---

## 🪟 Windows Standalone edition

The Windows Standalone edition is the easiest way to run SIMORGH SCANNER.

1. Download the Windows Standalone x64 executable.
2. Double-click the `.exe` file.
3. The embedded runtime is extracted locally.
4. The Web UI opens at `http://127.0.0.1:21301`.

### Standalone characteristics

- No Python installation is required.
- No `pip install` command is required.
- No initial internet connection is required to unpack or start the application.
- Python runtime, Python dependencies, scanner files, Xray files, and Web UI assets are bundled.
- Internet access is required only when the user actually scans or tests IPs.
- Application data is extracted under the current Windows user profile.

Because the executable is not necessarily code-signed with a public certificate, Windows SmartScreen may display a warning on first launch.

---

## 🤖 Android APK v0.3.0

Official APK filename:

```text
SIMORGH-Scanner-v0.3.0-arm64-v8.apk
```

Android build configuration:

```text
versionName: 0.3.0
ABI:         arm64-v8a
minSdk:      24
targetSdk:   35
```

### Android features

- Uses a native Go backend.
- Does not use a Python or Chaquopy backend.
- Preserves the SIMORGH SCANNER Web UI layout.
- Optimized for mobile screens.
- Uses the same local Web UI address: `127.0.0.1:21301`.
- Includes the native Android Xray library.
- Official release builds are aligned, signed, and verified by GitHub Actions.

> The provided APK supports `arm64-v8a` devices only.

---

## 🎯 Target selection

Choose one of these input modes:

```text
Manual targets
ISP list
```

Manual targets support:

```text
104.16.0.1
104.16.0.0/24
104.16.0.1-104.16.0.255
```

Multiple lines can be pasted at once:

```text
104.16.0.0/24
172.64.0.0/24
188.114.96.0-188.114.99.255
```

Built-in ISP categories:

```text
Iran
International
```

The ISP selector supports:

- multi-selection
- Check all
- Clear
- imported `.txt` ISP lists

---

## ⚙️ Scanner features

- VLESS configuration input
- Single IP, CIDR, range, and multi-line target input
- Built-in Iranian and international ISP lists
- Custom ISP `.txt` import
- Configurable worker count
- Optional latency re-check
- Optional speed test
- Stop, Continue, Clear, and Cancel controls
- Live application status
- Live result ranking
- Separate Scanner, Results, Files, and Log pages
- Ranked TXT outputs
- Best Configs and Final Ranked Configs

---

## 🔢 Maximum targets

`Maximum targets` controls how many targets are loaded and scanned:

```text
0 = no limit / all targets
```

Example limit:

```text
5000
```

Use a reasonable target limit and worker count when scanning large ranges.

---

## 📁 Output files

Output files are available from the **Files** page.

Common outputs include:

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

The Python scanner may also create CSV files for compatibility, while the main Web UI workflow focuses on ranked TXT output.

---

## 🛠️ Fixes in v0.3.0

- Fixed custom font loading.
- Fixed radar positioning.
- Fixed the header not remaining persistent across pages.
- Fixed floating control behavior and transition delays.
- Fixed unwanted white borders during hover.
- Fixed the Windows conflict between an `xray` file and the `Xray` directory.
- Fixed ZIP extraction problems on Windows and WinRAR.
- Updated the paths for `xray.exe`, `geoip.dat`, and `geosite.dat`.
- Fixed local-access-token errors in the standalone Web UI.
- Improved standalone extraction and local runtime handling.

---

## 🔒 Preserved behavior

- Core IP scanning logic remains available.
- Latency, re-check, pass count, and speed testing remain available.
- Ranked TXT exports remain available.
- Iranian and international ISP lists remain supported.

---

## ⚙️ Useful notes

- Local Web UI address: `http://127.0.0.1:21301`
- Suggested Windows workers: `10` to `30`
- Suggested Android Go workers: `15` to `30`
- Suggested mobile speed workers: `2` or `3`
- Excessively high concurrency may reduce reliability.

---

## 💚 Donate

USDT BEP20:

```text
0x304B5D9e118732C98FA60c473A763aD5076FFfb0
```

---

## ⚠️ Disclaimer

SIMORGH Scanner is intended for testing personal configurations and authorized ranges. You are responsible for how you use it.

Channel: `@pingplas_channel`
