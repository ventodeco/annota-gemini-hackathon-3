# Redacted secret-pattern scan over every blob reachable from all refs.
# Prints only rule, path, and a fingerprint-free classification; never values.
import re, subprocess, sys, collections
repo = sys.argv[1]
def run(*a, inp=None):
    return subprocess.run(["git", "-C", repo, *a], input=inp, capture_output=True, check=True).stdout
objs = run("rev-list", "--all", "--objects").decode().splitlines()
paths = {}
for line in objs:
    parts = line.split(" ", 1)
    if len(parts) == 2:
        paths.setdefault(parts[0], set()).add(parts[1])
batch = run("cat-file", "--batch-check=%(objectname) %(objecttype) %(objectsize)", inp="\n".join(paths).encode()).decode().splitlines()
blobs = [l.split()[0] for l in batch if l.split()[1] == "blob" and int(l.split()[2]) < 2_000_000]
RULES = {
  "google-api-key": re.compile(rb"AIza[0-9A-Za-z_\-]{35}"),
  "google-oauth-client-secret": re.compile(rb"GOCSPX-[0-9A-Za-z_\-]{20,}"),
  "openai/anthropic-style-key": re.compile(rb"\bsk-(?:ant-|proj-)?[0-9A-Za-z_\-]{20,}"),
  "openrouter-key": re.compile(rb"sk-or-v1-[0-9a-f]{20,}"),
  "clerk-secret": re.compile(rb"sk_(?:live|test)_[0-9A-Za-z]{20,}"),
  "aws-access-key": re.compile(rb"AKIA[0-9A-Z]{16}"),
  "private-key-block": re.compile(rb"-----BEGIN [A-Z ]*PRIVATE KEY-----"),
  "github-token": re.compile(rb"gh[pousr]_[0-9A-Za-z]{30,}"),
}
ASSIGN = re.compile(rb"(?im)^\s*(?:export\s+)?([A-Z0-9_]*(?:SECRET|PASSWORD|API_KEY|TOKEN|CLIENT_SECRET)[A-Z0-9_]*)\s*[:=]\s*['\"]?([^\s'\"#]*)")
PLACEHOLDER = re.compile(rb"(?i)^$|your|change|example|placeholder|xxx|<|\$\{|dummy|test|local|secret$|password$|annota$|postgres$|redacted|replace|sample|fake|todo|dev")
hits = collections.Counter(); assigns = collections.Counter()
for start in range(0, len(blobs), 500):
    chunk = blobs[start:start+500]
    out = run("cat-file", "--batch", inp="\n".join(chunk).encode())
    i = 0
    while i < len(out):
        nl = out.index(b"\n", i); sha, _, size = out[i:nl].split(); size = int(size)
        data = out[nl+1:nl+1+size]; i = nl + 1 + size + 1
        p = sorted(paths[sha.decode()])[0]
        for name, rx in RULES.items():
            for m in rx.finditer(data):
                hits[(name, p)] += 1
        for m in ASSIGN.finditer(data):
            val = m.group(2)
            if len(val) >= 12 and not PLACEHOLDER.search(val):
                assigns[(m.group(1).decode(), p, f"len={len(val)}")] += 1
print(f"scanned blobs={len(blobs)} refs=all (heads+PR heads)")
print("== high-confidence provider key patterns (rule, path, occurrences)")
for k, v in sorted(hits.items()): print(" ", k[0], k[1], v)
print("== non-placeholder-looking secret assignments, >=12 chars (var, path, value length only)")
for k, v in sorted(assigns.items()): print(" ", *k, v)
