#!/usr/bin/env python3
"""
RKh-CFS v0.1.4 - Colorful TUI Cloudflare Clean-IP scanner for VLESS configs.

Place xray.exe beside this script on Windows, or xray beside this script on Linux/macOS.
The scanner replaces only the outbound server address with each candidate IP while
preserving SNI/Host/path from the original VLESS config, then measures real latency
through Xray's local SOCKS proxy.

Use only on IPs/ranges you own or are authorized to test. Keep concurrency modest.
"""
from __future__ import annotations

import argparse
from bisect import bisect_right
import csv
import ipaddress
import json
import os
import platform
import random
import re
import shutil
import socket
import subprocess
import sys
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterable, Iterator, List, Optional, Tuple
from urllib.parse import parse_qs, unquote, urlparse

try:
    import requests
except Exception:
    requests = None

try:
    from rich import box
    from rich.align import Align
    from rich.console import Console
    from rich.panel import Panel
    from rich.progress import BarColumn, Progress, SpinnerColumn, TextColumn, TimeElapsedColumn
    from rich.prompt import Confirm, IntPrompt, Prompt
    from rich.table import Table
    from rich.text import Text
    RICH = True
except Exception:
    RICH = False

APP_NAME = "RKh-CFS"
APP_VERSION = "v0.1.4"
APP_CHANNEL = "@pingplas_channel"
APP_TITLE = f"{APP_NAME} {APP_VERSION}"
APP_HEADER = f"{APP_TITLE} | {APP_CHANNEL}"
DEFAULT_URL = "https://www.gstatic.com/generate_204"
LATENCY_TEST_URLS = [
    ("Google gstatic", "https://www.gstatic.com/generate_204", "Default; common Android-style connectivity check"),
    ("Cloudflare", "https://cp.cloudflare.com/generate_204", "Good match for Cloudflare Clean-IP testing"),
    ("Microsoft Edge", "https://edge.microsoft.com/captiveportal/generate_204", "Useful fallback on restricted networks"),
    ("Google connectivitycheck", "https://connectivitycheck.gstatic.com/generate_204", "Another Google connectivity endpoint"),
]
DEFAULT_SPEED_URL = "https://speed.cloudflare.com/__down?bytes={bytes}"
DEFAULT_SPEED_BYTES = 5 * 1024 * 1024
DEFAULT_TIMEOUT = 8
DEFAULT_CONCURRENCY = 10
DEFAULT_TRIES = 1
MAX_DEFAULT_HOSTS = 0  # 0 means unlimited / scan every expanded IP

console = Console() if RICH else None
_print_lock = threading.Lock()


# TUI width safety helpers.
# Important: normal panels/tables use Rich's content-sized layout (expand=False),
# so each box keeps its natural size like the earlier builds. ui_width() is kept
# only for compatibility/future use, not to force every box to the same width.
MIN_UI_WIDTH = 42
MAX_UI_WIDTH = 88

def ui_width() -> int:
    if not RICH:
        return 80
    try:
        width = int(console.size.width)
    except Exception:
        width = 80
    if width <= 0:
        width = 80
    # Never request a panel/table wider than the current terminal.
    # The previous clamp could force 42 columns even when PowerShell/Termux
    # was narrower, which broke borders after resizing.
    available = max(24, width - 2)
    if available < MIN_UI_WIDTH:
        return available
    return min(MAX_UI_WIDTH, available)

def ui_is_narrow() -> bool:
    if not RICH:
        return False
    try:
        return int(console.size.width) < 78
    except Exception:
        return False



@dataclass
class VlessConfig:
    raw: str
    uuid: str
    host: str
    port: int
    remark: str
    query: Dict[str, str]


@dataclass
class ScanResult:
    ip: str
    ok: bool
    latency_ms: Optional[float] = None
    status_code: Optional[int] = None
    error: str = ""
    speed_mbps: Optional[float] = None
    speed_bytes: int = 0
    speed_error: str = ""


@dataclass(frozen=True)
class TargetSource:
    start: int
    end: int
    version: int
    label: str = ""

    @property
    def size(self) -> int:
        return max(0, self.end - self.start + 1)


def app_dir() -> Path:
    if getattr(sys, "frozen", False):
        return Path(sys.executable).resolve().parent
    return Path(__file__).resolve().parent


def cprint(text: str = "", style: Optional[str] = None) -> None:
    if RICH:
        console.print(text, style=style)
    else:
        print(text)


def clear_screen() -> None:
    if sys.stdout.isatty():
        try:
            # Full clear + cursor home. The extra scrollback clear helps PowerShell after resizing.
            if os.name == "nt":
                os.system("cls")
            else:
                sys.stdout.write("\033[2J\033[H\033[3J")
                sys.stdout.flush()
        except Exception:
            pass
    if RICH:
        try:
            console.clear()
            return
        except Exception:
            pass
    if not sys.stdout.isatty():
        print("\n" * 2, end="")



def render_fixed_header(compact: bool = True, show_controls: bool = True) -> None:
    """Render the fixed boxed header for all post-login TUI screens.

    The first VLESS/config entry screen keeps its own large banner and does not
    call this function. Everywhere else gets a small natural-width Panel, so the
    header is boxed without forcing all other UI boxes to the same size.
    """
    if RICH:
        content = Text()
        content.append(APP_NAME, style="bold cyan")
        content.append(f" {APP_VERSION}", style="bold magenta")
        content.append("  •  Telegram: ", style="dim white")
        content.append(APP_CHANNEL, style="bold green")
        if not compact:
            content.append("\nClean-IP Scanner for VLESS + Xray", style="white")
        if show_controls:
            content.append("\n\n↑/↓ move   Space toggle   Enter confirm   Numbers still work", style="dim")
        console.print(Panel(content, border_style="cyan", box=box.ROUNDED, expand=False))
        console.print()
    else:
        print(f"==== {APP_HEADER} ====")
        if show_controls:
            print("↑/↓ move   Space toggle   Enter confirm   Numbers still work")
        print()


def show_banner(clear: bool = True) -> None:
    if clear:
        clear_screen()
    if RICH:
        logo = Text()
        logo.append("\n  ██████╗ ██╗  ██╗██╗  ██╗      ██████╗███████╗███████╗\n", style="bold cyan")
        logo.append("  ██╔══██╗██║ ██╔╝██║  ██║     ██╔════╝██╔════╝██╔════╝\n", style="bold cyan")
        logo.append("  ██████╔╝█████╔╝ ███████║     ██║     █████╗  ███████╗\n", style="bold cyan")
        logo.append("  ██╔══██╗██╔═██╗ ██╔══██║     ██║     ██╔══╝  ╚════██║\n", style="bold cyan")
        logo.append("  ██║  ██║██║  ██╗██║  ██║     ╚██████╗██║     ███████║\n", style="bold cyan")
        logo.append("  ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝      ╚═════╝╚═╝     ╚══════╝\n", style="bold cyan")
        subtitle = Text("Cloudflare Clean-IP Scanner for VLESS + Xray Core", style="white")
        version = Text(f"{APP_HEADER}", style="bold magenta")
        # Same welcome-banner style as v0.1.3. Other UI boxes stay natural/content-sized.
        console.print(Panel(Align.center(logo + Text("\n") + version + Text("\n") + subtitle), border_style="cyan", box=box.DOUBLE))
    else:
        print(f"==== {APP_HEADER} - Cloudflare Clean-IP Scanner ====")


def render_stage(title: str, subtitle: str = "", border_style: str = "cyan") -> None:
    """Clear the terminal and render only the current TUI stage.

    Stage boxes are content-sized and left-aligned. No shared fixed width is
    applied, so small messages stay small and larger tables grow naturally.
    """
    clear_screen()
    render_fixed_header(compact=True, show_controls=True)
    if RICH:
        content = f"[bold]{title}[/bold]"
        if subtitle:
            content += f"\n[dim]{subtitle}[/dim]"
        console.print(Panel(content, border_style=border_style, box=box.ROUNDED, expand=False))
    else:
        print(title)
        if subtitle:
            print(subtitle)
        print()

def pause(message: str = "Press Enter to continue") -> None:
    try:
        if RICH:
            Prompt.ask(f"[dim]{message}[/dim]", default="")
        else:
            input(message)
    except KeyboardInterrupt:
        raise


def prompt_str(message: str, default: Optional[str] = None, password: bool = False) -> str:
    if RICH:
        return Prompt.ask(message, default=default, password=password).strip()
    suffix = f" [{default}]" if default else ""
    val = input(f"{message}{suffix}: ").strip()
    return val or (default or "")


def prompt_int(message: str, default: int, min_value: int = 1, max_value: Optional[int] = None) -> int:
    while True:
        try:
            if RICH:
                value = IntPrompt.ask(message, default=default)
            else:
                raw = input(f"{message} [{default}]: ").strip()
                value = int(raw or default)
            if value < min_value or (max_value is not None and value > max_value):
                raise ValueError
            return value
        except Exception:
            cprint(f"Please enter a number between {min_value} and {max_value or '∞'}.", "yellow")


def prompt_confirm(message: str, default: bool = True) -> bool:
    if RICH:
        return Confirm.ask(message, default=default)
    raw = input(f"{message} [{'Y/n' if default else 'y/N'}]: ").strip().lower()
    if not raw:
        return default
    return raw in {"y", "yes", "1", "true"}

BACK_WORDS = {"b", "back", "q", "quit", "exit"}


def is_back_value(value: str) -> bool:
    return value.strip().lower() in BACK_WORDS


def prompt_str_nav(message: str, default: Optional[str] = None, password: bool = False, allow_back: bool = True) -> str:
    """Prompt for text and allow b/back to navigate to the previous TUI stage."""
    raw = prompt_str(message, default=default, password=password)
    if allow_back and is_back_value(raw):
        raise NavigationBack
    return raw


def prompt_int_nav(message: str, default: int, min_value: int = 1, max_value: Optional[int] = None, allow_back: bool = True) -> int:
    """Integer prompt that supports b/back in both Rich and plain terminals."""
    while True:
        try:
            label = message + (" [dim](b=back)[/dim]" if RICH and allow_back else "")
            if RICH:
                raw = Prompt.ask(label, default=str(default))
            else:
                raw = input(f"{message} [{default}]" + (" / b=back" if allow_back else "") + ": ").strip() or str(default)
            if allow_back and is_back_value(raw):
                raise NavigationBack
            value = int(str(raw).strip())
            if value < min_value or (max_value is not None and value > max_value):
                raise ValueError
            return value
        except NavigationBack:
            raise
        except Exception:
            cprint(f"Please enter a number between {min_value} and {max_value or '∞'}, or type b to go back.", "yellow")


def prompt_confirm_nav(message: str, default: bool = True, allow_back: bool = True) -> bool:
    """Yes/no prompt that supports b/back. Returns True/False, raises NavigationBack on back."""
    while True:
        if RICH:
            suffix = "Y/n" if default else "y/N"
            raw = Prompt.ask(f"{message} [dim]({suffix}, b=back)[/dim]", default="y" if default else "n")
        else:
            raw = input(f"{message} [{'Y/n' if default else 'y/N'} / b=back]: ").strip().lower()
            if not raw:
                raw = "y" if default else "n"
        raw = str(raw).strip().lower()
        if allow_back and is_back_value(raw):
            raise NavigationBack
        if raw in {"y", "yes", "1", "true"}:
            return True
        if raw in {"n", "no", "0", "false"}:
            return False
        cprint("Please answer y/n, or type b to go back.", "yellow")



def prompt_continue_nav(message: str = "Press Enter to continue", allow_back: bool = True) -> None:
    """Enter continues; b/back returns to the previous step."""
    if RICH:
        label = message + (" [dim](b=back)[/dim]" if allow_back else "")
        raw = Prompt.ask(label, default="")
    else:
        raw = input(message + (" / b=back" if allow_back else "") + ": ").strip()
    if allow_back and is_back_value(str(raw)):
        raise NavigationBack



class NavigationBack(Exception):
    """Raised internally when the user asks to go back in the TUI."""


class NavigationExit(Exception):
    """Raised internally when the user asks to exit from the TUI."""


class ScanInterrupted(Exception):
    """Raised when Ctrl+C interrupts a scan-like stage after partial results exist."""

    def __init__(self, results: List[ScanResult], stage: str = "scan") -> None:
        super().__init__(f"Interrupted during {stage}")
        self.results = results
        self.stage = stage


def _read_key() -> str:
    """Read one key press and normalize arrows/enter/backspace.

    Works on Windows through msvcrt and on Unix-like terminals through termios.
    If the terminal does not support raw key reading, callers should fall back
    to normal numeric prompts.
    """
    if os.name == "nt":
        import msvcrt
        ch = msvcrt.getwch()
        if ch in {"\x00", "\xe0"}:
            ch2 = msvcrt.getwch()
            return {"H": "up", "P": "down", "K": "left", "M": "right"}.get(ch2, "")
        if ch == "\r":
            return "enter"
        if ch == "\x1b":
            return "esc"
        if ch == "\b":
            return "backspace"
        if ch == " ":
            return "space"
        return ch

    import select
    import termios
    import tty

    fd = sys.stdin.fileno()
    old = termios.tcgetattr(fd)
    try:
        tty.setraw(fd)
        ch = sys.stdin.read(1)
        if ch == "\x1b":
            seq = ""
            for _ in range(2):
                r, _, _ = select.select([sys.stdin], [], [], 0.05)
                if r:
                    seq += sys.stdin.read(1)
            return {"[A": "up", "[B": "down", "[D": "left", "[C": "right"}.get(seq, "esc")
        if ch in {"\r", "\n"}:
            return "enter"
        if ch in {"\x7f", "\b"}:
            return "backspace"
        if ch == " ":
            return "space"
        if ch == "\x03":
            raise KeyboardInterrupt
        return ch
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, old)


def _terminal_supports_keys() -> bool:
    return bool(RICH and sys.stdin.isatty() and sys.stdout.isatty())


def _render_key_menu(title: str, rows: List[Tuple[str, str]], cursor: int, selected: Optional[set] = None,
                     typed: str = "", multi: bool = False, allow_back: bool = True) -> None:
    clear_screen()
    selected = selected or set()
    if RICH:
        render_fixed_header(compact=True, show_controls=True)
        table = Table(title=title, box=box.ROUNDED, border_style="bright_blue")
        table.add_column("", justify="center", width=3)
        table.add_column("#", justify="right", width=4, style="cyan")
        table.add_column("Item", style="bold white")
        table.add_column("Info", style="dim")
        for i, (name, detail) in enumerate(rows):
            marker = "●" if i == cursor else " "
            check = "☑" if i in selected else "☐"
            icon = check if multi else marker
            row_style = "bold black on cyan" if i == cursor else ("green" if i in selected else "")
            table.add_row(icon, str(i + 1), name, detail, style=row_style)
        console.print(table)
        help_text = "[bold]Enter[/bold]=confirm/select   [bold]↑/↓[/bold]=move"
        if multi:
            help_text += "   [bold]Space[/bold]=toggle   [bold]A[/bold]=all   [bold]N[/bold]=none"
        if allow_back:
            help_text += "   [bold]B/Esc[/bold]=back"
        help_text += "\n[dim]Numeric input examples: 1 | 1,3,5 | 2-6 | all[/dim]" if multi else "\n[dim]Numeric input: type item number then Enter[/dim]"
        if typed:
            help_text += f"\n[yellow]Typed:[/yellow] {typed}"
        console.print(Panel(help_text, border_style="magenta", expand=False))
    else:
        print(title)
        for i, (name, detail) in enumerate(rows):
            prefix = ">" if i == cursor else " "
            check = "[x]" if i in selected else "[ ]"
            print(f"{prefix} {check if multi else ''} {i+1}) {name} {detail}")
        print("Use arrows/Enter, or type numbers. B/Esc = back.")
        if typed:
            print(f"Typed: {typed}")


def key_select_menu(title: str, rows: List[Tuple[str, str]], default_index: int = 0, allow_back: bool = True) -> int:
    """Single-select menu with arrows plus numeric fallback."""
    if not rows:
        raise ValueError("Menu has no items")
    if not _terminal_supports_keys():
        render_stage(title, "Select with number. Type b/back to return." if allow_back else "Select with number.", "cyan")
        if RICH:
            table = Table(title=title, box=box.ROUNDED, border_style="cyan")
            table.add_column("#", justify="right", style="bold cyan")
            table.add_column("Item", style="bold white")
            table.add_column("Info", style="dim")
            for i, (name, detail) in enumerate(rows, 1):
                table.add_row(str(i), name, detail)
            console.print(table)
        else:
            for i, (name, detail) in enumerate(rows, 1):
                print(f"{i}) {name} {detail}")
        while True:
            raw = prompt_str("Select item" + (" / b=back" if allow_back else ""), str(default_index + 1)).strip().lower()
            if allow_back and raw in {"b", "back", "0", "q", "quit", "exit"}:
                raise NavigationBack
            if raw.isdigit() and 1 <= int(raw) <= len(rows):
                return int(raw) - 1
            cprint(f"Please choose 1-{len(rows)}.", "yellow")

    cursor = max(0, min(default_index, len(rows) - 1))
    typed = ""
    while True:
        _render_key_menu(title, rows, cursor, typed=typed, multi=False, allow_back=allow_back)
        key = _read_key()
        if key == "up":
            cursor = (cursor - 1) % len(rows)
            continue
        if key == "down":
            cursor = (cursor + 1) % len(rows)
            continue
        if key == "enter":
            if typed:
                raw = typed.strip().lower()
                typed = ""
                if allow_back and raw in {"b", "back", "0", "q", "quit", "exit"}:
                    raise NavigationBack
                if raw.isdigit() and 1 <= int(raw) <= len(rows):
                    return int(raw) - 1
                cprint("Invalid numeric selection.", "yellow")
                time.sleep(0.8)
                continue
            return cursor
        if key in {"esc"}:
            if allow_back:
                raise NavigationBack
        if key == "backspace":
            typed = typed[:-1]
            continue
        if len(key) == 1:
            if allow_back and key.lower() in {"b", "q"} and not typed:
                raise NavigationBack
            if key.isdigit():
                candidate = int(key)
                # Keep fast old-style single digit selection, but require Enter for ambiguity when >9.
                if 1 <= candidate <= len(rows) and len(rows) <= 9:
                    return candidate - 1
            if key.isprintable():
                typed += key


def key_multi_select_menu(title: str, rows: List[Tuple[str, str]], allow_back: bool = True) -> List[int]:
    """Multi-select menu with arrows/space plus old numeric syntax."""
    if not rows:
        raise ValueError("Menu has no items")
    if not _terminal_supports_keys():
        render_stage(title, "Select one or more items by number, range, or all. Type b/back to return.", "green")
        if RICH:
            table = Table(title=title, box=box.ROUNDED, border_style="green")
            table.add_column("#", justify="right", style="bold green")
            table.add_column("Item", style="bold white")
            table.add_column("Info", style="dim")
            for i, (name, detail) in enumerate(rows, 1):
                table.add_row(str(i), name, detail)
            console.print(table)
        else:
            for i, (name, detail) in enumerate(rows, 1):
                print(f"{i}) {name} {detail}")
        cprint("Select one or more items. Examples: 1 | 1,3,5 | 2-6 | all | b=back", "dim")
        while True:
            raw = prompt_str("Selection", "all")
            if allow_back and raw.strip().lower() in {"b", "back", "0", "q", "quit", "exit"}:
                raise NavigationBack
            try:
                return parse_multi_select(raw, len(rows))
            except Exception as exc:
                cprint(f"Invalid selection: {exc}", "red")

    cursor = 0
    selected = set()
    typed = ""
    while True:
        _render_key_menu(title, rows, cursor, selected=selected, typed=typed, multi=True, allow_back=allow_back)
        key = _read_key()
        if key == "up":
            cursor = (cursor - 1) % len(rows)
            continue
        if key == "down":
            cursor = (cursor + 1) % len(rows)
            continue
        if key == "space":
            if cursor in selected:
                selected.remove(cursor)
            else:
                selected.add(cursor)
            continue
        if key == "enter":
            if typed:
                raw = typed.strip().lower()
                typed = ""
                if allow_back and raw in {"b", "back", "0", "q", "quit", "exit"}:
                    raise NavigationBack
                try:
                    return parse_multi_select(raw, len(rows))
                except Exception as exc:
                    cprint(f"Invalid selection: {exc}", "red")
                    time.sleep(0.9)
                    continue
            if selected:
                return sorted(selected)
            selected.add(cursor)
            return sorted(selected)
        if key in {"esc"}:
            if allow_back:
                raise NavigationBack
        if key == "backspace":
            typed = typed[:-1]
            continue
        if len(key) == 1:
            low = key.lower()
            if allow_back and low in {"b", "q"} and not typed:
                raise NavigationBack
            if low == "a" and not typed:
                selected = set(range(len(rows)))
                continue
            if low == "n" and not typed:
                selected.clear()
                continue
            if key.isprintable():
                typed += key


def parse_vless(uri_or_file: str) -> VlessConfig:
    value = uri_or_file.strip().strip('"').strip("'")
    possible = Path(value)
    if not value.startswith("vless://") and possible.exists():
        value = possible.read_text(encoding="utf-8").strip()
    if not value.startswith("vless://"):
        raise ValueError("VLESS config must start with vless:// or be a file containing a vless:// link.")
    parsed = urlparse(value)
    uuid = parsed.username or parsed.netloc.split("@")[0]
    host = parsed.hostname or ""
    port = parsed.port or 443
    if not uuid or not host:
        raise ValueError("Invalid VLESS link: missing UUID or server address.")
    query_multi = parse_qs(parsed.query, keep_blank_values=True)
    query = {k: unquote(v[-1]) if v else "" for k, v in query_multi.items()}
    remark = unquote(parsed.fragment or "RKh-CFS")
    return VlessConfig(raw=value, uuid=uuid, host=host, port=port, remark=remark, query=query)


def clean_target_token(item: str) -> str:
    """Return the first valid-looking token from a target line.

    ISP files sometimes contain comments, trailing stars, tabs or notes like
    ``45.130.125.0/24 *``. This keeps the scanner tolerant without changing
    the original files packaged in ip-ranges/.
    """
    item = item.strip().strip(",;")
    if not item or item.startswith("#") or item.startswith("//"):
        return ""
    for marker in ("#", "//"):
        if marker in item:
            item = item.split(marker, 1)[0].strip()
    item = item.replace("*", " ")
    parts = re.split(r"[\s,;]+", item)
    return parts[0].strip() if parts else ""


def _network_host_bounds(net: ipaddress._BaseNetwork) -> Tuple[int, int]:
    """Return usable host bounds. For /31, /32 and IPv6 tiny nets keep all."""
    start = int(net.network_address)
    end = int(net.broadcast_address)
    if net.version == 4 and net.num_addresses > 2:
        start += 1
        end -= 1
    return start, end


def _target_sources(items: Iterable[str]) -> List[TargetSource]:
    sources: List[TargetSource] = []
    for item in items:
        raw_item = str(item).strip().strip('"').strip("'")
        if not raw_item:
            continue
        p = Path(raw_item)
        if p.exists() and p.is_file():
            lines = p.read_text(encoding="utf-8", errors="ignore").splitlines()
            sources.extend(_target_sources(lines))
            continue
        token = clean_target_token(raw_item)
        if not token:
            continue
        if "/" in token:
            net = ipaddress.ip_network(token, strict=False)
            start, end = _network_host_bounds(net)
            if end >= start:
                sources.append(TargetSource(start=start, end=end, version=net.version, label=token))
            continue
        if "-" in token:
            start_s, end_s = [x.strip() for x in token.split("-", 1)]
            start_ip, end_ip = ipaddress.ip_address(start_s), ipaddress.ip_address(end_s)
            if start_ip.version != end_ip.version or int(end_ip) < int(start_ip):
                raise ValueError(f"Invalid IP range: {token}")
            sources.append(TargetSource(start=int(start_ip), end=int(end_ip), version=start_ip.version, label=token))
            continue
        ip = ipaddress.ip_address(token)
        sources.append(TargetSource(start=int(ip), end=int(ip), version=ip.version, label=token))
    return sources


def _expand_sources_exact(sources: List[TargetSource], max_hosts: Optional[int] = None) -> List[str]:
    out: List[str] = []
    seen = set()
    limited = max_hosts is not None and max_hosts > 0
    for src in sources:
        for n in range(src.start, src.end + 1):
            ip = str(ipaddress.ip_address(n))
            if ip in seen:
                continue
            seen.add(ip)
            out.append(ip)
            if limited and len(out) > int(max_hosts):
                raise ValueError(f"Too many targets. Current limit is {max_hosts}. Use --max-hosts to raise it, or set it to 0 for unlimited.")
    return out


def _sample_sources_evenly(sources: List[TargetSource], max_hosts: int) -> List[str]:
    """Sample targets evenly across all selected ranges without enumerating them."""
    sources = [s for s in sources if s.size > 0]
    if not sources or max_hosts <= 0:
        return []
    totals: Dict[int, int] = {}
    by_version: Dict[int, List[TargetSource]] = {}
    for src in sources:
        totals[src.version] = totals.get(src.version, 0) + src.size
        by_version.setdefault(src.version, []).append(src)
    grand_total = sum(totals.values())
    allocations: Dict[int, int] = {}
    used = 0
    remainders: List[Tuple[float, int]] = []
    for version, total in totals.items():
        raw = (max_hosts * total) / grand_total
        alloc = max(1, int(raw))
        allocations[version] = alloc
        used += alloc
        remainders.append((raw - int(raw), version))
    while used > max_hosts:
        # take one away from the largest allocation that can spare it
        version = max((v for v, a in allocations.items() if a > 1), key=lambda v: allocations[v], default=None)
        if version is None:
            break
        allocations[version] -= 1
        used -= 1
    for _, version in sorted(remainders, reverse=True):
        if used >= max_hosts:
            break
        allocations[version] += 1
        used += 1

    out: List[str] = []
    seen = set()
    for version, version_sources in by_version.items():
        total = sum(src.size for src in version_sources)
        count = min(allocations.get(version, 0), total)
        if count <= 0:
            continue
        cumulative: List[int] = []
        run = 0
        for src in version_sources:
            run += src.size
            cumulative.append(run)
        if count == 1:
            positions = [total // 2]
        else:
            positions = [(i * (total - 1)) // (count - 1) for i in range(count)]
        for pos in positions:
            idx = bisect_right(cumulative, pos)
            prev = cumulative[idx - 1] if idx > 0 else 0
            src = version_sources[idx]
            n = src.start + (pos - prev)
            ip = str(ipaddress.ip_address(n))
            if ip not in seen:
                seen.add(ip)
                out.append(ip)
    return out[:max_hosts]


def expand_targets(items: Iterable[str], max_hosts: Optional[int] = MAX_DEFAULT_HOSTS, sample_if_too_many: bool = False) -> List[str]:
    sources = _target_sources(items)
    total = sum(src.size for src in sources)
    limit = None if max_hosts is None or max_hosts <= 0 else int(max_hosts)
    if limit is None:
        # Default behavior in v0.1.4: unlimited. Pressing Enter in the TUI scans every IP.
        return _expand_sources_exact(sources, None)
    if total <= limit:
        return _expand_sources_exact(sources, limit)
    if not sample_if_too_many:
        raise ValueError(f"Too many targets. Current limit is {limit}. Use --max-hosts to raise it, or set it to 0 for unlimited.")
    return _sample_sources_evenly(sources, limit)


def count_target_sources(items: Iterable[str]) -> Tuple[int, int]:
    sources = _target_sources(items)
    return len(sources), sum(src.size for src in sources)


def free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return int(s.getsockname()[1])


def find_xray(explicit: Optional[str] = None) -> Optional[Path]:
    candidates: List[Path] = []
    if explicit:
        candidates.append(Path(explicit))
    base = app_dir()
    exe = "xray.exe" if platform.system().lower().startswith("win") else "xray"
    candidates.extend([base / exe, base / "xray.exe", base / "xray"])
    for c in candidates:
        if c.exists() and c.is_file():
            return c.resolve()
    from_path = shutil.which("xray")
    if from_path:
        return Path(from_path).resolve()
    return None


def check_environment(xray_path: Optional[Path], show_help: bool = True) -> bool:
    ok = True
    rows = []
    base = app_dir()
    for name, exists in [
        ("xray.exe / xray", bool(xray_path)),
        ("geoip.dat", (base / "geoip.dat").exists()),
        ("geosite.dat", (base / "geosite.dat").exists()),
        ("requests[socks]", requests is not None),
    ]:
        rows.append((name, exists))
        if name in {"xray.exe / xray", "requests[socks]"} and not exists:
            ok = False
    if RICH:
        table = Table(title="Startup Check", box=box.ROUNDED, border_style="cyan")
        table.add_column("Component", style="white")
        table.add_column("Status")
        for name, exists in rows:
            table.add_row(name, "[green]FOUND[/green]" if exists else "[red]MISSING[/red]")
        console.print(table)
    else:
        for name, exists in rows:
            print(f"{'OK' if exists else 'MISSING'} - {name}")
    if not ok and show_help:
        cprint("\nPlace xray.exe/xray beside the app and install dependencies with: pip install -r requirements.txt", "yellow")
    return ok


def vless_to_xray_outbound(v: VlessConfig, server_ip: str) -> Dict:
    q = v.query
    network = (q.get("type") or q.get("network") or "tcp").lower()
    security = (q.get("security") or "none").lower()
    flow = q.get("flow", "")
    outbound = {
        "tag": "proxy",
        "protocol": "vless",
        "settings": {"vnext": [{"address": server_ip, "port": v.port, "users": [{"id": v.uuid, "encryption": q.get("encryption", "none")}]}]},
        "streamSettings": {"network": network, "security": security},
    }
    if flow:
        outbound["settings"]["vnext"][0]["users"][0]["flow"] = flow
    sni = q.get("sni") or q.get("servername") or q.get("serverName") or v.host
    fp = q.get("fp") or q.get("fingerprint") or "chrome"
    alpn = q.get("alpn", "")
    if security in {"tls", "reality"}:
        tls_key = "realitySettings" if security == "reality" else "tlsSettings"
        outbound["streamSettings"][tls_key] = {"serverName": sni, "fingerprint": fp, "allowInsecure": q.get("allowInsecure", "0") in {"1", "true"}}
        if alpn:
            outbound["streamSettings"][tls_key]["alpn"] = [x.strip() for x in alpn.split(",") if x.strip()]
        if security == "reality":
            if q.get("pbk"):
                outbound["streamSettings"][tls_key]["publicKey"] = q.get("pbk")
            if q.get("sid"):
                outbound["streamSettings"][tls_key]["shortId"] = q.get("sid")
            if q.get("spx"):
                outbound["streamSettings"][tls_key]["spiderX"] = q.get("spx")
    host_header = q.get("host") or q.get("authority") or sni
    path = q.get("path") or "/"
    if network == "ws":
        outbound["streamSettings"]["wsSettings"] = {"path": path, "headers": {"Host": host_header}}
    elif network == "grpc":
        outbound["streamSettings"]["grpcSettings"] = {"serviceName": q.get("serviceName") or q.get("serviceNameMode") or q.get("path", "").strip("/")}
    elif network in {"http", "h2"}:
        outbound["streamSettings"]["httpSettings"] = {"path": path, "host": [host_header]}
    elif network in {"httpupgrade", "httpupgrade".lower()}:
        outbound["streamSettings"]["httpupgradeSettings"] = {"path": path, "host": host_header}
    elif network in {"xhttp", "splithttp"}:
        outbound["streamSettings"]["splithttpSettings"] = {"path": path, "host": host_header}
    elif network == "tcp" and q.get("headerType") == "http":
        outbound["streamSettings"]["tcpSettings"] = {"header": {"type": "http", "request": {"path": [path], "headers": {"Host": [host_header]}}}}
    return outbound


def make_xray_config(v: VlessConfig, server_ip: str, socks_port: int, loglevel: str) -> Dict:
    return {
        "log": {"loglevel": loglevel},
        "inbounds": [{"tag": "socks-in", "listen": "127.0.0.1", "port": socks_port, "protocol": "socks", "settings": {"udp": False}}],
        "outbounds": [vless_to_xray_outbound(v, server_ip), {"tag": "direct", "protocol": "freedom"}, {"tag": "block", "protocol": "blackhole"}],
    }


def wait_port(port: int, timeout: float) -> bool:
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                return True
        except OSError:
            time.sleep(0.05)
    return False


def test_ip(v: VlessConfig, ip: str, xray: Path, timeout: int, tries: int, url: str, loglevel: str, keep_configs: bool = False) -> ScanResult:
    last_error = ""
    for _ in range(max(1, tries)):
        socks_port = free_port()
        temp_dir_obj = tempfile.TemporaryDirectory(prefix="rkh_cfs_")
        temp_dir = Path(temp_dir_obj.name)
        cfg_path = temp_dir / "config.json"
        cfg_path.write_text(json.dumps(make_xray_config(v, ip, socks_port, loglevel), indent=2), encoding="utf-8")
        proc = None
        start = time.perf_counter()
        try:
            proc = subprocess.Popen([str(xray), "run", "-config", str(cfg_path)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=str(app_dir()))
            if not wait_port(socks_port, min(3.0, timeout)):
                last_error = "Xray SOCKS port did not start"
                continue
            if requests is None:
                return ScanResult(ip=ip, ok=False, error="Python package requests[socks] is not installed")
            proxies = {"http": f"socks5h://127.0.0.1:{socks_port}", "https": f"socks5h://127.0.0.1:{socks_port}"}
            r = requests.get(url, proxies=proxies, timeout=timeout, headers={"User-Agent": f"{APP_HEADER}"})
            latency = (time.perf_counter() - start) * 1000
            if 200 <= r.status_code < 400:
                return ScanResult(ip=ip, ok=True, latency_ms=latency, status_code=r.status_code)
            last_error = f"HTTP {r.status_code}"
        except Exception as exc:
            last_error = str(exc).split("\n", 1)[0][:160]
        finally:
            if proc is not None:
                try:
                    proc.terminate()
                    proc.wait(timeout=1)
                except Exception:
                    try:
                        proc.kill()
                    except Exception:
                        pass
            if keep_configs:
                keep_dir = app_dir() / "configs" / "temp"
                keep_dir.mkdir(parents=True, exist_ok=True)
                shutil.copy2(cfg_path, keep_dir / f"{ip.replace(':','_')}_{int(time.time())}.json")
            temp_dir_obj.cleanup()
    return ScanResult(ip=ip, ok=False, error=last_error or "Failed")



def result_sort_key(r: ScanResult) -> Tuple[float, float, str]:
    """Sort by lowest latency first, then highest speed."""
    latency = r.latency_ms if r.latency_ms is not None else 10**12
    speed = r.speed_mbps if r.speed_mbps is not None else -1.0
    return (latency, -speed, r.ip)


def apply_result_filters(results: List[ScanResult], max_latency_ms: float = 0, min_speed_mbps: float = 0) -> List[ScanResult]:
    filtered: List[ScanResult] = []
    for r in results:
        if not r.ok:
            continue
        if max_latency_ms > 0 and (r.latency_ms is None or r.latency_ms > max_latency_ms):
            continue
        if min_speed_mbps > 0 and (r.speed_mbps is None or r.speed_mbps < min_speed_mbps):
            continue
        filtered.append(r)
    return sorted(filtered, key=result_sort_key)


def speed_test_ip(v: VlessConfig, base_result: ScanResult, xray: Path, timeout: int, speed_bytes: int, speed_url_template: str, loglevel: str) -> ScanResult:
    """Run a real download test through Xray for one already-working IP."""
    ip = base_result.ip
    socks_port = free_port()
    temp_dir_obj = tempfile.TemporaryDirectory(prefix="rkh_cfs_speed_")
    temp_dir = Path(temp_dir_obj.name)
    cfg_path = temp_dir / "config.json"
    cfg_path.write_text(json.dumps(make_xray_config(v, ip, socks_port, loglevel), indent=2), encoding="utf-8")
    proc = None
    out = ScanResult(
        ip=base_result.ip,
        ok=base_result.ok,
        latency_ms=base_result.latency_ms,
        status_code=base_result.status_code,
        error=base_result.error,
        speed_mbps=base_result.speed_mbps,
        speed_bytes=base_result.speed_bytes,
        speed_error=base_result.speed_error,
    )
    try:
        proc = subprocess.Popen([str(xray), "run", "-config", str(cfg_path)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, cwd=str(app_dir()))
        if not wait_port(socks_port, min(3.0, timeout)):
            out.speed_error = "Xray SOCKS port did not start"
            return out
        if requests is None:
            out.speed_error = "Python package requests[socks] is not installed"
            return out
        proxies = {"http": f"socks5h://127.0.0.1:{socks_port}", "https": f"socks5h://127.0.0.1:{socks_port}"}
        url = speed_url_template.format(bytes=speed_bytes)
        downloaded = 0
        start = time.perf_counter()
        with requests.get(url, proxies=proxies, timeout=timeout, stream=True, headers={"User-Agent": f"{APP_HEADER}"}) as r:
            if not (200 <= r.status_code < 400):
                out.speed_error = f"HTTP {r.status_code}"
                return out
            for chunk in r.iter_content(chunk_size=64 * 1024):
                if not chunk:
                    continue
                downloaded += len(chunk)
                if downloaded >= speed_bytes:
                    break
        elapsed = max(time.perf_counter() - start, 0.001)
        out.speed_bytes = downloaded
        out.speed_mbps = (downloaded * 8) / elapsed / 1_000_000
        out.speed_error = ""
        return out
    except Exception as exc:
        out.speed_error = str(exc).split("\n", 1)[0][:160]
        return out
    finally:
        if proc is not None:
            try:
                proc.terminate()
                proc.wait(timeout=1)
            except Exception:
                try:
                    proc.kill()
                except Exception:
                    pass
        temp_dir_obj.cleanup()


def run_speed_tests(v: VlessConfig, ok_results: List[ScanResult], xray: Path, workers: int, timeout: int, speed_bytes: int, speed_url_template: str, loglevel: str) -> List[ScanResult]:
    targets = sorted([r for r in ok_results if r.ok], key=result_sort_key)
    if not targets:
        return []
    results: List[ScanResult] = []
    mb = speed_bytes / 1024 / 1024
    if RICH:
        title = f"Speed testing: {mb:.1f} MB download per IP"
        progress = Progress(SpinnerColumn(), TextColumn("[bold yellow]{task.description}"), BarColumn(), TextColumn("{task.completed}/{task.total}"), TimeElapsedColumn(), console=console)
        with progress:
            task = progress.add_task(title, total=len(targets))
            ex = ThreadPoolExecutor(max_workers=workers)
            futures = {}
            try:
                futures = {ex.submit(speed_test_ip, v, r, xray, timeout, speed_bytes, speed_url_template, loglevel): r.ip for r in targets}
                for fut in as_completed(futures):
                    r = fut.result()
                    results.append(r)
                    if r.speed_mbps is not None:
                        console.print(f"[bright_green]SPD[/bright_green] {r.ip:<39} [bold]{r.speed_mbps:.2f} Mbps[/bold]  latency {r.latency_ms:.1f} ms")
                    else:
                        console.print(f"[yellow]SPD-FAIL[/yellow] {r.ip:<35} {r.speed_error}")
                    progress.advance(task)
            except KeyboardInterrupt:
                for fut in futures:
                    fut.cancel()
                ex.shutdown(wait=False, cancel_futures=True)
                raise ScanInterrupted(results, "speed test")
            finally:
                ex.shutdown(wait=False, cancel_futures=True)
    else:
        done = 0
        ex = ThreadPoolExecutor(max_workers=workers)
        futures = {}
        try:
            futures = {ex.submit(speed_test_ip, v, r, xray, timeout, speed_bytes, speed_url_template, loglevel): r.ip for r in targets}
            for fut in as_completed(futures):
                r = fut.result()
                results.append(r)
                done += 1
                if r.speed_mbps is not None:
                    print(f"SPD {r.ip:<39} {r.speed_mbps:.2f} Mbps latency {r.latency_ms:.1f} ms")
                print(f"Speed progress: {done}/{len(targets)}", end="\r")
            print()
        except KeyboardInterrupt:
            for fut in futures:
                fut.cancel()
            ex.shutdown(wait=False, cancel_futures=True)
            raise ScanInterrupted(results, "speed test")
        finally:
            ex.shutdown(wait=False, cancel_futures=True)
    return sorted(results, key=result_sort_key)


def save_results(results: List[ScanResult], output_dir: Path, filename_prefix: str = "clean_ips", max_latency_ms: float = 0, min_speed_mbps: float = 0, include_speed_errors: bool = False) -> Tuple[Path, Path]:
    """Save sorted results.

    Important behavior for speed-test output:
    - clean_ips_speed_tested.* always keeps all working IPs after speed test, even if
      speed test failed for some of them, so the file is never unexpectedly empty.
    - If filters are requested, save the filtered view with a separate prefix.
    """
    output_dir.mkdir(parents=True, exist_ok=True)
    ok = apply_result_filters(results, max_latency_ms=max_latency_ms, min_speed_mbps=min_speed_mbps)
    ok = sorted(ok, key=result_sort_key)
    txt_path = output_dir / f"{filename_prefix}.txt"
    csv_path = output_dir / f"{filename_prefix}.csv"

    lines: List[str] = []
    if not ok:
        lines.append("No results matched the selected filters.")
        if max_latency_ms > 0:
            lines.append(f"Max latency filter: {max_latency_ms} ms")
        if min_speed_mbps > 0:
            lines.append(f"Min speed filter: {min_speed_mbps} Mbps")
    else:
        for r in ok:
            latency = f"{r.latency_ms:.1f} ms" if r.latency_ms is not None else "-"
            speed = f"{r.speed_mbps:.2f} Mbps" if r.speed_mbps is not None else "-"
            extra = f" | speed_error: {r.speed_error}" if include_speed_errors and r.speed_error else ""
            lines.append(f"{r.ip} | latency: {latency} | speed: {speed} | HTTP {r.status_code}{extra}")
    txt_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    with csv_path.open("w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["ip", "latency_ms", "speed_mbps", "speed_bytes", "speed_error", "status_code", "tested_at"])
        now = datetime.now().isoformat(timespec="seconds")
        for r in ok:
            w.writerow([
                r.ip,
                f"{r.latency_ms:.1f}" if r.latency_ms is not None else "",
                f"{r.speed_mbps:.2f}" if r.speed_mbps is not None else "",
                r.speed_bytes or "",
                r.speed_error or "",
                r.status_code,
                now,
            ])
    return txt_path, csv_path

def show_config_summary(v: VlessConfig) -> None:
    q = v.query
    if RICH:
        table = Table(title="Configuration Summary", box=box.ROUNDED, border_style="green")
        table.add_column("Field", style="cyan")
        table.add_column("Value", style="white")
        table.add_row("Original Address", v.host)
        table.add_row("Port", str(v.port))
        table.add_row("Transport", q.get("type") or q.get("network") or "tcp")
        table.add_row("Security", q.get("security", "none"))
        table.add_row("SNI", q.get("sni") or q.get("servername") or v.host)
        table.add_row("Host Header", q.get("host") or q.get("authority") or q.get("sni") or v.host)
        table.add_row("Path", q.get("path", "/"))
        console.print(table)
    else:
        print(f"Address: {v.host}:{v.port} | SNI: {q.get('sni') or v.host}")



ISP_ROOT_DIR = "ip-ranges"
ISP_CATEGORIES = {
    "iran": "Iranian ISPs",
    "international": "International ISPs",
}


def isp_root() -> Path:
    return app_dir() / ISP_ROOT_DIR


def list_isp_files(category: str) -> List[Path]:
    root = isp_root() / category
    if not root.exists():
        return []
    return sorted([p for p in root.glob("*.txt") if p.is_file()], key=lambda p: p.stem.lower())


def isp_display_name(path: Path) -> str:
    return path.stem.replace("_", " ").strip()


def read_isp_stats(path: Path) -> Tuple[int, str]:
    try:
        lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
        valid = [clean_target_token(line) for line in lines]
        valid = [x for x in valid if x]
        preview = ", ".join(valid[:2]) + (" ..." if len(valid) > 2 else "")
        return len(valid), preview
    except Exception:
        return 0, ""


def parse_multi_select(raw: str, total: int) -> List[int]:
    raw = raw.strip().lower()
    if raw in {"all", "a", "*"}:
        return list(range(total))
    selected = set()
    for part in re.split(r"[\s,]+", raw):
        part = part.strip()
        if not part:
            continue
        if "-" in part:
            a, b = part.split("-", 1)
            if not a.isdigit() or not b.isdigit():
                raise ValueError("Invalid range selection")
            start, end = int(a), int(b)
            if start > end:
                start, end = end, start
            for n in range(start, end + 1):
                if not 1 <= n <= total:
                    raise ValueError("Selection out of range")
                selected.add(n - 1)
        else:
            if not part.isdigit():
                raise ValueError("Invalid selection")
            n = int(part)
            if not 1 <= n <= total:
                raise ValueError("Selection out of range")
            selected.add(n - 1)
    if not selected:
        raise ValueError("No item selected")
    return sorted(selected)


def show_main_target_menu() -> str:
    rows = [
        ("Manual IP ranges", "Enter one or more IP/CIDR/range values yourself"),
        ("ISP range list", "Pick from packaged Iranian or international ISP files"),
    ]
    idx = key_select_menu("Target Source", rows, default_index=0, allow_back=True)
    return str(idx + 1)


def ask_max_targets(default: int = MAX_DEFAULT_HOSTS) -> int:
    render_stage("Target Limit", "Choose how many targets to load before scanning.  B/back returns to the previous step.", "yellow")
    cprint("Press Enter to use the default: unlimited. That means every IP in the selected ranges will be expanded and scanned; large ranges can take a long time.", "dim")
    cprint("Enter a number only if you want to cap the scan and sample large ranges evenly.", "dim")
    while True:
        try:
            if RICH:
                raw = Prompt.ask("[bold yellow]Maximum targets to load/scan[/bold yellow] [dim](default ∞, b=back)[/dim]", default="")
            else:
                raw = input("Maximum targets to load/scan (default ∞, b=back): ").strip()
            raw = (raw or "").strip().lower()
            if not raw or raw == "0":
                return 0
            if is_back_value(raw):
                raise NavigationBack
            value = int(raw)
            if value < 1:
                raise ValueError
            return value
        except NavigationBack:
            raise
        except Exception:
            cprint("Enter a positive number, press Enter for unlimited, or type b to go back.", "yellow")


def select_manual_targets_tui() -> Tuple[List[str], str]:
    render_stage(
        "Manual Scan",
        "Paste IPs, CIDRs, or IP ranges in one batch.  b/back returns.",
        "cyan",
    )
    cprint("Paste multiple targets at once; one item per line is recommended.", "dim")
    cprint("Spaces, commas, and semicolons are also accepted.", "dim")
    cprint("After the last line, press Enter on an empty line to continue.", "dim")
    cprint("Usually this means pressing Enter twice after your pasted text.", "dim")
    cprint("")

    lines: List[str] = []
    while True:
        try:
            raw = input("› ").strip()
        except EOFError:
            raw = ""
        if not raw:
            if lines:
                break
            cprint("Enter at least one target, or type b/back to return.", "yellow")
            continue
        if not lines and is_back_value(raw):
            raise NavigationBack
        lines.append(raw)

    blob = "\n".join(lines)
    items = [x.strip() for x in re.split(r"[\s,;]+", blob) if x.strip()]
    if not items:
        raise NavigationBack
    return items, "Manual IP ranges"


def choose_isp_category_tui() -> str:
    rows = [
        ("Iranian ISPs", f"{len(list_isp_files('iran'))} packaged range file(s)"),
        ("International ISPs", f"{len(list_isp_files('international'))} packaged range file(s)"),
    ]
    idx = key_select_menu("ISP Categories", rows, default_index=0, allow_back=True)
    return "iran" if idx == 0 else "international"


def select_isp_targets_tui() -> Tuple[List[str], str]:
    while True:
        category = choose_isp_category_tui()
        files = list_isp_files(category)
        if not files:
            raise ValueError(f"No ISP files found in {isp_root() / category}")
        label = ISP_CATEGORIES.get(category, category)
        rows: List[Tuple[str, str]] = []
        for path in files:
            count, preview = read_isp_stats(path)
            rows.append((isp_display_name(path), f"{count} range line(s)  {preview}".strip()))
        try:
            idxs = key_multi_select_menu(f"{label} Range Files", rows, allow_back=True)
        except NavigationBack:
            # Back from the ISP list returns to category selection.
            continue
        selected_files = [files[i] for i in idxs]
        selected_names = ", ".join(isp_display_name(p) for p in selected_files[:5])
        if len(selected_files) > 5:
            selected_names += f", +{len(selected_files)-5} more"
        return [str(p) for p in selected_files], f"{label}: {selected_names}"


def select_targets_tui() -> Tuple[List[str], str]:
    """Select target sources, then ask the target limit exactly once."""
    while True:
        mode = show_main_target_menu()
        try:
            while True:
                if mode == "1":
                    items, target_label = select_manual_targets_tui()
                else:
                    items, target_label = select_isp_targets_tui()
                try:
                    max_hosts = ask_max_targets(MAX_DEFAULT_HOSTS)
                    targets = expand_targets(items, max_hosts=max_hosts, sample_if_too_many=True)
                    return targets, target_label
                except NavigationBack:
                    # Back from Target Limit returns to the immediately previous target selection screen.
                    continue
        except NavigationBack:
            # Back from Manual/ISP returns to the target-source menu.
            continue
        except Exception as exc:
            cprint(f"Target selection failed: {exc}", "red")
            try:
                retry = prompt_confirm_nav("Try target selection again?", True, allow_back=True)
            except NavigationBack:
                retry = True
            if not retry:
                raise


def print_isp_catalog_cli() -> None:
    for category, label in ISP_CATEGORIES.items():
        files = list_isp_files(category)
        cprint(f"\n{label}:", "bold cyan")
        for path in files:
            count, preview = read_isp_stats(path)
            cprint(f"  - {isp_display_name(path)} ({count} lines) {preview}")


def resolve_isp_files(category: str, names: Optional[List[str]]) -> List[Path]:
    files = list_isp_files(category)
    if not names or any(n.lower() == "all" for n in names):
        return files
    by_key = {p.stem.lower(): p for p in files}
    by_name = {isp_display_name(p).lower(): p for p in files}
    selected: List[Path] = []
    for name in names:
        key = name.lower().strip()
        match = by_key.get(key) or by_name.get(key)
        if not match:
            partial = [p for p in files if key in p.stem.lower() or key in isp_display_name(p).lower()]
            if len(partial) == 1:
                match = partial[0]
        if not match:
            raise ValueError(f"ISP not found in {category}: {name}")
        if match not in selected:
            selected.append(match)
    return selected


def summarize_latency(samples: List[ScanResult]) -> ScanResult:
    ok_samples = [r for r in samples if r.ok and r.latency_ms is not None]
    ip = samples[0].ip if samples else ""
    if not ok_samples:
        err = samples[-1].error if samples else "No samples"
        return ScanResult(ip=ip, ok=False, error=err)
    # Use average latency as the final score, while requiring at least one successful real Xray request.
    avg = sum(r.latency_ms or 0 for r in ok_samples) / len(ok_samples)
    status = ok_samples[-1].status_code
    return ScanResult(ip=ip, ok=True, latency_ms=avg, status_code=status, error=f"{len(ok_samples)}/{len(samples)} successful samples")


def retest_ip_latency(v: VlessConfig, ip: str, xray: Path, timeout: int, sample_count: int, url: str, loglevel: str) -> ScanResult:
    samples: List[ScanResult] = []
    for _ in range(max(1, sample_count)):
        samples.append(test_ip(v, ip, xray, timeout, 1, url, loglevel, False))
    return summarize_latency(samples)


def run_latency_recheck(v: VlessConfig, ok_results: List[ScanResult], xray: Path, workers: int, timeout: int, sample_count: int, url: str, loglevel: str) -> List[ScanResult]:
    ips = [r.ip for r in sorted(ok_results, key=lambda r: r.latency_ms or 10**9)]
    if not ips:
        return []
    results: List[ScanResult] = []
    if RICH:
        title = f"Re-checking latency: {sample_count} real Xray test(s) per IP"
        progress = Progress(SpinnerColumn(), TextColumn("[bold magenta]{task.description}"), BarColumn(), TextColumn("{task.completed}/{task.total}"), TimeElapsedColumn(), console=console)
        with progress:
            task = progress.add_task(title, total=len(ips))
            ex = ThreadPoolExecutor(max_workers=workers)
            futures = {}
            try:
                futures = {ex.submit(retest_ip_latency, v, ip, xray, timeout, sample_count, url, loglevel): ip for ip in ips}
                for fut in as_completed(futures):
                    r = fut.result()
                    results.append(r)
                    if r.ok:
                        console.print(f"[bright_green]RE-OK[/bright_green] {r.ip:<39} [bold]{r.latency_ms:.1f} ms avg[/bold]  ({r.error})")
                    else:
                        console.print(f"[red]DROP[/red]  {r.ip:<39} {r.error}")
                    progress.advance(task)
            except KeyboardInterrupt:
                for fut in futures:
                    fut.cancel()
                ex.shutdown(wait=False, cancel_futures=True)
                raise ScanInterrupted(results, "latency re-check")
            finally:
                ex.shutdown(wait=False, cancel_futures=True)
    else:
        done = 0
        ex = ThreadPoolExecutor(max_workers=workers)
        futures = {}
        try:
            futures = {ex.submit(retest_ip_latency, v, ip, xray, timeout, sample_count, url, loglevel): ip for ip in ips}
            for fut in as_completed(futures):
                r = fut.result()
                results.append(r)
                done += 1
                if r.ok:
                    print(f"RE-OK {r.ip:<39} {r.latency_ms:.1f} ms avg ({r.error})")
                print(f"Re-check progress: {done}/{len(ips)}", end="\r")
            print()
        except KeyboardInterrupt:
            for fut in futures:
                fut.cancel()
            ex.shutdown(wait=False, cancel_futures=True)
            raise ScanInterrupted(results, "latency re-check")
        finally:
            ex.shutdown(wait=False, cancel_futures=True)
    return results

def render_welcome_config_screen(xray: Optional[Path], error: str = "") -> bool:
    """One combined first page: big logo, startup check, and config entry prompt."""
    clear_screen()
    show_banner(clear=False)
    env_ok = check_environment(xray, show_help=False)
    if RICH:
        help_text = (
            "[bold]Paste your VLESS config below[/bold] or enter a file path containing one.\n"
            "[dim]q/exit = quit. This first page replaces the old separate startup screen.[/dim]"
        )
        if error:
            help_text += f"\n[red]{error}[/red]"
        console.print(Panel(help_text, border_style="magenta", box=box.ROUNDED, expand=False))
    else:
        print("Paste your VLESS config below, or enter a file path containing one. q/exit = quit.")
        if error:
            print(f"ERROR: {error}")
    return bool(env_ok and xray and requests is not None)


def read_vless_tui(xray: Optional[Path]) -> VlessConfig:
    error = ""
    while True:
        if not render_welcome_config_screen(xray, error):
            cprint("\nMissing required runtime files/dependencies. Fix them and run again.", "yellow")
            pause("Press Enter to exit")
            raise NavigationExit
        raw = prompt_str("VLESS config")
        if raw.strip().lower() in {"q", "quit", "exit", "b", "back"}:
            raise NavigationExit
        try:
            v = parse_vless(raw)
            cprint("Configuration loaded successfully.", "green")
            show_config_summary(v)
            prompt_continue_nav("Press Enter to continue to target selection", allow_back=False)
            return v
        except Exception as e:
            error = f"Invalid configuration: {e}"


def choose_latency_test_url_tui() -> str:
    """Choose the HTTP endpoint used for latency checks.

    The default stays on gstatic. Arrow-key users can move up/down and press
    Enter; numeric selection still works through key_select_menu.
    """
    rows = [(name, url) for name, url, _detail in LATENCY_TEST_URLS]
    idx = key_select_menu("Latency Test URL", rows, default_index=0, allow_back=True)
    return LATENCY_TEST_URLS[idx][1]


def ask_scan_settings_tui() -> Tuple[int, int, int, str, Path]:
    render_stage("Step 3/4 - Scan Settings", "Set scan speed manually. b/back returns to target selection.", "cyan")
    concurrency = prompt_int_nav("Scan speed / workers", DEFAULT_CONCURRENCY, 1, 200, allow_back=True)
    timeout = prompt_int_nav("Connection timeout seconds", DEFAULT_TIMEOUT, 2, 60, allow_back=True)
    tries = DEFAULT_TRIES
    test_url = choose_latency_test_url_tui()
    output_dir = app_dir() / "results"
    return concurrency, timeout, tries, test_url, output_dir


def confirm_scan_plan_tui(targets: List[str], target_label: str, concurrency: int, timeout: int, test_url: str, xray: Path, output_dir: Path) -> bool:
    render_stage("Step 4/4 - Ready", "Review the scan plan before starting. Type b/back to return to settings.", "green")
    if RICH:
        panel = (
            f"Target source: [bold]{target_label}[/bold]\n"
            f"Targets: [bold]{len(targets)}[/bold]\n"
            f"Scan workers: [bold]{concurrency}[/bold]\n"
            f"Timeout: [bold]{timeout}s[/bold]\n"
            f"Latency URL: [bold]{test_url}[/bold]\n"
            f"Xray: [bold]{xray.name}[/bold]\n"
            f"Output: [bold]{output_dir}[/bold]"
        )
        console.print(Panel(panel, title="Scan Plan", border_style="green", expand=False))
    else:
        print(f"Source: {target_label} | Targets: {len(targets)} | Workers: {concurrency} | Timeout: {timeout}s | URL: {test_url}")
    return prompt_confirm_nav("Start scan now?", True, allow_back=True)

def interactive() -> None:
    xray = find_xray()

    while True:
        try:
            v = read_vless_tui(xray)
        except NavigationExit:
            render_stage("Exited", "No scan was started.", "yellow")
            return

        while True:
            try:
                targets, target_label = select_targets_tui()
            except NavigationBack:
                # Back from target selection returns to the VLESS step.
                break

            render_stage("Targets Loaded", f"Loaded {len(targets)} target IP(s) from: {target_label}", "green")
            try:
                prompt_continue_nav("Press Enter to continue to scan settings", allow_back=True)
            except NavigationBack:
                # Back here returns to target selection, not all the way to config.
                continue

            while True:
                try:
                    concurrency, timeout, tries, test_url, output_dir = ask_scan_settings_tui()
                except NavigationBack:
                    # Back from settings returns to target selection.
                    break

                try:
                    start_scan = confirm_scan_plan_tui(targets, target_label, concurrency, timeout, test_url, xray, output_dir)
                except NavigationBack:
                    # Back from ready screen returns to scan settings.
                    continue

                if not start_scan:
                    render_stage("Scan Canceled", "No scan was started.", "yellow")
                    return

                render_stage("Scanning", f"Source: {target_label} | Targets: {len(targets)} | Workers: {concurrency}", "cyan")
                try:
                    results = run_scan(v, targets, xray, concurrency, timeout, tries, test_url, "warning", False)
                    txt, csvp = save_results(results, output_dir, "clean_ips")
                    render_stage("Scan Results", "Latency scan finished. Results are saved below.", "green")
                except ScanInterrupted as interrupted:
                    results = interrupted.results
                    txt, csvp = save_results(results, output_dir, "clean_ips_interrupted")
                    render_stage("Scan Interrupted", "Ctrl+C detected. Partial results up to this point were saved.", "yellow")
                    show_final_results(results, txt, csvp)
                    return
                show_final_results(results, txt, csvp)

                final_results = [r for r in results if r.ok]

                if final_results:
                    pause("Press Enter for optional re-check/speed-test menu")
                    render_stage("Optional Step - Latency Re-check", "Run another latency check for working IPs? Type b/back to skip.", "magenta")
                try:
                    do_recheck = bool(final_results and prompt_confirm_nav("Run a second latency check for working IPs?", True, allow_back=True))
                except NavigationBack:
                    do_recheck = False
                if do_recheck:
                    try:
                        sample_count = prompt_int_nav("Latency tests per IP", 5, 1, 20, allow_back=True)
                        recheck_workers = prompt_int_nav("Re-check workers", min(concurrency, 10), 1, 100, allow_back=True)
                    except NavigationBack:
                        sample_count = 0
                        recheck_workers = 0
                    if sample_count and recheck_workers:
                        render_stage("Re-checking Latency", f"{sample_count} real Xray test(s) per IP", "magenta")
                        try:
                            final_results = run_latency_recheck(v, final_results, xray, recheck_workers, timeout, sample_count, test_url, "warning")
                            retxt, recsv = save_results(final_results, output_dir, "clean_ips_rechecked")
                            render_stage("Re-check Results", "Second latency check finished. Results are saved below.", "green")
                        except ScanInterrupted as interrupted:
                            final_results = interrupted.results
                            retxt, recsv = save_results(final_results, output_dir, "clean_ips_rechecked_interrupted")
                            render_stage("Re-check Interrupted", "Ctrl+C detected. Partial re-check results were saved.", "yellow")
                            show_final_results(final_results, retxt, recsv)
                            return
                        show_final_results(final_results, retxt, recsv)

                if final_results:
                    pause("Press Enter for optional speed-test menu")
                    render_stage("Optional Step - Speed Test", "Run speed test for working IPs? Type b/back to skip.", "magenta")
                try:
                    do_speed = bool(final_results and prompt_confirm_nav("Run speed test for working IPs?", False, allow_back=True))
                except NavigationBack:
                    do_speed = False
                if do_speed:
                    try:
                        speed_mb = prompt_int_nav("Download size per IP (MB)", 5, 1, 200, allow_back=True)
                        speed_workers = prompt_int_nav("Speed test workers", min(concurrency, 5), 1, 50, allow_back=True)
                        speed_timeout = prompt_int_nav("Speed test timeout seconds", max(timeout, 15), 5, 120, allow_back=True)
                        speed_url = prompt_str_nav("Speed test URL", DEFAULT_SPEED_URL, allow_back=True)
                        min_speed = float(prompt_str_nav("Minimum speed filter Mbps (0 = disabled)", "0", allow_back=True) or "0")
                        max_latency = float(prompt_str_nav("Maximum latency filter ms (0 = disabled)", "0", allow_back=True) or "0")
                    except NavigationBack:
                        do_speed = False
                    if do_speed:
                        render_stage("Speed Testing", f"Download size per IP: {speed_mb} MB | Workers: {speed_workers}", "magenta")
                        try:
                            final_results = run_speed_tests(v, final_results, xray, speed_workers, speed_timeout, speed_mb * 1024 * 1024, speed_url, "warning")
                            stxt, scsv = save_results(final_results, output_dir, "clean_ips_speed_tested", include_speed_errors=True)
                            render_stage("Speed Test Results", "Speed test finished. Results are saved below.", "green")
                        except ScanInterrupted as interrupted:
                            final_results = interrupted.results
                            stxt, scsv = save_results(final_results, output_dir, "clean_ips_speed_tested_interrupted", include_speed_errors=True)
                            render_stage("Speed Test Interrupted", "Ctrl+C detected. Partial speed-test results were saved.", "yellow")
                            show_final_results(final_results, stxt, scsv)
                            return
                        show_final_results(final_results, stxt, scsv)
                        if min_speed > 0 or max_latency > 0:
                            ftxt, fcsv = save_results(final_results, output_dir, "clean_ips_speed_filtered", max_latency_ms=max_latency, min_speed_mbps=min_speed, include_speed_errors=True)
                            render_stage("Filtered Results", "Filtered speed/latency output is saved below.", "green")
                            show_final_results(final_results, ftxt, fcsv, max_latency_ms=max_latency, min_speed_mbps=min_speed)

                pause("Press Enter to exit...")
                return
            # settings Back lands here and returns to target selection.
            continue
        # target Back lands here and returns to VLESS config.
        continue


def run_scan(v: VlessConfig, targets: List[str], xray: Path, concurrency: int, timeout: int, tries: int, url: str, loglevel: str, keep_configs: bool) -> List[ScanResult]:
    results: List[ScanResult] = []
    if RICH:
        progress = Progress(SpinnerColumn(), TextColumn("[bold cyan]{task.description}"), BarColumn(), TextColumn("{task.completed}/{task.total}"), TimeElapsedColumn(), console=console)
        with progress:
            task = progress.add_task("Scanning", total=len(targets))
            ex = ThreadPoolExecutor(max_workers=concurrency)
            futures = {}
            try:
                futures = {ex.submit(test_ip, v, ip, xray, timeout, tries, url, loglevel, keep_configs): ip for ip in targets}
                for fut in as_completed(futures):
                    r = fut.result()
                    results.append(r)
                    if r.ok:
                        console.print(f"[green]OK[/green]  {r.ip:<39} [bold]{r.latency_ms:.1f} ms[/bold]  HTTP {r.status_code}")
                    progress.advance(task)
            except KeyboardInterrupt:
                for fut in futures:
                    fut.cancel()
                ex.shutdown(wait=False, cancel_futures=True)
                raise ScanInterrupted(results, "scan")
            finally:
                ex.shutdown(wait=False, cancel_futures=True)
    else:
        done = 0
        ex = ThreadPoolExecutor(max_workers=concurrency)
        futures = {}
        try:
            futures = {ex.submit(test_ip, v, ip, xray, timeout, tries, url, loglevel, keep_configs): ip for ip in targets}
            for fut in as_completed(futures):
                r = fut.result()
                results.append(r)
                done += 1
                if r.ok:
                    print(f"OK {r.ip:<39} {r.latency_ms:.1f} ms HTTP {r.status_code}")
                print(f"Progress: {done}/{len(targets)}", end="\r")
            print()
        except KeyboardInterrupt:
            for fut in futures:
                fut.cancel()
            ex.shutdown(wait=False, cancel_futures=True)
            raise ScanInterrupted(results, "scan")
        finally:
            ex.shutdown(wait=False, cancel_futures=True)
    return results


def show_final_results(results: List[ScanResult], txt_path: Path, csv_path: Path, max_latency_ms: float = 0, min_speed_mbps: float = 0) -> None:
    ok = apply_result_filters(results, max_latency_ms=max_latency_ms, min_speed_mbps=min_speed_mbps)
    failed = len([r for r in results if not r.ok])
    filtered_out = len([r for r in results if r.ok]) - len(ok)
    if RICH:
        filter_line = ""
        if max_latency_ms > 0 or min_speed_mbps > 0:
            filter_line = f"\nFiltered out: [bold yellow]{filtered_out}[/bold yellow]"
        console.print(Panel(f"[bold green]Scan Finished[/bold green]\n\nWorking IPs: [bold green]{len(ok)}[/bold green]\nFailed IPs: [bold red]{failed}[/bold red]{filter_line}", border_style="green", expand=False))
        table = Table(title="Best Results", box=box.ROUNDED, border_style="cyan")
        table.add_column("#", justify="right", style="dim")
        table.add_column("IP", style="white")
        table.add_column("Latency", justify="right", style="green")
        table.add_column("Speed", justify="right", style="yellow")
        table.add_column("HTTP", justify="right")
        for i, r in enumerate(ok[:25], 1):
            latency = f"{r.latency_ms:.1f} ms" if r.latency_ms is not None else "-"
            speed = f"{r.speed_mbps:.2f} Mbps" if r.speed_mbps is not None else "-"
            table.add_row(str(i), r.ip, latency, speed, str(r.status_code))
        console.print(table)
        console.print(f"[green]Saved TXT:[/green] {txt_path}")
        console.print(f"[green]Saved CSV:[/green] {csv_path}")
    else:
        print(f"\nScan Finished | Working: {len(ok)} | Failed: {failed} | Filtered: {filtered_out}")
        for i, r in enumerate(ok[:25], 1):
            latency = f"{r.latency_ms:.1f} ms" if r.latency_ms is not None else "-"
            speed = f"{r.speed_mbps:.2f} Mbps" if r.speed_mbps is not None else "-"
            print(f"{i}. {r.ip} latency={latency} speed={speed} HTTP {r.status_code}")
        print(f"Saved TXT: {txt_path}\nSaved CSV: {csv_path}")

def build_arg_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description=f"{APP_HEADER} - Cloudflare Clean-IP scanner for VLESS + Xray")
    p.add_argument("-c", "--config", help="VLESS URI or text file containing a VLESS URI")
    p.add_argument("-t", "--targets", nargs="*", help="IP, CIDR, range, or file path")
    p.add_argument("--xray", help="Path to xray executable. Default: xray.exe/xray beside script")
    p.add_argument("-o", "--output-dir", default=str(app_dir() / "results"), help="Output folder")
    p.add_argument("--concurrency", type=int, default=DEFAULT_CONCURRENCY)
    p.add_argument("--tries", type=int, default=DEFAULT_TRIES)
    p.add_argument("--timeout", type=int, default=DEFAULT_TIMEOUT)
    p.add_argument("--max-hosts", type=int, default=MAX_DEFAULT_HOSTS, help="Maximum targets to load. 0/default = unlimited; positive numbers cap and sample large ISP ranges.")
    p.add_argument("--url", default=DEFAULT_URL)
    p.add_argument("--loglevel", choices=["debug", "info", "warning", "error", "none"], default="warning")
    p.add_argument("--keep-configs", action="store_true")
    p.add_argument("--recheck", action="store_true", help="After the first scan, re-test working IPs and save updated results")
    p.add_argument("--recheck-samples", type=int, default=5, help="Latency tests per working IP during --recheck")
    p.add_argument("--recheck-workers", type=int, default=None, help="Workers for --recheck. Default: same as --concurrency")
    p.add_argument("--speed-test", action="store_true", help="Run a download speed test for working IPs after scanning/recheck")
    p.add_argument("--speed-workers", type=int, default=5, help="Workers for speed testing")
    p.add_argument("--speed-mb", type=int, default=5, help="Download size per IP in MB")
    p.add_argument("--speed-timeout", type=int, default=20, help="Timeout for each speed test")
    p.add_argument("--speed-url", default=DEFAULT_SPEED_URL, help="Speed test URL. Use {bytes} placeholder for download size")
    p.add_argument("--min-speed", type=float, default=0, help="Filter final output by minimum Mbps")
    p.add_argument("--max-latency", type=float, default=0, help="Filter final output by maximum latency in ms")
    p.add_argument("--list-isps", action="store_true", help="List packaged ISP range files and exit")
    p.add_argument("--isp-category", choices=["iran", "international"], help="Use packaged ISP ranges from this category")
    p.add_argument("--isp", nargs="*", help="ISP names/stems to use from --isp-category, or all")
    p.add_argument("--sample-large-ranges", action="store_true", help="Evenly sample ranges when selected targets exceed a positive --max-hosts value")
    return p


def main() -> int:
    parser = build_arg_parser()
    args = parser.parse_args()
    if args.list_isps:
        print_isp_catalog_cli()
        return 0

    using_isp_cli = bool(args.isp_category)
    if not args.config and not args.targets and not using_isp_cli:
        try:
            interactive()
            return 0
        except KeyboardInterrupt:
            cprint("\nCanceled by user.", "yellow")
            return 130

    if not args.config or (not args.targets and not using_isp_cli):
        parser.error("CLI mode requires -c/--config and either -t/--targets or --isp-category. Run without arguments for interactive mode.")
    xray = find_xray(args.xray)
    if not xray:
        cprint("xray.exe/xray not found. Place it beside the script or pass --xray.", "red")
        return 2
    if requests is None:
        cprint("Missing dependency: pip install requests[socks] rich", "red")
        return 2
    try:
        v = parse_vless(args.config)
        if using_isp_cli:
            isp_files = resolve_isp_files(args.isp_category, args.isp)
            if not isp_files:
                raise ValueError(f"No ISP files found for category: {args.isp_category}")
            targets = expand_targets([str(p) for p in isp_files], args.max_hosts, sample_if_too_many=True)
            cprint(f"Loaded {len(targets)} target(s) from {args.isp_category} ISP list.", "green")
        else:
            targets = expand_targets(args.targets, args.max_hosts, sample_if_too_many=args.sample_large_ranges)
    except Exception as e:
        cprint(str(e), "red")
        return 2
    try:
        results = run_scan(v, targets, xray, args.concurrency, args.timeout, args.tries, args.url, args.loglevel, args.keep_configs)
        txt, csvp = save_results(results, Path(args.output_dir), "clean_ips")
    except ScanInterrupted as interrupted:
        results = interrupted.results
        txt, csvp = save_results(results, Path(args.output_dir), "clean_ips_interrupted")
        cprint("Ctrl+C detected. Partial scan results were saved.", "yellow")
        show_final_results(results, txt, csvp)
        return 130
    show_final_results(results, txt, csvp)
    final_results = [r for r in results if r.ok]
    if args.recheck:
        try:
            final_results = run_latency_recheck(v, final_results, xray, args.recheck_workers or args.concurrency, args.timeout, args.recheck_samples, args.url, args.loglevel)
            txt, csvp = save_results(final_results, Path(args.output_dir), "clean_ips_rechecked")
        except ScanInterrupted as interrupted:
            final_results = interrupted.results
            txt, csvp = save_results(final_results, Path(args.output_dir), "clean_ips_rechecked_interrupted")
            cprint("Ctrl+C detected. Partial re-check results were saved.", "yellow")
            show_final_results(final_results, txt, csvp)
            return 130
        show_final_results(final_results, txt, csvp)
    if args.speed_test:
        try:
            final_results = run_speed_tests(v, final_results, xray, args.speed_workers, args.speed_timeout, args.speed_mb * 1024 * 1024, args.speed_url, args.loglevel)
            txt, csvp = save_results(final_results, Path(args.output_dir), "clean_ips_speed_tested", include_speed_errors=True)
        except ScanInterrupted as interrupted:
            final_results = interrupted.results
            txt, csvp = save_results(final_results, Path(args.output_dir), "clean_ips_speed_tested_interrupted", include_speed_errors=True)
            cprint("Ctrl+C detected. Partial speed-test results were saved.", "yellow")
            show_final_results(final_results, txt, csvp)
            return 130
        show_final_results(final_results, txt, csvp)
        if args.max_latency > 0 or args.min_speed > 0:
            txt, csvp = save_results(final_results, Path(args.output_dir), "clean_ips_speed_filtered", max_latency_ms=args.max_latency, min_speed_mbps=args.min_speed, include_speed_errors=True)
            show_final_results(final_results, txt, csvp, max_latency_ms=args.max_latency, min_speed_mbps=args.min_speed)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
