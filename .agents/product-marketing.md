# PRX product marketing context

**Document version:** v3 (assessment-led website approved; engagement terms remain scoped)
**Last updated:** 2026-10-30

## Known direction

Lead with an on-site assessment of the current AI agent toolchain for credit
unions and small/midsize businesses. Include developer coding agents, internal
automations, and everyday assistants. Describe workflows without ranking the
people who use them. Give organizations and their admins a practical view of
what staff use and where governance needs attention before proposing PRX.

This direction comes from the user brief and the user's authorization to commit
and publish the completed website work on 2026-10-30. This approves the
assessment-led positioning, not fixed commercial terms, customer research,
or a claim of complete product coverage.

## Product overview and boundaries

PRX by Nebuless is a self-hosted AI agent gateway. For traffic routed through
supported gateway integrations, it can inspect requests, apply policy
allow/deny/approval decisions, inject scoped service credentials, and keep
request records. Configuration, protocols, routing, and deployment determine
coverage. Direct or unsupported traffic is outside that boundary.

The assessment is a service activity, not an automatic PRX discovery feature.
Interviewing staff and reviewing authorized configurations can reveal tools
and workflows PRX does not observe or govern. PRX does not automatically discover
every AI tool, control every endpoint, or cover every assistant. It does not
replace host security, network controls, or operational review.

Gateway hosts: Linux/macOS. Per-command `nebprx run`: Linux-only. macOS
whole-machine routing uses WireGuard. Windows implementation remains backlog.
The website and README describe the release-supported platforms. Verify
implementation and tests before adding other platform claims; do not promise
platform parity.

## Audience and buyer outcomes

Target organizations: credit unions and small/midsize businesses. Proposed
stakeholders, to validate: IT/security admins and technical owners as champions,
leadership as decision-makers, and staff as workflow experts. Buying authority,
budget, segment size, and procurement process are not established.

Proposed assessment outcomes, subject to an agreed scope:
- Inventory staff-reported and reviewed AI tools across coding, automation,
  and assistant workflows. Record scope, evidence, and unknowns.
- Map integrations, credential ownership/scopes, and data access without
  collecting secret values unnecessarily.
- Identify visibility, ownership, policy, and approval gaps.
- Determine which workflows supported PRX integrations can govern and which
  require other controls or further investigation.
- Propose a scoped deployment with named admins, credential profiles,
  allow/deny rules, approval responsibilities, and record-review practices.

## Proposed service journey and deliverables

1. Discovery conversation: understand workflows, stakeholders, access boundaries,
   and the decision the organization needs to make.
2. Scoped on-site assessment: agree systems, participants, authorized evidence,
   exclusions, and handling of sensitive data before any access.
3. Findings and control plan: proposed inventory, workflow/access map, gap list,
   PRX fit/non-fit matrix, and prioritized controls with owners. Final deliverable
   format and depth remain to be agreed.
4. Fit-based PRX pilot: only where routing/integrations fit. Agree a limited
   workflow, admins, policies, approval paths, record handling, and success checks.
   An assessment may conclude that PRX is not the appropriate control.

An assessment inventories and recommends. A pilot tests configured PRX behavior.
Neither establishes compliance certification or guaranteed risk reduction.

## Website prototype positioning

**Homepage headline:** Know your AI toolchain.

**Assessment page headline:** Assess your AI toolchain.

**Value proposition:** Start with an on-site AI toolchain assessment for your
credit union or small/midsize business. Review how staff use coding agents,
internal automations, and everyday assistants. Leave with a scoped control plan
that identifies where PRX can give admins request visibility, policy decisions,
and credential controls.

**Assessment CTA:** Discuss an assessment.

**CTA expectation:** Begin with a discovery conversation to agree assessment
scope and fit. This is not an instant booking or a promise of a free assessment.
Existing contact: pilot@nebuless.cloud. Do not change its use or create a booking
flow without approval.

## Voice, proof, and research gaps

Use plain, specific, admin-readable language. Explain visibility and governance
through concrete inventory, ownership, policy, approval, and record-review work.
Use PRX or PRX by Nebuless, CLI `nebprx`, and prx.nebuless.cloud. The repository
remains Nebuless/clawpatrol. Visual identity stays Clear Glass violet #6538d8.

Customer quotes, validated pains, competitors, switching motivations, actual
objections, pricing, customer outcomes, and conversion metrics are not supplied.
Do not invent them. Working hypotheses: admins may lack a consolidated inventory
or clear workflow ownership, and may prefer a scoped fit check before deployment.
Validate separately for each target segment with customer-research.

Potential fit objection: “Does this cover every assistant?” Answer: the service
reviews agreed workflows, while PRX governs supported traffic routed through it.
This is a draft response, not a recorded customer objection.

Do not claim automatic universal discovery, universal endpoint control,
comprehensive assistant coverage, certification, proven customer outcomes, ROI,
or quantified risk reductions. Existing marketing and upstream skill examples
are not evidence for PRX claims. Preserve broader current claims rather than
silently rewriting them. Flag unsupported statements and name the evidence needed
before publication. Request records may contain sensitive data and are not a
promise of a complete organizational audit trail.

## Open commercial questions

Duration, fees, travel geography, remote option, final deliverables and coverage,
access/consent arrangements, evidence retention, and pilot terms remain open.
No answers are implied by this context. The published inquiry asks the customer
to agree scope and timing before work begins. Resolve these questions before
quoting or committing to a specific engagement; do not publish invented terms.

## Sources and learnings

Sources: user task brief; README.md; site/src/sections/HeroSection.tsx;
site/src/pages/Assessment.tsx; site/src/pages/ProductPages.tsx. The local website
prototype now leads with assessment and places the PRX pilot after a fit review.
It has not been deployed to production. Product assertions need implementation
verification before sale.
No tested campaign learnings or customer proof are recorded yet.

## Changelog

- v2 (2026-10-26) — Recorded the implemented homepage, assessment route, CTA,
  and assessment-to-pilot journey in the local prototype.
- v1 (2026-10-26) — Initial draft of assessment-led direction, product boundaries,
  proposed journey, and unresolved commercial scope for user review.
