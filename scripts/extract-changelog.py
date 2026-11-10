#!/usr/bin/env python3
"""
Sovereign: Changelog & Release Notes Extractor
Extracts the release notes section for a given tag from CHANGELOG.md.
If the tag is not documented in CHANGELOG.md, optionally generates notes from git log.

Usage:
    python3 scripts/extract-changelog.py [options] <tag>

Options:
    --file, -f <path>    Path to CHANGELOG.md (default: CHANGELOG.md at repo root)
    --fallback-git       Generate git commit log if tag is missing in CHANGELOG.md
    --subject-only       Print only the first line / summary subject
    --body-only          Print only the body (excluding the top title line)
    --help, -h           Show this help message
"""

import sys
import os
import re
import subprocess
import argparse

def extract_section_from_changelog(changelog_path, tag):
    if not os.path.isfile(changelog_path):
        return None

    with open(changelog_path, "r", encoding="utf-8") as f:
        content = f.read()

    # Normalize tag (with and without leading 'v')
    raw_tag = tag.lstrip("v")
    tag_pattern = rf"##\s*\[(?:v)?{re.escape(raw_tag)}\](?:\s*-\s*([^\n]+))?"

    match = re.search(tag_pattern, content, re.IGNORECASE)
    if not match:
        return None

    start_idx = match.start()
    date_str = match.group(1) or ""

    # Find the next section starting with "## ["
    next_match = re.search(r"\n##\s*\[", content[match.end():])
    if next_match:
        end_idx = match.end() + next_match.start()
        section_text = content[start_idx:end_idx].strip()
    else:
        section_text = content[start_idx:].strip()

    return section_text

def generate_git_notes(tag, repo_root):
    # Find previous tag
    try:
        tags_output = subprocess.check_output(
            ["git", "tag", "--sort=-creatordate"],
            cwd=repo_root,
            stderr=subprocess.DEVNULL,
            text=True
        ).strip().splitlines()
    except Exception:
        tags_output = []

    prev_tag = None
    for t in tags_output:
        if t != tag:
            prev_tag = t
            break

    if prev_tag:
        rev_range = f"{prev_tag}..HEAD"
        header = f"Changes since {prev_tag}:"
    else:
        rev_range = "HEAD"
        header = "Initial release commits:"

    try:
        log_output = subprocess.check_output(
            ["git", "log", "--no-merges", "--pretty=format:- %s (%h)", rev_range],
            cwd=repo_root,
            stderr=subprocess.DEVNULL,
            text=True
        ).strip()
    except Exception:
        log_output = ""

    if not log_output:
        log_output = f"- Release {tag}"

    return f"## [{tag}]\n\n{header}\n\n{log_output}"

def main():
    parser = argparse.ArgumentParser(description="Extract release notes from CHANGELOG.md")
    parser.add_argument("tag", help="Release tag (e.g. v1.0.0 or v1.0.0-rc.2)")
    parser.add_argument("-f", "--file", default=None, help="Path to CHANGELOG.md")
    parser.add_argument("--fallback-git", action="store_true", help="Fallback to git log if tag not in CHANGELOG.md")
    parser.add_argument("--subject-only", action="store_true", help="Print only the subject line")
    parser.add_argument("--body-only", action="store_true", help="Print only the markdown body without top header")

    args = parser.parse_args()

    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    changelog_path = args.file or os.path.join(repo_root, "CHANGELOG.md")

    section = extract_section_from_changelog(changelog_path, args.tag)

    if not section:
        if args.fallback_git:
            section = generate_git_notes(args.tag, repo_root)
        else:
            sys.stderr.write(f"Error: Tag '{args.tag}' not found in {changelog_path}\n")
            sys.exit(1)

    lines = section.splitlines()
    if args.subject_only:
        # First heading line cleaned up
        title = lines[0].lstrip("#").strip() if lines else f"Release {args.tag}"
        print(title)
    elif args.body_only:
        # Strip the header line
        body_lines = lines[1:] if len(lines) > 1 else []
        body = "\n".join(body_lines).strip()
        print(body)
    else:
        print(section)

if __name__ == "__main__":
    main()
