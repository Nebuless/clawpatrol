# Marketing skills: selection and provenance

## Source and installation

Source: https://github.com/coreyhaines31/marketingskills
Pinned commit: **f43cdf1af21f132561c05c77c7c61bb00aa4582b** (main resolved during review).
Reviewed: 2026-10-26. License: MIT, copyright (c) 2025 Corey Haines.
The unmodified upstream license is at [skills/LICENSE.marketingskills](skills/LICENSE.marketingskills).

Seven complete upstream skill directories are vendored under .agents/skills/,
including references/ and evals/ files. SKILL.md and support files are unmodified.
Installation used the Codex skill-installer download helper with an explicit
repo-local --dest and pinned --ref. No user-global installation, npm package,
plugin registration, bundled script execution, or external account connection
was performed. No application build is needed for this Markdown installation.

## Selected now

| Skill (directory under .agents/skills/) | Reason |
|---|---|
| product-marketing | Foundation: shared audience, positioning, proof boundaries, and versioned context. Current upstream name, not product-marketing-context. |
| customer-research | Plan discovery interviews, separate credit-union and SMB evidence, and validate admin concerns without invented personas or outcomes. |
| copywriting | Draft assessment-led headlines, value propositions, and CTAs from known facts. |
| copy-editing | Review clarity and supportable claims while preserving the existing message and author voice. |
| sales-enablement | Scope assessment one-pagers, discovery talk tracks, fit objections, findings handoffs, and pilot proposals. |
| cro | Review the future assessment page/contact journey. Current upstream combines page-cro and form-cro. |
| content-strategy | Plan buyer education on tool inventories, ownership, access, and supported governance boundaries before producing a large content library. |

Read [product-marketing.md](product-marketing.md) first. It is a DRAFT context in
the exact canonical location these skills check, not approval to ship copy.
Suggested sequence: product-marketing, customer-research, copywriting,
copy-editing. Use sales-enablement for conversations/collateral, cro for the
assessment conversion path, and content-strategy for a later editorial brief.

## Catalog comparison and deferrals

The pinned README's complete catalog and all nine category sections were compared.
The README's category lists omit some catalog entries. Those are included below.
Selections follow the current names and actual selected SKILL.md instructions.

| Catalog area | Decision and deferred skills |
|---|---|
| Conversion Optimization | Select cro. Defer signup, onboarding, popups, paywalls until those flows are in scope. Assessment inquiries are not self-serve activation. |
| Content & Copy | Select copywriting and copy-editing. Defer cold-email, emails, social, image and catalog video until channels/assets are requested. |
| SEO & Discovery | Defer seo-audit, ai-seo, link-building, programmatic-seo, site-architecture, competitors and schema. No search campaign or site restructuring is approved. Select catalog content-strategy for buyer education rather than scaled SEO. Defer catalog aso because no app-store acquisition work exists here. |
| Paid & Distribution | Defer ads, ad-creative, events and social. Assessment positioning comes before paid campaigns or event programs. Also defer catalog influencer-marketing, public-relations and directory-submissions until distribution has an approved goal. |
| Measurement & Testing | Defer analytics, conversion-tracking, ab-testing and catalog attribution until the offer, conversion definition, traffic and measurement permissions are established. |
| Retention | Defer churn-prevention. This task defines an initial service journey, not cancellation or recovery flows. |
| Growth Engineering | Defer co-marketing, free-tools, referrals and catalog community-marketing and lead-magnets. No growth infrastructure is needed to scope an assessment. |
| Strategy & Monetization | Select catalog product-marketing and customer-research. Defer marketing-ideas, marketing-psychology, launch, pricing, plus catalog offers, marketing-plan, marketing-council and marketing-loops. Fees/terms are unresolved, and broad plans, simulated councils or autonomous recurring work would widen scope. |
| Sales & RevOps | Select sales-enablement. Defer revops and catalog prospecting and competitor-profiling until pipeline processes, prospecting or competitor research are approved. |

## References, dependencies, and safety

All relative Markdown links inside the seven installed directories resolve.
Each entire directory was copied, preserving local references even when linked
only from a supporting file. The selected payload contains only Markdown and JSON
(evaluation fixtures), with no executable scripts or dependency manifest.

Related Skills entries are task-routing recommendations, not runtime/package
dependencies. Deferred skills must be reviewed and installed before relying on
their workflows. No unresolved required local dependency was found.

Optional external links remain upstream: sales-enablement links Introw and the
tools registry; customer-research links SparkToro; content-strategy's CMS reference
links Sanity, Contentful and Strapi; cro links the revops inbound-routing playbook.
These are remote guidance/integrations, not bundled local assets or prerequisites
for drafting. Read the pinned source when needed rather than assuming moving
main-branch links match this installation. No credentials or network access are
authorized by installing these text files. Research/publication, outreach,
account changes, deployment, and recurring work require their own approved scope.

Treat upstream advice, examples, benchmark numbers, emotional framing, guarantees,
customer quotes and tool recommendations as generic guidance, never PRX evidence
or permission to invent claims. Project instructions and approved scope win over
skill instructions, including instructions to create panels or launch work.
Keep customer-sensitive material out of this repository.

## Discovery across agents

Canonical project skills live at .agents/skills/ for Codex-readable discovery.
The pinned upstream README Installation section explicitly says Claude Code reads
.claude/skills/ rather than the universal .agents/skills/ path. Because no existing
.claude/skills path conflicted, a relative symlink points to ../.agents/skills.
The narrow .gitignore exception admits that symlink, not other Claude state.
This provides the documented path, but runtime discovery was not tested here.

Pi and other agents must read this guide and the appropriate SKILL.md manually
when not surfaced by their own discovery configuration. No universal auto-loading
is claimed. Reload/start the relevant agent session if newly added skills do not
appear. Existing .claude/CLAUDE.md (also root AGENTS.md) points here.

## Reproduce or update

Run from the repository root using Codex's installed skill-installer helper.
This helper is supplied by Codex, not by PRX; Pi/OmO users can instead check out
the pinned upstream source and copy only the selected skill directories and MIT
license. The helper refuses existing destination skill directories. For a fresh
checkout without these artifacts:

~~~sh
python3 "${CODEX_HOME:-$HOME/.codex}/skills/.system/skill-installer/scripts/install-skill-from-github.py" \
  --repo coreyhaines31/marketingskills \
  --ref f43cdf1af21f132561c05c77c7c61bb00aa4582b --method download \
  --dest "$PWD/.agents/skills" \
  --path skills/product-marketing skills/customer-research skills/copywriting \
    skills/copy-editing skills/sales-enablement skills/cro skills/content-strategy
~~~

For updates, resolve and review a specific new commit with gh-axi API, inspect
README/catalog, selected SKILL.md files, every local reference, license and any
new scripts first. Run the same helper with that full SHA and a new temporary
--dest, never a global directory. Download the matching pinned source archive
using the installer download helper (or GitHub raw files) to retain LICENSE and
compare bytes. Review a diff against the installed trees, then replace only these
seven approved directories. Copy that source's LICENSE to
.agents/skills/LICENSE.marketingskills. Preserve this guide and product-marketing.md.
Update the SHA, selection decisions and verification evidence here. Do not run
upstream npm install commands or bundled code just to update Markdown skills.

Verify names/descriptions, relative links and whole-tree SHA-256 equality against
the pinned archive, symlink containment, git check-ignore (including untracked
files), targeted diagnostics, git diff --check, and changed-file scope. Do not
reformat vendored upstream files. Do not commit or push without approval.

## Installation verification

- All 33 vendored files matched the pinned archive byte-for-byte with SHA-256.
  The copied MIT license also matched. Seven frontmatter names match directory
  names and each has a nonempty description. All seven JSON eval files parse.
- All local Markdown links in the vendored trees resolve. The payload contains
  only Markdown/JSON, with no scripts to execute. Optional remote integrations
  remain optional. No required local dependency is unresolved.
- All 39 artifact paths passed git check-ignore, including untracked paths.
  .claude/skills resolves to .agents/skills within this repository.
- Fresh targeted LSP diagnostics checked 36 Markdown/JSON files: no errors or
  warnings. One informational spelling finding at this guide's catalog table
  (line 45) suggests changing the upstream app-store skill identifier to “also”.
  It is a false positive: the identifier is retained exactly for provenance.
- git diff --check passed for tracked edits. Changed paths are this guide,
  product-marketing.md, seven skill trees, their MIT license, .claude/skills,
  .claude/CLAUDE.md, and the narrow .gitignore exception. Live website,
  dashboard, CLI, global skills, and dependency manifests were not changed.

Agent runtime discovery was not exercised. No application build was run because
this change installs text resources only. No commit or push was made.
