#!/usr/bin/env python3
"""End-to-end test: drive a real interactive fish through a PTY.

Usage: python3 scripts/e2e_fish.py path/to/apura
Exercises the actual hooks (fish_preexec/fish_postexec), the env wrapper,
background recording, notes, marks, show and distill.
"""
import os, pty, re, select, sys, tempfile, time, subprocess

apura = os.path.abspath(sys.argv[1] if len(sys.argv) > 1 else "bin/apura")
home = tempfile.mkdtemp(prefix="apura-e2e-")
os.makedirs(f"{home}/.config/fish", exist_ok=True)
os.makedirs(f"{home}/proj", exist_ok=True)
with open(f"{home}/.config/fish/config.fish", "w") as f:
    f.write(f"status is-interactive; and {apura} init fish | source\n")
    f.write("function fish_prompt; echo -n 'PROMPT> '; end\n")
    f.write("set -g fish_greeting ''\n")
    f.write("set -g fish_autosuggestion_enabled 0\n")
    # keep the screen predictable: no right prompt (e.g. conda's "(base)"), no vi mode marker
    f.write("function fish_right_prompt; end\n")
    f.write("function fish_mode_prompt; end\n")
env = dict(os.environ, HOME=home, XDG_CONFIG_HOME=f"{home}/.config",
           XDG_DATA_HOME=f"{home}/.local/share",
           TERM=os.environ.get("E2E_TERM", "dumb"))
env.pop("APURA_SESSION", None)

pid, fd = pty.fork()
if pid == 0:
    os.chdir(home)
    os.execvpe("fish", ["fish", "-i"], env)

# Terminal control sequences fish may emit around the prompt (CSI, OSC, DCS...).
# fish 4.x (especially on macOS) adds e.g. bracketed-paste toggles, OSC 133
# prompt marks and capability queries; they are invisible but break matching.
ANSI = re.compile(
    rb"\x1b\[[0-?]*[ -/]*[@-~]"            # CSI ... final byte
    rb"|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)"  # OSC ... BEL or ST
    rb"|\x1b[P^_X][^\x1b]*\x1b\\"           # DCS/PM/APC/SOS ... ST
    rb"|\x1b[@-Z\\-_=>]"                     # 2-byte escapes
)

# Answers to queries a real terminal would reply to; fish may wait for them.
REPLIES = {
    b"\x1b[c": b"\x1b[?62;22c",    # primary device attributes
    b"\x1b[0c": b"\x1b[?62;22c",
    b"\x1b[6n": b"\x1b[1;1R",      # cursor position
    b"\x1b[?u": b"\x1b[?0u",       # kitty keyboard protocol flags
}


def visible(raw: bytes) -> str:
    return ANSI.sub(b"", raw).replace(b"\r", b"").decode(errors="replace")


buf = b""
def read_until(token="PROMPT> ", timeout=20):
    global buf
    end = time.time() + timeout
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.4)
        if r:
            try:
                chunk = os.read(fd, 4096)
            except OSError:
                break
            buf += chunk
            for query, answer in REPLIES.items():
                if query in chunk:
                    os.write(fd, answer)
            continue
        # quiet for a moment and sitting at a prompt => command finished
        if visible(buf).rstrip().endswith(token.rstrip()):
            out, buf = buf, b""
            return visible(out)
    raise SystemExit(
        "timeout waiting for prompt.\n"
        f"visible output:\n{visible(buf)}\n"
        f"raw tail (please report if this fails): {buf[-300:]!r}"
    )

def sh(cmd, pause=0.15):
    os.write(fd, cmd.encode() + b"\r")
    out = read_until()
    time.sleep(pause)  # let the background _record finish
    return out

read_until()
sh("apura start e2e-demo -g 'prove fish works' -t demo")
sh("echo $APURA_SESSION_NAME")
sh("cd proj")
sh("false")
sh("apura note 'this one fails on purpose'")
sh("touch made.txt")
sh("touch made.txt")
sh("ls")
sh(" echo private")
sh("set -gx MY_TOKEN abc123")
sh("apura mark skip")
show = sh("apura show")
distilled = sh("apura distill --format sh")
sh("apura end -r success")
after = sh('echo "[$APURA_SESSION]"')
os.write(fd, b"exit\r")
time.sleep(0.3)

db = f"{home}/.local/share/apura/apura.db"
rows = subprocess.run(["sqlite3", db, "select seq, exit_code, mark, cwd, command from steps order by seq"],
                      capture_output=True, text=True).stdout if subprocess.run(["which","sqlite3"],capture_output=True).returncode==0 else ""
print("---- apura show ----\n" + show)
print("---- distill ----\n" + distilled)

checks = {
    "start set APURA_SESSION_NAME in the shell": "e2e-demo" in show,
    "failed step recorded with exit 1": "false" in show and "  1" in show,
    "note attached": "this one fails on purpose" in show,
    "private (leading space) command skipped": "private" not in show,
    "secret redacted": "abc123" not in show and "MY_TOKEN ***" in show,
    "apura's own commands not recorded": "apura note" not in show,
    "cwd captured at preexec (cd recorded in home)": ("cd " + home + "/proj") in distilled or "cd ~/proj" in distilled,
    "distill dropped failure, noise, repeat, skip": distilled.count("touch made.txt") == 1 and "\nfalse" not in distilled and "MY_TOKEN" not in distilled,
    "end unset the variable": "[]" in after,
}
ok = all(checks.values())
for k, v in checks.items():
    print(("PASS " if v else "FAIL ") + k)
sys.exit(0 if ok else 1)
