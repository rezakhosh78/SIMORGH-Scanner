# RKh-CFS v0.1.4

Cloudflare Clean-IP Scanner for VLESS configs using Xray Core.

## What's new in v0.1.4

- More colorful and user-friendly Rich-based TUI.
- Two main target modes:
  1. Manual IP/CIDR/range input.
  2. Packaged ISP range lists.
- ISP range lists are grouped into:
  - `Iranian ISPs`
  - `International ISPs`
- Multi-select ISP files with arrow keys + Space/Enter, while keeping numeric input like `1`, `1,3,5`, `2-6`, or `all`.
- Added Back support in target source, ISP category, and ISP list menus (`B` or `Esc`).
- Clears the screen between TUI stages so only the current step is visible.
- Added `ip-ranges/iran` for Iranian ISP files.
- Added `ip-ranges/international` for non-Iranian ISP files.
- More tolerant IP range parser for trailing spaces, tabs, comments, and `*` markers.
- Large ranges can be capped with a max target count and sampled evenly across all selected ranges.
- Added `--list-isps` CLI option.
- Added CLI ISP selection with `--isp-category` and `--isp`.
- Selectable latency test URL in the TUI, with `https://www.gstatic.com/generate_204` as the default.
- Added optional latency re-check after the first scan (`5 real Xray test(s) per IP` by default).
- Added optional download speed test at the end of the scan, saved in `results/clean_ips_speed_tested.csv`.

## TUI target flow

After entering the VLESS config, choose one of these target sources:

```text
1) Manual IP ranges
2) ISP range list
```

If you choose `ISP range list`, choose a category:

```text
1) Iran
2) International
```

Then select one or more ISP files from the displayed list. Use Up/Down to move, Space to toggle multiple items, and Enter to confirm. Old numeric selection still works. Press `B` or `Esc` to go back.

## Requirements

Place these files beside the Python script:

- `xray.exe` on Windows or `xray` on Linux/macOS
- `geoip.dat`
- `geosite.dat`

Install Python dependencies:

```bash
pip install -r requirements.txt
```

## Run on Windows

```powershell
py RKh-CFS-v0.1.4.py
```

Or double-click:

```text
run_windows.bat
```

## CLI examples

List packaged ISP files:

```bash
python RKh-CFS-v0.1.4.py --list-isps
```

Scan all Iranian ISP ranges from CLI:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category iran --isp all --max-hosts 0
```

Scan selected international ISP ranges:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." --isp-category international --isp Fastly Nocix --max-hosts 3000
```

Manual scan with sampling for huge ranges:

```bash
python RKh-CFS-v0.1.4.py -c "vless://..." -t 104.16.0.0/16 172.64.0.0/13 --sample-large-ranges --max-hosts 5000
```

## Latency test URL

During scan settings, you can choose the endpoint used for the real latency request. Move with Up/Down and press Enter to confirm. Numeric selection still works too.

Default:

```text
https://www.gstatic.com/generate_204
```

Available options:

```text
https://www.gstatic.com/generate_204
https://cp.cloudflare.com/generate_204
https://edge.microsoft.com/captiveportal/generate_204
https://connectivitycheck.gstatic.com/generate_204
```

For CLI mode, use `--url` to set the same value manually.

## Re-checking latency and download speed test

After the first scan, if working IPs are found, the scanner can run a second latency check. In this step, each working IP is tested several times through a real Xray connection, for example:

```text
Re-checking Latency
5 real Xray test(s) per IP
```

This means the program starts Xray again for every clean IP, sends real requests through the tunnel, repeats the test 5 times per IP by default, and then saves a more stable average latency result in:

```text
results/clean_ips_rechecked.txt
```

At the end, the scanner can also run a download speed test on the selected clean IPs. This helps separate IPs that only have good latency from IPs that are actually better for real usage. Speed-test results are saved in:

```text
results/clean_ips_speed_tested.csv
```

## Output

Results are saved in:

```text
results/clean_ips.txt
results/clean_ips.csv
results/clean_ips_rechecked.txt
results/clean_ips_speed_tested.csv
```

> Use only on IPs/ranges you own or are authorized to test.

Channel: `@pingplas_channel`

## v0.1.4 UI update

- Menus support both numeric selection and arrow-key navigation.
- Multi-select menus support Space to toggle items and Enter to confirm.
- Back navigation is available with B or Esc.

## Termux / Android package

A separate Termux package is provided. It does not include `xray`, `geoip.dat`, or `geosite.dat`. Place those files manually beside the Python file after extraction:

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

Then run:

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


## v0.1.4 notes

- The default Maximum targets value is unlimited; press Enter to scan every IP in the selected ranges. Enter a number only when you want to cap/sample large ranges.
- If you press Ctrl+C during scan / re-check / speed-test, partial results collected up to that point are saved in the results folder.


### v0.1.4 corrective rebuild
- The large RKh CFS startup logo is kept visible until Enter is pressed.
- Main TUI stages consistently support Back using `b/back` or `Esc`.
- `Maximum targets to load/scan` is now asked exactly once after target-source selection.

### Manual input refinement in this build
- Manual IP ranges no longer ask for one item at a time.
- You can paste many IP / CIDR / range values at once.
- Finish manual input with an empty line; after the last pasted line, press Enter twice to continue.
- A dim helper note is shown directly under the manual input screen.
