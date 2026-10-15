<!-- bmad:context -->
<!-- Verified 2026-10-15 against 8c41cd3. Managed by bmad-project-context; edits inside this block are replaced on refresh. Keep anything you want preserved outside the markers. -->

## sovereign

Sovereign home server project using the BMad Method for planning and implementation. Planning and implementation artifacts live in `_bmad-output/`.

## Policy

- Commit and push (`git push origin main`) with clear, descriptive commit messages after every important step or completed milestone.
- Remote repository is `http://git.crins:3000/jens/sovereign`.
- Do not commit personal configs (`*.user.toml`).

## Where things are

- BMad configuration: `_bmad/`
- Output artifacts: `_bmad-output/planning-artifacts/` and `_bmad-output/implementation-artifacts/`
- Skill definitions: `.agent/skills/` and `.agents/skills/`

## Running and verifying

- Python commands should use `python3` (e.g. for `_bmad/scripts/...`).
- Network operations against `git.crins` may require running outside the sandbox (`BypassSandbox: true`).

<!-- /bmad:context -->
