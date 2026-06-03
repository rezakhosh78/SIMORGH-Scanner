#!/data/data/com.termux/files/usr/bin/bash
set -e
cd "$(dirname "$0")"

clear
printf '\033[1;36mRKh-CFS Termux v0.2.0\033[0m | \033[1;32m@pingplas_channel\033[0m\n'
printf 'Cloudflare Clean-IP Scanner for VLESS + Xray Core\n\n'

if ! command -v python >/dev/null 2>&1; then
  echo "Python not found. Install it first: pkg install python"
  exit 1
fi

if [ ! -f "./xray" ]; then
  echo "Missing ./xray"
  echo "Place the Android/Termux xray binary beside this script, then run: chmod +x xray"
  exit 1
fi

chmod +x ./xray 2>/dev/null || true

if [ ! -f "./geoip.dat" ] || [ ! -f "./geosite.dat" ]; then
  echo "Warning: geoip.dat or geosite.dat is missing. Put them beside this script if your Xray config needs them."
  echo
fi

python ./RKh-CFS-Termux-v0.2.0.py "$@"
