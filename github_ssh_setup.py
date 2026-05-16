#!/usr/bin/env python3
"""
github_ssh_setup.py — Production-Grade GitHub SSH Setup
========================================================
Cross-platform: Windows 10+ / macOS 10.12+ / Linux
Requires : Python 3.8+  |  Zero external dependencies
Version  : 2.0.0

Usage
-----
  python github_ssh_setup.py                          # interactive
  python github_ssh_setup.py -e you@example.com       # with email
  python github_ssh_setup.py -e you@example.com -k github_work
  python github_ssh_setup.py --dry-run                # preview only
  python github_ssh_setup.py --verbose                # debug output
  python github_ssh_setup.py --force                  # overwrite existing key
"""

from __future__ import annotations

import argparse
import logging
import os
import platform
import re
import shutil
import socket
import subprocess
import sys
import textwrap
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional

# ─────────────────────────────────────────────────────────────────────────────
# Constants
# ─────────────────────────────────────────────────────────────────────────────

VERSION      = "2.0.0"
MIN_PYTHON   = (3, 8)
GITHUB_HOST  = "github.com"
MAX_RETRIES  = 5
RETRY_DELAY  = 2          # seconds between auto-retries on transient failures
SSH_TIMEOUT  = 10         # seconds for ssh -T connection test

# GitHub's official fingerprints (SHA256)
# Source: https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints
GITHUB_FINGERPRINTS = {
    "ED25519":  "SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
    "ECDSA":    "SHA256:p2QAMXNIC1TJYWeIOttrVc98/R1BUFWu3/LiyKgUfQM",
    "RSA":      "SHA256:uNiVztksCsDhcc0u9e8BujQXVUpKZIDTMczCvj3tD2s",
}

# ─────────────────────────────────────────────────────────────────────────────
# Platform detection
# ─────────────────────────────────────────────────────────────────────────────

SYSTEM   = platform.system()   # "Windows" | "Darwin" | "Linux"
IS_WIN   = SYSTEM == "Windows"
IS_MAC   = SYSTEM == "Darwin"
IS_LINUX = SYSTEM == "Linux"
RELEASE  = platform.release()
HOSTNAME = socket.gethostname()

# ─────────────────────────────────────────────────────────────────────────────
# ANSI color — zero dependencies
# Enable VT processing on Windows (works on Win10+ conhost, WT, pwsh, cmd)
# ─────────────────────────────────────────────────────────────────────────────

if IS_WIN:
    os.system("")   # flips ENABLE_VIRTUAL_TERMINAL_PROCESSING in the console

R   = "\033[0m"      # reset
B   = "\033[1m"      # bold
DIM = "\033[2m"      # dim
RE  = "\033[0;31m"   # red
GR  = "\033[0;32m"   # green
YE  = "\033[1;33m"   # yellow
CY  = "\033[0;36m"   # cyan
MG  = "\033[0;35m"   # magenta

# ─────────────────────────────────────────────────────────────────────────────
# Custom exceptions — typed, named, clean control flow
# ─────────────────────────────────────────────────────────────────────────────

class SetupError(RuntimeError):
    """Unrecoverable setup failure — will exit with code 1."""

class ToolMissingError(SetupError):
    """A required CLI tool could not be found or installed.  exit 2."""

class AgentError(SetupError):
    """Could not start or connect to ssh-agent.  exit 4."""

class GithubConnectionError(SetupError):
    """GitHub SSH connection could not be verified.  exit 5."""

class ValidationError(ValueError):
    """User-supplied input failed validation.  exit 3."""

# ─────────────────────────────────────────────────────────────────────────────
# Config dataclass — single source of truth for all runtime state
# ─────────────────────────────────────────────────────────────────────────────

@dataclass
class Config:
    email:    str
    key_name: str  = "id_ed25519"
    dry_run:  bool = False
    force:    bool = False
    verbose:  bool = False
    retries:  int  = MAX_RETRIES

    # Derived paths — set in __post_init__
    ssh_dir:     Path = field(init=False)
    key_path:    Path = field(init=False)
    pub_path:    Path = field(init=False)
    ssh_config:  Path = field(init=False)
    known_hosts: Path = field(init=False)
    agent_env:   Path = field(init=False)

    def __post_init__(self) -> None:
        self.ssh_dir     = Path.home() / ".ssh"
        self.key_path    = self.ssh_dir / self.key_name
        self.pub_path    = Path(str(self.key_path) + ".pub")
        self.ssh_config  = self.ssh_dir / "config"
        self.known_hosts = self.ssh_dir / "known_hosts"
        self.agent_env   = self.ssh_dir / "agent.env"

# ─────────────────────────────────────────────────────────────────────────────
# Logging — dual channel: timestamped file + pretty terminal
# ─────────────────────────────────────────────────────────────────────────────

LOG_FILE = Path.home() / ".ssh" / "github_setup.log"
log: logging.Logger   # assigned in main()


def _setup_logging(verbose: bool) -> logging.Logger:
    """
    Two handlers:
      StreamHandler  → pretty coloured output to stdout (INFO+ normal, DEBUG if -v)
      FileHandler    → plain timestamped log to ~/.ssh/github_setup.log (always DEBUG)
    """
    logger = logging.getLogger("github_ssh_setup")
    logger.setLevel(logging.DEBUG)

    console = logging.StreamHandler(sys.stdout)
    console.setLevel(logging.DEBUG if verbose else logging.INFO)
    console.setFormatter(logging.Formatter("%(message)s"))
    logger.addHandler(console)

    try:
        Path.home().joinpath(".ssh").mkdir(exist_ok=True)
        fh = logging.FileHandler(LOG_FILE, encoding="utf-8")
        fh.setLevel(logging.DEBUG)
        fh.setFormatter(
            logging.Formatter(
                "%(asctime)s [%(levelname)-8s] %(message)s",
                datefmt="%Y-%m-%d %H:%M:%S",
            )
        )
        logger.addHandler(fh)
    except OSError:
        pass  # non-fatal; continue without file logging

    return logger

# ─────────────────────────────────────────────────────────────────────────────
# Pretty-print helpers
# ─────────────────────────────────────────────────────────────────────────────

def _print(msg: str = "") -> None:
    print(msg)
    log.debug("OUT: %s", msg)

def info(msg: str)  -> None:
    print(f"{CY}[INFO]{R}  {msg}")
    log.info("INFO: %s", msg)

def ok(msg: str)    -> None:
    print(f"{GR}[ OK ]{R}  {msg}")
    log.info("OK: %s", msg)

def warn(msg: str)  -> None:
    print(f"{YE}[WARN]{R}  {msg}")
    log.warning("WARN: %s", msg)

def error(msg: str) -> None:
    print(f"{RE}[ERR ]{R}  {msg}", file=sys.stderr)
    log.error("ERR: %s", msg)

def dbg(msg: str)   -> None:
    log.debug("DBG: %s", msg)

def step(n: int, total: int, title: str) -> None:
    print(f"\n{B}{CY}── [{n}/{total}] {title} {R}")
    log.info("STEP %d/%d: %s", n, total, title)

def divider(char: str = "─", width: int = 62) -> None:
    print(f"{DIM}{char * width}{R}")

def box(lines: list, color: str = B) -> None:
    width = max(len(ln) for ln in lines) + 4
    print(f"{color}╔{'═' * width}╗{R}")
    for ln in lines:
        pad = width - len(ln) - 2
        print(f"{color}║ {ln}{' ' * pad} ║{R}")
    print(f"{color}╚{'═' * width}╝{R}")

# ─────────────────────────────────────────────────────────────────────────────
# Argument parsing
# ─────────────────────────────────────────────────────────────────────────────

def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="github_ssh_setup",
        description=textwrap.dedent("""\
            Production-grade GitHub SSH key setup.
            Supports Windows 10+ / macOS 10.12+ / Linux — zero dependencies.
        """),
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=textwrap.dedent("""\
            Examples:
              python github_ssh_setup.py
              python github_ssh_setup.py -e you@example.com
              python github_ssh_setup.py -e you@example.com -k work_key
              python github_ssh_setup.py --dry-run --verbose
              python github_ssh_setup.py --force
        """),
    )
    parser.add_argument("-e", "--email",    metavar="EMAIL",
                        help="GitHub email (prompted if omitted)")
    parser.add_argument("-k", "--key-name", metavar="NAME", default="id_ed25519",
                        help="Key filename inside ~/.ssh  (default: id_ed25519)")
    parser.add_argument("--force",   action="store_true",
                        help="Overwrite an existing key at the target path")
    parser.add_argument("--dry-run", action="store_true",
                        help="Preview every action without making any changes")
    parser.add_argument("-v", "--verbose", action="store_true",
                        help="Enable debug-level output")
    parser.add_argument("--retries", type=int, default=MAX_RETRIES, metavar="N",
                        help=f"GitHub verification retry attempts (default: {MAX_RETRIES})")
    parser.add_argument("--version", action="version", version=f"%(prog)s {VERSION}")
    return parser.parse_args()

# ─────────────────────────────────────────────────────────────────────────────
# Input validation
# ─────────────────────────────────────────────────────────────────────────────

_EMAIL_RE   = re.compile(r"^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$")
_KEYNAME_RE = re.compile(r"^[a-zA-Z0-9_\-]{1,64}$")


def validate_email(email: str) -> str:
    email = email.strip()
    if not email:
        raise ValidationError("Email address cannot be empty.")
    if not _EMAIL_RE.match(email):
        raise ValidationError(
            f"'{email}' is not a valid email address.\n"
            "  Expected format: user@domain.tld"
        )
    return email


def validate_key_name(name: str) -> str:
    name = name.strip()
    if not _KEYNAME_RE.match(name):
        raise ValidationError(
            f"Key name '{name}' contains invalid characters.\n"
            "  Allowed: letters, digits, underscores, hyphens (max 64 chars)."
        )
    return name


def prompt_email() -> str:
    """Prompt for email interactively, allow up to 3 attempts."""
    for attempt in range(1, 4):
        try:
            raw = input(f"{CY}  Enter your GitHub email address:{R} ").strip()
            return validate_email(raw)
        except ValidationError as exc:
            error(str(exc))
            if attempt == 3:
                raise SetupError("Too many invalid email attempts.") from exc
    raise SetupError("Email prompt failed.")

# ─────────────────────────────────────────────────────────────────────────────
# Subprocess wrapper
# ─────────────────────────────────────────────────────────────────────────────

def run(
    cmd: list,
    *,
    capture: bool = False,
    check: bool = True,
    env: Optional[dict] = None,
    timeout: Optional[int] = None,
    input_text: Optional[str] = None,
    dry_run: bool = False,
) -> subprocess.CompletedProcess:
    """
    Safe, logged subprocess runner.

    - Never uses shell=True (prevents injection)
    - Converts all args to str (prevents TypeError on Path objects)
    - Raises typed exceptions instead of raw CalledProcessError
    - Logs every invocation at DEBUG level
    """
    safe_cmd = [str(a) for a in cmd]
    dbg(f"run: {' '.join(safe_cmd)}")

    if dry_run:
        info(f"  [dry-run] {' '.join(safe_cmd)}")
        return subprocess.CompletedProcess(safe_cmd, 0, stdout="", stderr="")

    merged_env = {**os.environ, **(env or {})}

    try:
        result = subprocess.run(
            safe_cmd,
            capture_output=capture,
            text=True,
            check=False,           # we handle exit codes manually for richer errors
            env=merged_env,
            timeout=timeout,
            input=input_text,
        )
    except FileNotFoundError:
        raise ToolMissingError(f"Command not found: {safe_cmd[0]!r}")
    except subprocess.TimeoutExpired:
        raise SetupError(f"Command timed out after {timeout}s: {' '.join(safe_cmd)}")
    except OSError as exc:
        raise SetupError(f"OS error executing {safe_cmd[0]!r}: {exc}") from exc

    dbg(f"  rc={result.returncode}  stdout={result.stdout!r:.120}  stderr={result.stderr!r:.120}")

    if check and result.returncode != 0:
        combined = (result.stderr or result.stdout or "").strip()
        raise SetupError(
            f"Command failed (exit {result.returncode}): {' '.join(safe_cmd)}\n"
            f"  Output: {combined}"
        )
    return result

# ─────────────────────────────────────────────────────────────────────────────
# Tool detection + installation
# ─────────────────────────────────────────────────────────────────────────────

def find_tool(name: str) -> Optional[str]:
    return shutil.which(name)


def require_tool(name: str, hint: str = "") -> str:
    path = find_tool(name)
    if not path:
        msg = f"Required tool '{name}' not found."
        if hint:
            msg += f"\n  {hint}"
        raise ToolMissingError(msg)
    return path


def install_package(*, linux: str, mac: str, win: str, dry_run: bool = False) -> None:
    """Best-effort package installation. Raises ToolMissingError if impossible."""
    if IS_WIN:
        if find_tool("winget"):
            run(["winget", "install", "--id", win, "-e", "--silent",
                 "--accept-package-agreements"], check=False, dry_run=dry_run)
        elif find_tool("choco"):
            run(["choco", "install", win, "-y"], check=False, dry_run=dry_run)
        else:
            raise ToolMissingError(
                f"Cannot install '{win}': neither winget nor choco found.\n"
                "  Install winget: https://aka.ms/winget-install"
            )
    elif IS_MAC:
        if find_tool("brew"):
            run(["brew", "install", mac], dry_run=dry_run)
        else:
            raise ToolMissingError(
                "Homebrew not found.\n"
                "  Install from: https://brew.sh"
            )
    else:
        managers = [
            (["apt-get"], ["sudo", "apt-get", "update", "-qq"],
                          ["sudo", "apt-get", "install", "-y", linux]),
            (["dnf"],     None, ["sudo", "dnf", "install", "-y", linux]),
            (["pacman"],  None, ["sudo", "pacman", "-Sy", "--noconfirm", linux]),
            (["zypper"],  None, ["sudo", "zypper", "install", "-y", linux]),
            (["apk"],     None, ["sudo", "apk", "add", "--no-cache", linux]),
        ]
        for tools, pre, install in managers:
            if find_tool(tools[0]):
                if pre:
                    run(pre, dry_run=dry_run)
                run(install, dry_run=dry_run)
                return
        raise ToolMissingError(
            f"No supported package manager found. Install '{linux}' manually."
        )

# ─────────────────────────────────────────────────────────────────────────────
# Filesystem helpers
# ─────────────────────────────────────────────────────────────────────────────

def ensure_ssh_dir(cfg: Config) -> None:
    if cfg.dry_run:
        info(f"  [dry-run] mkdir {cfg.ssh_dir} (mode 700)")
        return
    cfg.ssh_dir.mkdir(mode=0o700, exist_ok=True)
    if not IS_WIN:
        cfg.ssh_dir.chmod(0o700)


def safe_append(path: Path, content: str, mode: int = 0o600,
                dry_run: bool = False) -> None:
    """Append content to path (creating if needed) then set permissions."""
    if dry_run:
        info(f"  [dry-run] append to {path}")
        dbg(f"  content: {content!r}")
        return
    with path.open("a", encoding="utf-8") as fh:
        fh.write(content)
    if not IS_WIN:
        path.chmod(mode)


def backup_file(path: Path, dry_run: bool = False) -> Optional[Path]:
    """Create a timestamped .bak copy before modifying a file."""
    if not path.exists():
        return None
    ts = time.strftime("%Y%m%d_%H%M%S")
    backup = path.with_suffix(f".bak_{ts}")
    if dry_run:
        info(f"  [dry-run] backup {path} → {backup}")
        return backup
    shutil.copy2(path, backup)
    dbg(f"Backed up {path} → {backup}")
    return backup

# ─────────────────────────────────────────────────────────────────────────────
# Clipboard
# ─────────────────────────────────────────────────────────────────────────────

def copy_to_clipboard(text: str, dry_run: bool = False) -> bool:
    """Copy text to clipboard. Returns True on success. Never raises."""
    if dry_run:
        info("  [dry-run] copy public key to clipboard")
        return True
    try:
        if IS_WIN:
            proc = subprocess.Popen(["clip"], stdin=subprocess.PIPE)
            proc.communicate(input=text.encode("utf-16-le"))
            return proc.returncode == 0
        if IS_MAC:
            proc = subprocess.Popen(["pbcopy"], stdin=subprocess.PIPE)
            proc.communicate(input=text.encode("utf-8"))
            return proc.returncode == 0
        # Linux — try in preference order
        for cmd in [["xclip", "-selection", "clipboard"],
                    ["xsel", "--clipboard", "--input"],
                    ["wl-copy"]]:
            if find_tool(cmd[0]):
                proc = subprocess.Popen(cmd, stdin=subprocess.PIPE)
                proc.communicate(input=text.encode("utf-8"))
                return proc.returncode == 0
    except Exception as exc:
        dbg(f"Clipboard error: {exc}")
    return False

# ─────────────────────────────────────────────────────────────────────────────
# SSH agent helpers
# ─────────────────────────────────────────────────────────────────────────────

def _parse_agent_output(output: str) -> dict:
    env_vars: dict = {}
    for line in output.splitlines():
        m = re.match(r"^(SSH_AUTH_SOCK|SSH_AGENT_PID)=([^;]+);", line)
        if m:
            env_vars[m.group(1)] = m.group(2)
    return env_vars


def _agent_alive(env: dict) -> bool:
    """Return True if ssh-agent is reachable via env."""
    try:
        result = subprocess.run(
            ["ssh-add", "-l"], capture_output=True, text=True,
            env={**os.environ, **env}, timeout=5,
        )
        return result.returncode in (0, 1)  # 0=keys loaded, 1=no keys but alive
    except Exception:
        return False


def _key_fingerprint(key_path: Path) -> Optional[str]:
    try:
        r = subprocess.run(
            ["ssh-keygen", "-lf", str(key_path)],
            capture_output=True, text=True, timeout=5,
        )
        if r.returncode == 0:
            parts = r.stdout.strip().split()
            return parts[1] if len(parts) >= 2 else None
    except Exception:
        pass
    return None


def _key_in_agent(key_path: Path, agent_env: dict) -> bool:
    fp = _key_fingerprint(key_path)
    if not fp:
        return False
    try:
        r = subprocess.run(
            ["ssh-add", "-l"], capture_output=True, text=True,
            env={**os.environ, **agent_env}, timeout=5,
        )
        return fp in r.stdout
    except Exception:
        return False


def start_or_reuse_agent_unix(cfg: Config) -> dict:
    """
    Resolution order:
      1. SSH_AUTH_SOCK already set in this shell session
      2. Saved env in ~/.ssh/agent.env from a previous run
      3. Live agent socket found in /tmp
      4. Start a fresh ssh-agent process
    """
    # 1. Current session
    sock = os.environ.get("SSH_AUTH_SOCK", "")
    if sock and Path(sock).exists():
        env = {"SSH_AUTH_SOCK": sock}
        if _agent_alive(env):
            ok("Reusing ssh-agent from current shell session")
            return env

    # 2. Saved agent.env
    if cfg.agent_env.exists():
        try:
            saved: dict = {}
            for line in cfg.agent_env.read_text().splitlines():
                m = re.match(r'^export\s+(SSH_AUTH_SOCK|SSH_AGENT_PID)=(.+)$', line)
                if m:
                    saved[m.group(1)] = m.group(2).strip()
            if saved and _agent_alive(saved):
                ok(f"Reusing saved ssh-agent ({cfg.agent_env})")
                os.environ.update(saved)
                return saved
        except OSError:
            pass

    # 3. Scan /tmp for live sockets
    try:
        user = os.environ.get("USER") or os.environ.get("USERNAME") or ""
        for p in Path("/tmp").glob("ssh-*/agent.*"):
            try:
                if p.owner() == user:
                    env = {"SSH_AUTH_SOCK": str(p)}
                    if _agent_alive(env):
                        ok(f"Reusing existing agent socket: {p}")
                        os.environ["SSH_AUTH_SOCK"] = str(p)
                        return env
            except (PermissionError, KeyError):
                continue
    except OSError:
        pass

    # 4. Start fresh
    if cfg.dry_run:
        info("  [dry-run] would start new ssh-agent")
        return {"SSH_AUTH_SOCK": "/tmp/dry-run.sock", "SSH_AGENT_PID": "0"}

    result = run(["ssh-agent", "-s"], capture=True)
    agent_env = _parse_agent_output(result.stdout)

    if not agent_env.get("SSH_AUTH_SOCK"):
        raise AgentError("ssh-agent started but SSH_AUTH_SOCK not found in output.")

    os.environ.update(agent_env)
    ok(f"New ssh-agent started (PID: {agent_env.get('SSH_AGENT_PID', '?')})")

    # Persist for future terminal sessions
    try:
        content = "\n".join(f"export {k}={v}" for k, v in agent_env.items()) + "\n"
        cfg.agent_env.write_text(content, encoding="utf-8")
        if not IS_WIN:
            cfg.agent_env.chmod(0o600)
        info(f"Agent env saved → {cfg.agent_env}")
        info("To persist across terminals, add to ~/.bashrc or ~/.zshrc:")
        print(f"    {YE}[ -f ~/.ssh/agent.env ] && source ~/.ssh/agent.env{R}")
    except OSError as exc:
        warn(f"Could not save agent env: {exc}")

    return agent_env


def setup_agent_windows() -> dict:
    """
    Ensure the Windows OpenSSH Agent service is running.
    Also force Git to use the Windows system ssh.exe (not MSYS2's) to prevent
    git push passphrase prompts when agent credentials are cached.
    """
    result = run(
        ["powershell", "-NoProfile", "-Command",
         "(Get-Service -Name ssh-agent -ErrorAction SilentlyContinue).Status"],
        capture=True, check=False,
    )
    if "Running" not in result.stdout:
        warn("OpenSSH Agent service not running — attempting to start...")
        start = run(
            ["powershell", "-NoProfile", "-Command",
             "Set-Service -Name ssh-agent -StartupType Automatic;"
             " Start-Service ssh-agent"],
            capture=True, check=False,
        )
        if start.returncode != 0:
            raise AgentError(
                "Could not start OpenSSH Agent service.\n"
                "  Run PowerShell as Administrator:\n"
                "    Set-Service -Name ssh-agent -StartupType Automatic\n"
                "    Start-Service ssh-agent"
            )
        ok("OpenSSH Agent service started and set to auto-start")
    else:
        ok("OpenSSH Agent service already running")

    # Point Git at Windows system ssh.exe to avoid MSYS2 agent conflicts
    sys_ssh = (
        Path(os.environ.get("SystemRoot", r"C:\Windows"))
        / "System32" / "OpenSSH" / "ssh.exe"
    )
    if sys_ssh.exists() and find_tool("git"):
        run(["git", "config", "--global", "core.sshCommand", str(sys_ssh)],
            capture=True, check=False)
        dbg(f"Set git core.sshCommand = {sys_ssh}")

    return {}  # Windows agent uses service; no socket env vars needed

# ─────────────────────────────────────────────────────────────────────────────
# macOS Keychain integration
# ─────────────────────────────────────────────────────────────────────────────

def _apple_ssh_add_supported() -> bool:
    """Return True if /usr/bin/ssh-add supports --apple-use-keychain."""
    test = subprocess.run(
        ["/usr/bin/ssh-add", "--apple-use-keychain", "/dev/null"],
        capture_output=True, text=True,
    )
    return "illegal option" not in (test.stderr or "")


def add_key_macos_keychain(cfg: Config) -> bool:
    """
    Store key passphrase in macOS Keychain via Apple's ssh-add.
    Falls back gracefully if Apple's binary isn't present or doesn't support the flag.
    Returns True if Keychain add succeeded.
    """
    if not Path("/usr/bin/ssh-add").exists():
        return False
    if not _apple_ssh_add_supported():
        warn("Non-Apple ssh-add detected (likely Homebrew OpenSSH). Skipping Keychain.")
        return False
    if cfg.dry_run:
        info("  [dry-run] /usr/bin/ssh-add --apple-use-keychain <key>")
        return True
    result = run(
        ["/usr/bin/ssh-add", "--apple-use-keychain", str(cfg.key_path)],
        check=False,
    )
    if result.returncode == 0:
        ok("Passphrase stored in macOS Keychain ✓")
        return True
    warn("macOS Keychain storage failed — falling back to regular ssh-add")
    return False

# ─────────────────────────────────────────────────────────────────────────────
# known_hosts — fetch + fingerprint verification
# ─────────────────────────────────────────────────────────────────────────────

def github_in_known_hosts(cfg: Config) -> bool:
    if not cfg.known_hosts.exists():
        return False
    try:
        r = subprocess.run(
            ["ssh-keygen", "-F", GITHUB_HOST, "-f", str(cfg.known_hosts)],
            capture_output=True, text=True, timeout=5,
        )
        return r.returncode == 0 and GITHUB_HOST in r.stdout
    except Exception:
        return False


def fetch_and_verify_github_fingerprint(cfg: Config) -> None:
    """
    Fetch GitHub host keys via ssh-keyscan, verify at least one fingerprint
    against GitHub's published list, then append to known_hosts.
    Warns (does not abort) if fingerprint cannot be verified — some corporate
    proxies intercept keyscan traffic.
    """
    if cfg.dry_run:
        info("  [dry-run] ssh-keyscan + fingerprint verification")
        return

    info("Fetching GitHub host fingerprints (ssh-keyscan)...")
    result = run(
        ["ssh-keyscan", "-H", "-T", "10", GITHUB_HOST],
        capture=True, check=False,
    )

    if not result.stdout.strip():
        raise SetupError(
            f"ssh-keyscan returned no output for {GITHUB_HOST}.\n"
            "  Check network/DNS connectivity and try again."
        )

    # Verify fingerprint against published values using a temp file
    import tempfile
    with tempfile.NamedTemporaryFile(
        mode="w", suffix=".known_hosts", delete=False, encoding="utf-8"
    ) as tmp:
        tmp.write(result.stdout)
        tmp_path = Path(tmp.name)

    verified = False
    try:
        fp_result = run(
            ["ssh-keygen", "-l", "-f", str(tmp_path)],
            capture=True, check=False,
        )
        combined = fp_result.stdout + fp_result.stderr
        for fp in GITHUB_FINGERPRINTS.values():
            if fp in combined:
                verified = True
                dbg(f"Verified fingerprint: {fp}")
                break
    finally:
        try:
            tmp_path.unlink()
        except OSError:
            pass

    if not verified:
        warn(
            "Could not verify GitHub fingerprint against known published values.\n"
            "  This may indicate a network proxy/MITM or an outdated fingerprint\n"
            "  list in this script. Proceeding — verify manually:\n"
            "  https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints"
        )
    else:
        ok("GitHub host fingerprint verified ✓")

    safe_append(cfg.known_hosts, result.stdout, mode=0o644)

# ─────────────────────────────────────────────────────────────────────────────
# SSH config writer
# ─────────────────────────────────────────────────────────────────────────────

def write_ssh_config(cfg: Config) -> None:
    """
    Append a GitHub Host block to ~/.ssh/config.
    - Backs up existing config before touching it
    - macOS: adds UseKeychain yes if Apple ssh-add is present
    - Skips silently if block already exists
    """
    existing = cfg.ssh_config.read_text(encoding="utf-8") if cfg.ssh_config.exists() else ""

    if "Host github.com" in existing:
        warn("~/.ssh/config already has a 'Host github.com' block — skipping.")
        warn(f"  Review {cfg.ssh_config} if you encounter connection issues.")
        return

    if cfg.ssh_config.exists():
        bk = backup_file(cfg.ssh_config, dry_run=cfg.dry_run)
        if bk:
            info(f"Config backed up → {bk}")

    # macOS Keychain line — only if Apple's ssh-add supports the flag
    keychain_line = ""
    if IS_MAC and _apple_ssh_add_supported():
        keychain_line = "\n    UseKeychain yes"

    block = textwrap.dedent(f"""
        # ── GitHub — added by github_ssh_setup.py v{VERSION} ──
        Host github.com
            HostName github.com
            User git
            IdentityFile {cfg.key_path}
            IdentitiesOnly yes
            AddKeysToAgent yes{keychain_line}
    """)

    safe_append(cfg.ssh_config, block, mode=0o600, dry_run=cfg.dry_run)
    ok("~/.ssh/config updated with GitHub block")
    if keychain_line:
        info("  UseKeychain yes enabled (macOS Keychain integration)")

# ─────────────────────────────────────────────────────────────────────────────
# GitHub connection test
# ─────────────────────────────────────────────────────────────────────────────

def test_github_connection(agent_env: dict) -> Optional[str]:
    """
    Run 'ssh -T git@github.com'. Returns GitHub username on success, None on failure.
    Uses BatchMode=yes to prevent interactive passphrase prompts during the test.
    Never raises.
    """
    try:
        result = subprocess.run(
            [
                "ssh", "-T",
                "-o", "BatchMode=yes",
                "-o", f"ConnectTimeout={SSH_TIMEOUT}",
                "-o", "StrictHostKeyChecking=yes",   # never prompt; use known_hosts
                f"git@{GITHUB_HOST}",
            ],
            capture_output=True, text=True,
            timeout=SSH_TIMEOUT + 5,
            env={**os.environ, **agent_env},
        )
        output = (result.stdout + result.stderr).strip()
        dbg(f"ssh -T rc={result.returncode} output={output!r}")

        if "successfully authenticated" in output:
            m = re.search(r"Hi ([^!]+)!", output)
            return m.group(1) if m else "unknown"
        return None
    except Exception as exc:
        dbg(f"Connection test exception: {exc}")
        return None

# ─────────────────────────────────────────────────────────────────────────────
# Environment guard
# ─────────────────────────────────────────────────────────────────────────────

def check_python_version() -> None:
    if sys.version_info < MIN_PYTHON:
        raise SetupError(
            f"Python {'.'.join(str(x) for x in MIN_PYTHON)}+ required. "
            f"You have {platform.python_version()}.\n"
            "  Download: https://www.python.org/downloads/"
        )

# ─────────────────────────────────────────────────────────────────────────────
# Main setup flow
# ─────────────────────────────────────────────────────────────────────────────

TOTAL_STEPS = 8


def run_setup(cfg: Config) -> None:

    if cfg.dry_run:
        box(["DRY RUN MODE — no changes will be made"], color=YE)

    # ── [1/8] Environment ────────────────────────────────────────────────────
    step(1, TOTAL_STEPS, "Checking environment")
    check_python_version()
    ok(f"Python {platform.python_version()}")
    ok(f"OS: {SYSTEM} {RELEASE}")
    info(f"Hostname : {HOSTNAME}")
    info(f"Email    : {cfg.email}")
    info(f"Key path : {cfg.key_path}")
    info(f"Log file : {LOG_FILE}")

    # ── [2/8] OpenSSH ────────────────────────────────────────────────────────
    step(2, TOTAL_STEPS, "Checking OpenSSH")
    if not find_tool("ssh"):
        warn("OpenSSH not found — attempting installation...")
        install_package(linux="openssh-client", mac="openssh",
                        win="Microsoft.OpenSSH.Beta", dry_run=cfg.dry_run)
    require_tool(
        "ssh",
        "Install OpenSSH:\n"
        "  Windows: winget install Microsoft.OpenSSH.Beta\n"
        "  macOS  : brew install openssh\n"
        "  Linux  : sudo apt install openssh-client",
    )
    ver_result = run(["ssh", "-V"], capture=True, check=False)
    version_str = (ver_result.stderr or ver_result.stdout).strip()
    ok(f"OpenSSH: {version_str}")

    # Enforce minimum version — ed25519 needs 6.5+
    m = re.search(r"OpenSSH_(\d+)\.(\d+)", version_str)
    if m and (int(m.group(1)), int(m.group(2))) < (6, 5):
        raise SetupError(
            f"OpenSSH {m.group(1)}.{m.group(2)} is too old for ed25519 (needs 6.5+).\n"
            "  Please update OpenSSH."
        )

    # ── [3/8] Git ─────────────────────────────────────────────────────────────
    step(3, TOTAL_STEPS, "Checking Git")
    if not find_tool("git"):
        warn("Git not found — attempting installation...")
        install_package(linux="git", mac="git", win="Git.Git", dry_run=cfg.dry_run)
    require_tool("git", "Install from: https://git-scm.com/downloads")
    git_ver = run(["git", "--version"], capture=True)
    ok(git_ver.stdout.strip())

    # ── [4/8] ~/.ssh directory ───────────────────────────────────────────────
    step(4, TOTAL_STEPS, "Preparing ~/.ssh directory")
    ensure_ssh_dir(cfg)
    ok(f"~/.ssh ready{'' if IS_WIN else ' (mode 700)'}")

    # ── [5/8] Generate key ───────────────────────────────────────────────────
    step(5, TOTAL_STEPS, "Generating SSH key (ed25519)")

    if cfg.key_path.exists() and not cfg.force:
        warn(f"Key already exists: {cfg.key_path}")
        warn("Pass --force to regenerate (this will overwrite the existing key).")
        info("Skipping key generation — using existing key.")
    else:
        if cfg.key_path.exists() and cfg.force:
            bk = backup_file(cfg.key_path, dry_run=cfg.dry_run)
            if bk:
                ok(f"Existing key backed up → {bk}")
            backup_file(cfg.pub_path, dry_run=cfg.dry_run)

        _print()
        _print("  A passphrase protects your key if your disk is ever compromised.")
        _print("  Press ENTER twice to skip (not recommended on shared machines).")
        _print()

        run([
            "ssh-keygen",
            "-t", "ed25519",
            "-a", "100",     # 100 KDF rounds — significantly stronger passphrase
            "-C", f"{cfg.email} ({time.strftime('%Y-%m')})",
            "-f", str(cfg.key_path),
        ], dry_run=cfg.dry_run)

        if not cfg.dry_run:
            if not IS_WIN:
                cfg.key_path.chmod(0o600)
                cfg.pub_path.chmod(0o644)
            ok(f"Key generated: {cfg.key_path}")
            fp = _key_fingerprint(cfg.key_path)
            if fp:
                info(f"Fingerprint : {fp}")

    # ── [6/8] ssh-agent + load key ───────────────────────────────────────────
    step(6, TOTAL_STEPS, "Starting ssh-agent & loading key")
    agent_env: dict = {}

    if IS_WIN:
        agent_env = setup_agent_windows()
    else:
        agent_env = start_or_reuse_agent_unix(cfg)

    full_env = {**os.environ, **agent_env}

    if _key_in_agent(cfg.key_path, agent_env):
        ok("Key already loaded in agent")
    else:
        if cfg.dry_run:
            info(f"  [dry-run] ssh-add {cfg.key_path}")
        elif IS_MAC:
            if not add_key_macos_keychain(cfg):
                run(["ssh-add", str(cfg.key_path)], env=full_env)
                ok("Key added to agent")
        else:
            run(["ssh-add", str(cfg.key_path)], env=full_env)
            ok("Key added to agent")

    # ── [7/8] known_hosts + ssh config ───────────────────────────────────────
    step(7, TOTAL_STEPS, "Configuring SSH for GitHub")

    if github_in_known_hosts(cfg):
        ok("GitHub already in known_hosts — skipping keyscan")
    else:
        fetch_and_verify_github_fingerprint(cfg)
        if not cfg.dry_run:
            ok(f"GitHub fingerprint saved → {cfg.known_hosts}")

    write_ssh_config(cfg)

    # ── [8/8] Show key, pause, verify ────────────────────────────────────────
    step(8, TOTAL_STEPS, "Register key on GitHub")

    if cfg.dry_run:
        info("  [dry-run] would display public key and wait for confirmation")
        _print()
        ok("Dry-run complete — no changes made.")
        return

    pub_key = cfg.pub_path.read_text(encoding="utf-8").strip()
    _print()
    _print(f"{B}Your public SSH key:{R}")
    divider()
    _print(pub_key)
    divider()
    _print()

    copied = copy_to_clipboard(pub_key)
    if copied:
        ok("Public key copied to clipboard ✓")
    else:
        warn("No clipboard tool found — copy the key above manually")

    _print()
    box([
        "ACTION REQUIRED — Add Key to GitHub",
        "",
        "  1.  Open: https://github.com/settings/ssh/new",
        f"  2.  Title    → {HOSTNAME}  (or any label you prefer)",
        "  3.  Key type → Authentication Key",
        f"  4.  {'Paste key — already in your clipboard ✓' if copied else 'Paste the key shown above'}",
        "  5.  Click  [Add SSH key]",
    ], color=CY)
    _print()

    # Retry loop
    for attempt in range(1, cfg.retries + 1):
        try:
            input(f"{B}  Press ENTER once you've added the key to GitHub...{R} ")
        except (EOFError, KeyboardInterrupt):
            _print()
            raise SetupError("Interrupted during key registration.")

        _print()
        info(f"Testing connection (attempt {attempt}/{cfg.retries})...")

        if attempt > 1:
            time.sleep(RETRY_DELAY)  # give GitHub a moment to propagate

        github_user = test_github_connection(agent_env)

        if github_user:
            _print()
            box([
                "  ✓  Connection Verified!",
                "",
                f"  Authenticated as: {github_user}",
            ], color=GR)
            _print()
            ok(f"GitHub user: {B}{github_user}{R}")
            _print()
            break
        else:
            if attempt >= cfg.retries:
                _print()
                error(f"Could not verify after {cfg.retries} attempts.")
                _print()
                print(f"{B}Troubleshooting checklist:{R}")
                print(f"  {YE}•{R} Confirm you pasted the {B}public{R} key (ends with your email)")
                print(f"  {YE}•{R} Visit https://github.com/settings/keys to confirm it saved")
                print(f"  {YE}•{R} Verbose debug: {CY}ssh -vvvT git@github.com{R}")
                print(f"  {YE}•{R} Agent keys:    {CY}ssh-add -l{R}")
                print(f"  {YE}•{R} Config check:  {CY}cat ~/.ssh/config{R}")
                print(f"  {YE}•{R} Setup log:     {CY}cat {LOG_FILE}{R}")
                _print()
                raise GithubConnectionError(
                    "GitHub SSH verification failed. See troubleshooting above."
                )
            warn("Not authenticated yet.")
            print(f"  → Make sure you clicked {B}Add SSH key{R} on GitHub, then try again.")
            _print()

    # ── Usage summary ─────────────────────────────────────────────────────────
    divider("═")
    print(f"{B}  You're all set! Quick reference:{R}")
    divider("═")
    _print()
    print(f"  {CY}Clone a private repo:{R}")
    print(f"    {YE}git clone git@github.com:USERNAME/REPO.git{R}")
    _print()
    print(f"  {CY}Switch HTTPS remote → SSH:{R}")
    print(f"    {YE}git remote set-url origin git@github.com:USERNAME/REPO.git{R}")
    _print()
    print(f"  {CY}Check current remote:{R}")
    print(f"    {YE}git remote -v{R}")
    _print()
    print(f"  {CY}Re-test connection:{R}")
    print(f"    {YE}ssh -T git@github.com{R}")
    _print()
    print(f"  {DIM}Key fingerprint : {_key_fingerprint(cfg.key_path) or 'n/a'}{R}")
    print(f"  {DIM}Public key      : {cfg.pub_path}{R}")
    print(f"  {DIM}Setup log       : {LOG_FILE}{R}")
    _print()
    ok("All done. Happy coding!")
    _print()

# ─────────────────────────────────────────────────────────────────────────────
# Entry point
# ─────────────────────────────────────────────────────────────────────────────

def main() -> None:
    global log

    args = parse_args()
    log = _setup_logging(args.verbose)

    # Banner
    _print()
    box([
        f"  GitHub SSH Setup  •  v{VERSION}  •  Python {platform.python_version()}",
        f"  Platform: {SYSTEM} {RELEASE}",
    ], color=B)
    _print()

    # Validate inputs
    try:
        email    = validate_email(args.email) if args.email else prompt_email()
        key_name = validate_key_name(args.key_name)
    except (ValidationError, SetupError) as exc:
        error(str(exc))
        sys.exit(3)

    cfg = Config(
        email    = email,
        key_name = key_name,
        dry_run  = args.dry_run,
        force    = args.force,
        verbose  = args.verbose,
        retries  = args.retries,
    )
    log.debug("Config: %s", cfg)

    # Run with typed exit codes per exception class
    try:
        run_setup(cfg)
    except KeyboardInterrupt:
        _print()
        warn("Interrupted by user (Ctrl+C).")
        sys.exit(130)
    except ToolMissingError as exc:
        error(str(exc))
        log.exception("ToolMissingError")
        sys.exit(2)
    except ValidationError as exc:
        error(str(exc))
        log.exception("ValidationError")
        sys.exit(3)
    except AgentError as exc:
        error(str(exc))
        log.exception("AgentError")
        sys.exit(4)
    except GithubConnectionError as exc:
        error(str(exc))
        log.exception("GithubConnectionError")
        sys.exit(5)
    except SetupError as exc:
        error(str(exc))
        log.exception("SetupError")
        sys.exit(1)
    except Exception as exc:
        error(f"Unexpected error: {exc}")
        log.exception("Unhandled exception")
        if args.verbose:
            import traceback
            traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
