# Taste Skill

Installed skill: design-taste-frontend, the sole current principal skill in
https://github.com/Leonxlnx/taste-skill.
Pinned revision: 12af471f627f866411b4bd785b19646d3672784c.
Upstream directory: skills/taste-skill/.
Canonical local directory: .agents/skills/design-taste-frontend/.
All 39 upstream skill files are unchanged. The upstream root MIT license
(Copyright 2026 Leonxlnx) is retained as design-taste-frontend/LICENSE.

## Discovery

| Runtime | Project discovery path | Validation |
| --- | --- | --- |
| Codex | .agents/skills/design-taste-frontend/SKILL.md | Official repository skill-root source below. No plugin required. |
| Pi 1.4.0 | Same canonical path | Installed package manager found one enabled project resource. Skill loader accepted frontmatter without diagnostics. |
| OmO 5.1.38 / Senpi 2026.10.19 | Same canonical path | Installed Senpi package manager and loader returned one enabled project skill without diagnostics. |

Pi and OmO discover .agents/skills natively after project trust. No additional
.pi, .omo or .senpi links/settings are needed. Checks used trusted in-memory
empty settings, not changes to trust or global configuration. Runtime-specific
disabled-skills settings can override discovery. The existing .claude/skills
symlink to ../.agents/skills also exposes this payload.

Start a new Codex session. Use /reload or a new session in Pi and OmO.
Discovery validation did not execute the design workflow, browser or image search.
Node 22+ and Chrome/Edge/Chromium are upstream execution requirements, not
installation dependencies.

Evidence: installed Pi docs/skills.md and dist/core/package-manager.js;
installed @code-yeongyu/senpi/docs/skills.md and its package manager, reached
through the installed omo-ai launcher;
[Codex skill roots](https://github.com/openai/codex/blob/main/codex-rs/ext/skills/src/host_roots.rs).

## Install and update

Equivalent pinned, project-local installation for the requested
skills add Leonxlnx/taste-skill, restricted to the README's principal skill.
The following helper comes with Codex and is not shipped by PRX. Pi/OmO users
can check out the pinned upstream source and copy skills/taste-skill plus its
root MIT license to the named canonical directory instead:

~~~sh
python3 "${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-installer/scripts/install-skill-from-github.py" --repo Leonxlnx/taste-skill --ref 12af471f627f866411b4bd785b19646d3672784c --path skills/taste-skill --name design-taste-frontend --dest .agents/skills --method download
~~~

The installer refuses an existing destination. To update, review a new SHA,
run the command with that SHA and a temporary --dest, copy the upstream root
LICENSE into that skill directory, review the diff, then replace only
.agents/skills/design-taste-frontend/ and update this record. Preserve other skills.
No global installation, package manifest or node_modules is needed.

## Project boundaries and upstream caveats

PRX's existing **Clear Glass** design system, brand assets, shared tokens and
approved UI conventions take precedence over this skill's aesthetic defaults,
including its blanket no-glass rule. Installation does not authorize a redesign.
Preserve existing styles unless the user explicitly approves changing them.

Required bundled references, script imports and JSON libraries are present.
Upstream documents optional scene demos and library/scenes.json that do not
ship at this revision. references/scenes.md explicitly says no scene library
ships yet; draw.mjs handles its absence. No missing files were fabricated.
External imagery and fonts retain their own licenses, as upstream explains.
Invoked scripts may create .taste-skill/ and user draw history. None were invoked.
