#!/usr/bin/env python3
"""Pins the Coolify raw-Compose webhook provenance contract.

Coolify's raw Docker Compose deployments do not reliably hand SOURCE_COMMIT to
the build, and a checkout without .git leaves the image with nothing to bake in
— the dashboard then honestly reports "unknown". The signed relay closes that
gap: it reads the `after` SHA out of a verified GitHub push and writes exactly
one environment entry, FLAGIT_COMMIT, under the flagit service of the generated
Compose file before Coolify restarts the stack.

This script is the executable statement of that contract. It builds a minimal
production-like Compose fixture in a temporary directory with no .git anywhere,
applies the same narrow stamping semantics the relay must use, and asserts the
properties the deployment depends on:

  * a value that is not a 40-hex object name is rejected, and the file on disk
    is left byte-identical — the relay fails closed rather than deploying a
    guess or letting a ref name reach the container environment;
  * a malformed or ambiguous Compose file is rejected the same way;
  * a successful stamp changes exactly one line, inside the flagit service's
    environment mapping and nowhere else;
  * restamping the same SHA is a no-op, and restamping a new one replaces the
    single line rather than accumulating entries;
  * the tailscale-admin service, the named volumes and the port declarations
    survive untouched — the relay must never widen the exposure boundary;
  * with the relay's SHA in place, version resolution cannot land on "unknown".

Standard library only, no network, no secrets, and nothing outside its own
temporary directory. Run it directly:

    ./scripts/test-relay-commit-stamp.py
"""

from __future__ import annotations

import os
import re
import shutil
import sys
import tempfile

# The relay only ever writes this one key, and only under this one service.
SERVICE = "flagit"
KEY = "FLAGIT_COMMIT"

# GitHub sends the head of the pushed ref as a full lowercase 40-hex object
# name. Nothing shorter, nothing abbreviated, no ref names: the value is about
# to become a container environment variable that the dashboard presents as
# provenance, so anything that is not unmistakably an object name is refused.
SHA_RE = re.compile(r"\A[0-9a-f]{40}\Z")

# A push that deletes a branch reports the null SHA as `after`. That is not a
# deploy and must never be stamped.
NULL_SHA = "0" * 40

VERSION_UNKNOWN = "unknown"  # internal/version.Unknown


class Rejected(Exception):
    """The relay refused to stamp. Nothing was written."""


def is_relay_sha(value: str) -> bool:
    """Reports whether value is a SHA the relay is allowed to stamp."""
    return bool(SHA_RE.fullmatch(value)) and value != NULL_SHA


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def _is_structural(line: str) -> bool:
    """A line that carries indentation meaning — not blank, not a comment."""
    stripped = line.strip()
    return bool(stripped) and not stripped.startswith("#")


def stamp_commit(text: str, sha: str) -> str:
    """Returns text with FLAGIT_COMMIT set to sha under the flagit service.

    Deliberately a line-oriented edit rather than a YAML round-trip: the file
    belongs to Coolify, and re-emitting it would reformat and reorder parts the
    relay has no business touching. Every failure raises Rejected, so a caller
    that writes only on success cannot half-apply a change.
    """
    if not is_relay_sha(sha):
        raise Rejected(f"not a 40-hex commit object name: {sha!r}")

    lines = text.splitlines()
    if any("\t" in line for line in lines):
        raise Rejected("tab indentation: block structure is not reliably readable")

    services_at = [i for i, line in enumerate(lines) if line.rstrip() == "services:"]
    if len(services_at) != 1:
        raise Rejected(f"expected exactly one top-level services: block, found {len(services_at)}")
    start = services_at[0] + 1

    # The indent of the first entry under services: defines what a service is.
    entry_indent = None
    for line in lines[start:]:
        if _is_structural(line):
            entry_indent = _indent(line)
            break
    if not entry_indent:
        raise Rejected("services: block is empty")

    # Service headers only — a nested `flagit:` (a depends_on entry, a volume
    # name) sits deeper and must not be mistaken for the service itself.
    headers = [
        i
        for i in range(start, len(lines))
        if _is_structural(lines[i])
        and _indent(lines[i]) == entry_indent
        and lines[i].strip().rstrip(":") == SERVICE
        and lines[i].strip().endswith(":")
    ]
    if len(headers) != 1:
        raise Rejected(f"expected exactly one {SERVICE} service, found {len(headers)}")
    head = headers[0]

    # The service ends where the next line at or above the service indent starts.
    end = len(lines)
    for i in range(head + 1, len(lines)):
        if _is_structural(lines[i]) and _indent(lines[i]) <= entry_indent:
            end = i
            break

    env_at = [
        i
        for i in range(head + 1, end)
        if _is_structural(lines[i])
        and _indent(lines[i]) == entry_indent * 2
        and lines[i].strip() == "environment:"
    ]
    if len(env_at) != 1:
        raise Rejected(f"expected exactly one environment: mapping under {SERVICE}")
    env = env_at[0]

    # Where the environment mapping ends, and where its entries live.
    env_end = end
    for i in range(env + 1, end):
        if _is_structural(lines[i]) and _indent(lines[i]) <= entry_indent * 2:
            env_end = i
            break

    body = [i for i in range(env + 1, env_end) if _is_structural(lines[i])]
    if not body:
        raise Rejected(f"{SERVICE} environment: mapping is empty")
    if any(lines[i].lstrip().startswith("- ") for i in body):
        raise Rejected("environment: is a list, not a mapping — refusing to rewrite its form")

    value_indent = _indent(lines[body[0]])
    entry = f"{' ' * value_indent}{KEY}: {sha}"

    existing = [i for i in body if lines[i].split(":", 1)[0].strip() == KEY]
    if len(existing) > 1:
        raise Rejected(f"{KEY} appears {len(existing)} times under {SERVICE}")

    out = list(lines)
    if existing:
        out[existing[0]] = entry  # idempotent: same SHA rewrites the same bytes
    else:
        out.insert(body[-1] + 1, entry)

    trailing = "\n" if text.endswith("\n") else ""
    return "\n".join(out) + trailing


def stamp_file(path: str, sha: str) -> None:
    """Stamps a Compose file in place, writing only when the edit succeeded."""
    with open(path, encoding="utf-8") as fh:
        original = fh.read()
    updated = stamp_commit(original, sha)  # raises before anything is written
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(updated)


def flagit_commit_env(text: str) -> str:
    """Reads back what the container would receive as FLAGIT_COMMIT."""
    for line in text.splitlines():
        head, _, value = line.partition(":")
        if head.strip() == KEY:
            return value.strip()
    return ""


def resolve_version(override: str, baked: str = "") -> str:
    """Mirrors internal/version.Resolve: the runtime override wins, else baked."""
    if override.strip():
        return override.strip()
    if baked.strip():
        return baked.strip()
    return VERSION_UNKNOWN


# A trimmed stand-in for the Compose file Coolify generates for this stack:
# no published host ports, the admin dashboard reachable only through the
# Tailscale sidecar, both named volumes present. Secrets appear only as
# interpolation references — no values live here.
FIXTURE = """\
services:
  flagit:
    build:
      context: .
    restart: unless-stopped
    expose:
      - "8080"
      - "3000"
    volumes:
      - flagit_data:/data
    environment:
      # Externally reachable base URL, used to build the ticket link.
      FLAGIT_PUBLIC_URL: https://flagit.example.test
      FLAGIT_ADMIN_KEY: ${FLAGIT_ADMIN_KEY}

  tailscale-admin:
    image: tailscale/tailscale:latest
    restart: unless-stopped
    hostname: flagit-admin
    environment:
      TS_AUTHKEY: ${TS_AUTHKEY}
      TS_HOSTNAME: flagit-admin
      TS_STATE_DIR: /var/lib/tailscale
      TS_SERVE_CONFIG: /config/serve.json
      TS_USERSPACE: "true"
      TS_ACCEPT_DNS: "false"
    volumes:
      - tailscale_state:/var/lib/tailscale
      - ./deploy/tailscale:/config:ro
    depends_on:
      flagit:
        condition: service_healthy

volumes:
  flagit_data:
  tailscale_state:
"""

# The same stack with the services declared the other way round. A relay that
# stamps the first environment: mapping it finds passes the fixture above and
# poisons the tailnet sidecar here.
FIXTURE_REORDERED = "\n\n".join(
    reversed(FIXTURE.split("volumes:\n  flagit_data:")[0].rstrip().split("\n\n"))
)
FIXTURE_REORDERED = (
    "services:\n"
    + FIXTURE_REORDERED.replace("services:\n", "")
    + "\n\nvolumes:\n  flagit_data:\n  tailscale_state:\n"
)

SHA_A = "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678"
SHA_B = "fedcba98765432100123456789abcdef01234567"

TAILSCALE_BLOCK = FIXTURE[FIXTURE.index("  tailscale-admin:") : FIXTURE.index("volumes:\n  flagit")]


class Runner:
    def __init__(self) -> None:
        self.passed = 0
        self.failed = 0

    def check(self, name: str, ok: bool, detail: str = "") -> None:
        if ok:
            self.passed += 1
            print(f"PASS  {name}")
        else:
            self.failed += 1
            print(f"FAIL  {name}" + (f"\n        {detail}" if detail else ""))


def main() -> int:
    run = Runner()
    workdir = tempfile.mkdtemp(prefix="flagit-relay-contract-")
    try:
        compose = os.path.join(workdir, "docker-compose.yml")
        with open(compose, "w", encoding="utf-8") as fh:
            fh.write(FIXTURE)

        # --- the premise: nothing here can answer the provenance question ----
        run.check(
            "fixture has no .git anywhere, so the image could only report unknown",
            not any(".git" in names or ".git" in dirs for _, dirs, names in os.walk(workdir)),
        )
        run.check(
            "without the relay the deployment resolves to unknown",
            resolve_version(flagit_commit_env(FIXTURE), baked="") == VERSION_UNKNOWN,
        )

        # --- fail closed on anything that is not a 40-hex object name --------
        bad_values = {
            "empty": "",
            "unknown sentinel": VERSION_UNKNOWN,
            "abbreviated sha": "a1b2c3d",
            "39 hex": "a" * 39,
            "41 hex": "a" * 41,
            "non-hex character": "g" + "a" * 39,
            "uppercase": SHA_A.upper(),
            "branch name": "refs/heads/main",
            "uninterpolated variable": "${SOURCE_COMMIT}",
            "shell fragment": "$(git rev-parse HEAD)",
            "null sha of a branch deletion": NULL_SHA,
            "sha with trailing whitespace": SHA_A + " ",
            "yaml injection via newline": SHA_A + "\n      TS_AUTHKEY: stolen",
            "quote injection": f'"{SHA_A}" # ',
        }
        before = open(compose, encoding="utf-8").read()
        for label, value in bad_values.items():
            try:
                stamp_file(compose, value)
                rejected = False
            except Rejected:
                rejected = True
            run.check(f"rejects {label}", rejected, f"stamped {value!r}")
        run.check(
            "a rejected SHA leaves the compose file byte-identical",
            open(compose, encoding="utf-8").read() == before,
        )

        # --- fail closed on a compose file it cannot read confidently --------
        malformed = {
            "no services: block": "volumes:\n  flagit_data:\n",
            "two services: blocks": FIXTURE + "services:\n  other:\n    image: x\n",
            "no flagit service": FIXTURE.replace("  flagit:\n", "  other:\n", 1),
            "flagit service without environment:": (
                "services:\n  flagit:\n    image: flagit\n    expose:\n      - \"8080\"\n"
            ),
            "environment: as a list": (
                "services:\n  flagit:\n    environment:\n      - FLAGIT_PUBLIC_URL=https://x\n"
            ),
            "duplicate FLAGIT_COMMIT entries": FIXTURE.replace(
                "      FLAGIT_ADMIN_KEY: ${FLAGIT_ADMIN_KEY}\n",
                f"      {KEY}: {SHA_A}\n      {KEY}: {SHA_B}\n",
                1,
            ),
            "tab indentation": FIXTURE.replace("  flagit:", "\tflagit:", 1),
        }
        for label, text in malformed.items():
            try:
                stamp_commit(text, SHA_A)
                rejected = False
            except Rejected:
                rejected = True
            run.check(f"rejects compose with {label}", rejected)

        # --- the happy path: exactly one line, in exactly one place ----------
        stamp_file(compose, SHA_A)
        stamped = open(compose, encoding="utf-8").read()
        before_lines, after_lines = FIXTURE.splitlines(), stamped.splitlines()
        added = [line for line in after_lines if line not in before_lines]
        removed = [line for line in before_lines if line not in after_lines]
        run.check(
            "stamping adds exactly one line and removes none",
            len(added) == 1 and not removed,
            f"added={added} removed={removed}",
        )
        run.check(
            "the added line is the FLAGIT_COMMIT entry at the environment indent",
            added == [f"      {KEY}: {SHA_A}"],
            f"added={added}",
        )
        run.check(
            "the entry lands inside the flagit service, above tailscale-admin",
            0 < stamped.index(f"{KEY}: {SHA_A}") < stamped.index("tailscale-admin:"),
        )
        run.check(
            f"{KEY} appears exactly once in the whole file",
            stamped.count(f"{KEY}:") == 1,
        )

        # --- nothing else in the deployment moved ----------------------------
        run.check(
            "the tailscale-admin service is byte-identical",
            TAILSCALE_BLOCK in stamped,
        )
        run.check(
            "the named volumes are preserved",
            "volumes:\n  flagit_data:\n  tailscale_state:\n" in stamped,
        )
        run.check(
            "the expose declarations are preserved",
            'expose:\n      - "8080"\n      - "3000"' in stamped,
        )
        run.check(
            "no ports: entry is introduced — the exposure boundary is unchanged",
            "ports:" not in stamped,
        )
        run.check(
            "no other environment entry is touched",
            "FLAGIT_PUBLIC_URL: https://flagit.example.test" in stamped
            and "FLAGIT_ADMIN_KEY: ${FLAGIT_ADMIN_KEY}" in stamped
            and "TS_AUTHKEY: ${TS_AUTHKEY}" in stamped,
        )
        run.check(
            "the comment above the first environment entry stays with it",
            "# Externally reachable base URL, used to build the ticket link.\n"
            "      FLAGIT_PUBLIC_URL:" in stamped,
        )

        # --- idempotence -----------------------------------------------------
        stamp_file(compose, SHA_A)
        run.check(
            "restamping the same SHA is a byte-for-byte no-op",
            open(compose, encoding="utf-8").read() == stamped,
        )
        stamp_file(compose, SHA_B)
        restamped = open(compose, encoding="utf-8").read()
        run.check(
            "a new SHA replaces the single line instead of appending one",
            restamped.count(f"{KEY}:") == 1
            and f"{KEY}: {SHA_B}" in restamped
            and SHA_A not in restamped
            and len(restamped.splitlines()) == len(stamped.splitlines()),
        )

        # --- service order must not decide which service gets stamped --------
        reordered = stamp_commit(FIXTURE_REORDERED, SHA_A)
        flagit_start = reordered.index("  flagit:")
        run.check(
            "stamps flagit even when tailscale-admin is declared first",
            reordered.index(f"{KEY}: {SHA_A}") > flagit_start
            and "TS_AUTHKEY: ${TS_AUTHKEY}\n" in reordered
            and reordered.count(f"{KEY}:") == 1,
        )

        # --- what the container ends up reporting ----------------------------
        value = flagit_commit_env(stamped)
        run.check(f"the container receives {KEY} verbatim", value == SHA_A, f"got {value!r}")
        resolved = resolve_version(value, baked="")
        run.check(
            "version resolution returns the relay's SHA, not unknown",
            resolved == SHA_A and resolved != VERSION_UNKNOWN,
            f"resolved {resolved!r}",
        )
        run.check(
            "the relay's SHA still wins over a stale baked-in commit",
            resolve_version(value, baked=SHA_B) == SHA_A,
        )
        run.check(
            "/internal/version would report it as known",
            resolved != VERSION_UNKNOWN and len(resolved) == 40,
        )
    finally:
        shutil.rmtree(workdir, ignore_errors=True)

    total = run.passed + run.failed
    print(f"\n{run.passed}/{total} checks passed")
    if run.failed:
        print(f"{run.failed} failed")
        return 1
    print("Coolify raw-Compose webhook provenance contract holds.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
