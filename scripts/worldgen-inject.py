#!/usr/bin/env python3
"""
worldgen-inject.py — merge a Haiku-generated world fragment into world.yaml.

Reads YAML from stdin. Strips markdown code fences if present.
Uses yq to merge rooms, loot_tables, and crafting_recipes, skipping
entries whose IDs already exist (idempotent).

World file: $GLITCH_MUD_WORLD  (default: ~/Projects/gl1tch-mud/worlds/cyberspace/world.yaml)
Requires: yq v4 (brew install yq)
"""

import os
import re
import sys
import shutil
import datetime
import subprocess
import tempfile

WORLD_PATH = os.path.expanduser(
    os.environ.get(
        "GLITCH_MUD_WORLD",
        "~/Projects/gl1tch-mud/worlds/cyberspace/world.yaml",
    )
)

MERGE_KEYS = ["rooms", "loot_tables", "crafting_recipes", "factions"]


def strip_fences(text: str) -> str:
    text = re.sub(r"^```(?:yaml)?\s*\n?", "", text.strip(), flags=re.MULTILINE)
    text = re.sub(r"\n?```\s*$", "", text.strip(), flags=re.MULTILINE)
    return text.strip()


def sanitize_yaml(text: str) -> str:
    """Fix common Haiku YAML mistakes before parsing."""
    # Strip <brain>...</brain> blocks that Haiku sometimes appends.
    text = re.sub(r'<brain\b[^>]*>.*?</brain>', '', text, flags=re.DOTALL)
    # Strip any remaining bare XML/HTML tags.
    text = re.sub(r'<[^>]+>', '', text)
    # Strip markdown *emphasis* / *action text* outside quoted strings.
    text = re.sub(r'\s*\*[^*\n]+\*', '', text)
    return text


def extract_yaml_block(text: str) -> str:
    """Pull the YAML portion starting from the first recognised top-level key."""
    top_keys = "|".join(MERGE_KEYS)
    m = re.search(rf"^({top_keys}):", text, re.MULTILINE)
    return text[m.start():] if m else text


def yq(expr: str, *paths: str, input_text: str | None = None) -> str:
    """Run yq with the given expression and return stdout."""
    cmd = ["/opt/homebrew/bin/yq", expr, *paths]
    result = subprocess.run(
        cmd,
        input=input_text,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(f"yq failed: {result.stderr.strip()}")
    return result.stdout.strip()


def existing_ids(key: str) -> set[str]:
    """Return the set of IDs already present in a top-level array in world.yaml."""
    try:
        raw = yq(f".{key}[].id", WORLD_PATH)
        return {line.strip() for line in raw.splitlines() if line.strip() and line.strip() != "null"}
    except RuntimeError:
        return set()


def fragment_entries(key: str, fragment_path: str) -> list[str]:
    """Return each entry in the fragment array as a YAML string."""
    try:
        count_raw = yq(f".{key} | length", fragment_path)
        count = int(count_raw) if count_raw.isdigit() else 0
    except (RuntimeError, ValueError):
        return []
    entries = []
    for i in range(count):
        try:
            entry = yq(f".{key}[{i}]", fragment_path)
            eid = yq(f".{key}[{i}].id", fragment_path)
            if eid and eid != "null":
                entries.append((eid, entry))
        except RuntimeError:
            pass
    return entries


def append_entry(key: str, entry_yaml: str) -> None:
    """Append one entry to a top-level array in world.yaml using yq in-place."""
    yq(
        f".{key} += [load(\"/dev/stdin\")]",
        WORLD_PATH,
        f"--from-file=/dev/stdin",
    )
    # yq can't easily read two inputs; use a temp file for the entry
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tf:
        tf.write(entry_yaml)
        tf_path = tf.name
    try:
        result = subprocess.run(
            ["/opt/homebrew/bin/yq", f".{key} += [load(\"{tf_path}\")]", "--inplace", WORLD_PATH],
            capture_output=True,
            text=True,
        )
        if result.returncode != 0:
            raise RuntimeError(f"yq append failed: {result.stderr.strip()}")
    finally:
        os.unlink(tf_path)


def _merge_loot_entries(table_id: str, table_yaml: str) -> list[str]:
    """Merge new loot entries into an existing loot table, skipping duplicate item_ids."""
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tf:
        tf.write(table_yaml)
        frag_path = tf.name
    try:
        count_raw = yq(".entries | length", frag_path)
        count = int(count_raw) if count_raw.isdigit() else 0
    except (RuntimeError, ValueError):
        os.unlink(frag_path)
        return []

    # Collect existing item_ids in this table.
    try:
        raw = yq(f".loot_tables[] | select(.id == \"{table_id}\") | .entries[].item_id", WORLD_PATH)
        existing = {line.strip() for line in raw.splitlines() if line.strip() and line.strip() != "null"}
    except RuntimeError:
        existing = set()

    added = []
    for i in range(count):
        try:
            item_id = yq(f".entries[{i}].item_id", frag_path)
            if not item_id or item_id == "null" or item_id in existing:
                continue
            entry_yaml = yq(f".entries[{i}]", frag_path)
        except RuntimeError:
            continue
        with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as etf:
            etf.write(entry_yaml)
            entry_path = etf.name
        try:
            result = subprocess.run(
                [
                    "/opt/homebrew/bin/yq",
                    f"(.loot_tables[] | select(.id == \"{table_id}\") | .entries) += [load(\"{entry_path}\")]",
                    "--inplace",
                    WORLD_PATH,
                ],
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                print(f"worldgen-inject: warn: loot entry {item_id}: {result.stderr.strip()}", file=sys.stderr)
                continue
        finally:
            os.unlink(entry_path)
        existing.add(item_id)
        added.append(item_id)

    os.unlink(frag_path)
    return added


def _merge_room_items(room_id: str, room_yaml: str) -> list[str]:
    """Merge new items into an existing room's items list, skipping duplicate item ids."""
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tf:
        tf.write(room_yaml)
        frag_path = tf.name
    try:
        count_raw = yq(".items | length", frag_path)
        count = int(count_raw) if count_raw.isdigit() else 0
    except (RuntimeError, ValueError):
        os.unlink(frag_path)
        return []

    try:
        raw = yq(f".rooms[] | select(.id == \"{room_id}\") | .items[].id", WORLD_PATH)
        existing = {line.strip() for line in raw.splitlines() if line.strip() and line.strip() != "null"}
    except RuntimeError:
        existing = set()

    added = []
    for i in range(count):
        try:
            item_id = yq(f".items[{i}].id", frag_path)
            if not item_id or item_id == "null" or item_id in existing:
                continue
            item_yaml = yq(f".items[{i}]", frag_path)
        except RuntimeError:
            continue
        with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as etf:
            etf.write(item_yaml)
            entry_path = etf.name
        try:
            result = subprocess.run(
                [
                    "/opt/homebrew/bin/yq",
                    f"(.rooms[] | select(.id == \"{room_id}\") | .items) += [load(\"{entry_path}\")]",
                    "--inplace",
                    WORLD_PATH,
                ],
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                print(f"worldgen-inject: warn: room item {item_id}: {result.stderr.strip()}", file=sys.stderr)
                continue
        finally:
            os.unlink(entry_path)
        existing.add(item_id)
        added.append(item_id)

    os.unlink(frag_path)
    return added


def _merge_room_array(room_id: str, array_key: str, id_field: str, room_yaml: str) -> list[str]:
    """Generic helper to merge new entries into an existing room's array by id_field."""
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tf:
        tf.write(room_yaml)
        frag_path = tf.name
    try:
        count_raw = yq(f".{array_key} | length", frag_path)
        count = int(count_raw) if count_raw.isdigit() else 0
    except (RuntimeError, ValueError):
        os.unlink(frag_path)
        return []

    try:
        raw = yq(f".rooms[] | select(.id == \"{room_id}\") | .{array_key}[].{id_field}", WORLD_PATH)
        existing = {line.strip() for line in raw.splitlines() if line.strip() and line.strip() != "null"}
    except RuntimeError:
        existing = set()

    added = []
    for i in range(count):
        try:
            eid = yq(f".{array_key}[{i}].{id_field}", frag_path)
            if not eid or eid == "null" or eid in existing:
                continue
            entry_yaml = yq(f".{array_key}[{i}]", frag_path)
        except RuntimeError:
            continue
        with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as etf:
            etf.write(entry_yaml)
            entry_path = etf.name
        try:
            result = subprocess.run(
                [
                    "/opt/homebrew/bin/yq",
                    f"(.rooms[] | select(.id == \"{room_id}\") | .{array_key}) += [load(\"{entry_path}\")]",
                    "--inplace",
                    WORLD_PATH,
                ],
                capture_output=True,
                text=True,
            )
            if result.returncode != 0:
                print(f"worldgen-inject: warn: room {array_key} {eid}: {result.stderr.strip()}", file=sys.stderr)
                continue
        finally:
            os.unlink(entry_path)
        existing.add(eid)
        added.append(eid)

    os.unlink(frag_path)
    return added


def main() -> None:
    raw = sys.stdin.read()
    if not raw.strip():
        print("worldgen-inject: empty input, nothing to do", file=sys.stderr)
        sys.exit(0)

    cleaned = strip_fences(raw)
    cleaned = sanitize_yaml(cleaned)
    cleaned = extract_yaml_block(cleaned)

    if not os.path.exists(WORLD_PATH):
        print(f"worldgen-inject: world file not found: {WORLD_PATH}", file=sys.stderr)
        sys.exit(1)

    # Write fragment to a temp file so yq can read it.
    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as tf:
        tf.write(cleaned)
        frag_path = tf.name

    try:
        # Validate the fragment is parseable.
        try:
            yq(".", frag_path)
        except RuntimeError as e:
            print(f"worldgen-inject: invalid YAML fragment — {e}", file=sys.stderr)
            print("--- fragment ---", file=sys.stderr)
            print(cleaned[:4000], file=sys.stderr)
            sys.exit(1)

        # Backup world.yaml before modifying.
        bak_path = WORLD_PATH + ".bak"
        shutil.copy2(WORLD_PATH, bak_path)

        added: dict[str, list[str]] = {}

        for key in MERGE_KEYS:
            ext = existing_ids(key)
            entries = fragment_entries(key, frag_path)
            for eid, entry_yaml in entries:
                if eid not in ext:
                    # New top-level entry — append it.
                    with tempfile.NamedTemporaryFile("w", suffix=".yaml", delete=False) as etf:
                        etf.write(entry_yaml)
                        entry_path = etf.name
                    try:
                        result = subprocess.run(
                            [
                                "/opt/homebrew/bin/yq",
                                f".{key} += [load(\"{entry_path}\")]",
                                "--inplace",
                                WORLD_PATH,
                            ],
                            capture_output=True,
                            text=True,
                        )
                        if result.returncode != 0:
                            print(
                                f"worldgen-inject: warn: failed to add {key}/{eid}: {result.stderr.strip()}",
                                file=sys.stderr,
                            )
                            continue
                    finally:
                        os.unlink(entry_path)
                    ext.add(eid)
                    added.setdefault(key, []).append(eid)
                elif key == "loot_tables":
                    # Existing loot table — merge new entries by item_id.
                    merged = _merge_loot_entries(eid, entry_yaml)
                    if merged:
                        added.setdefault(key + ":entries", []).extend(
                            f"{eid}/{iid}" for iid in merged
                        )
                elif key == "rooms":
                    # Existing room — merge new items, npcs, systems, and locks.
                    for sub_key, id_field in [("items", "id"), ("npcs", "id"), ("systems", "id"), ("locks", "id")]:
                        merged = (
                            _merge_room_items(eid, entry_yaml)
                            if sub_key == "items"
                            else _merge_room_array(eid, sub_key, id_field, entry_yaml)
                        )
                        if merged:
                            added.setdefault(f"{key}:{sub_key}", []).extend(
                                f"{eid}/{iid}" for iid in merged
                            )

    finally:
        os.unlink(frag_path)

    if not added:
        print("worldgen-inject: all IDs already present, nothing added")
        sys.exit(0)

    ts = datetime.datetime.now().isoformat(timespec="seconds")
    print(f"[{ts}] worldgen-inject: world expanded:")
    for key, ids in added.items():
        print(f"  {key}: {', '.join(ids)}")
    print(f"  backup: {bak_path}")


if __name__ == "__main__":
    main()
