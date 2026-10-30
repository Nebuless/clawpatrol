import { Layout } from "../Layout.tsx";
import { Button } from "../components/Button.tsx";
import { assessmentEmail } from "../lib/contact.ts";
import { CtaSection } from "../sections/CtaSection.tsx";

export function AssessmentOverview() {
  return (
    <section class="assessment-section">
      <div class="assessment-frame assessment-split">
        <div>
          <h2>
            Map the work.
            <br />
            Name the controls.
          </h2>
          <p>
            Review coding agents, internal automations, and everyday assistants with the people who
            use and administer them.
          </p>
          <a class="prx-link-cta" href="/assessment/">
            Explore the assessment
          </a>
        </div>
        <dl class="assessment-outputs">
          <div>
            <dt>Visibility</dt>
            <dd>Which tools connect to which services, and what data can they reach?</dd>
          </div>
          <div>
            <dt>Ownership</dt>
            <dd>Who owns the credentials, access decisions, and request review?</dd>
          </div>
          <div>
            <dt>Control</dt>
            <dd>Where should policy block an action or wait for approval?</dd>
          </div>
        </dl>
      </div>
    </section>
  );
}

export function Assessment() {
  return (
    <Layout>
      <section class="prx-hero assessment-hero assessment-frame">
        <div class="assessment-split">
          <div>
            <h1>
              Assess your
              <br />
              AI toolchain
            </h1>
            <p class="assessment-lead">
              An on-site AI toolchain assessment for credit unions and small/midsize businesses.
            </p>
            <p>
              Understand how work reaches your systems. Agree the visibility, ownership, and
              approval controls your admins need.
            </p>
            <Button href={assessmentEmail} size="lg">
              Discuss an assessment
            </Button>
            <p class="assessment-note">
              Start with a conversation. Agree scope and timing together.
            </p>
          </div>
          <div class="assessment-scope prx-glass-panel">
            <h2>Follow the access</h2>
            <ul>
              <li>Tools and integrations</li>
              <li>Credential ownership and scopes</li>
              <li>Data access and destinations</li>
              <li>Policy and approval paths</li>
              <li>Request records and review owners</li>
            </ul>
          </div>
        </div>
      </section>
      <section class="assessment-section">
        <div class="assessment-frame">
          <h2>What we review</h2>
          <p class="assessment-intro">
            Explore the workflows in your scope. Staff conversations and authorized configuration
            review establish what is known and what still needs investigation.
          </p>
          <div class="assessment-workflows">
            <details open>
              <summary>Coding agents</summary>
              <p>
                Review developer agents, repository access, shell tools, service integrations, and
                development credentials. Identify who can approve changes and sensitive actions.
              </p>
            </details>
            <details>
              <summary>Internal automations</summary>
              <p>
                Trace scheduled jobs, connected services, agent workflows, and service accounts.
                Establish credential owners, data destinations, and exception handling.
              </p>
            </details>
            <details>
              <summary>Everyday assistants</summary>
              <p>
                Review staff-reported assistants, connected workplace apps, shared content, and
                administrative settings. Record access boundaries and controls outside PRX coverage.
              </p>
            </details>
          </div>
        </div>
      </section>
      <section class="assessment-section bg-navy-700 text-canvas">
        <div class="assessment-frame assessment-split">
          <div>
            <h2>A practical control plan</h2>
            <p>
              Agree the depth and format of the outputs before assessment work begins. Findings stay
              tied to the systems and evidence reviewed.
            </p>
          </div>
          <dl class="assessment-outputs">
            <div>
              <dt>Inventory and workflow map</dt>
              <dd>Tools, integrations, access paths, credential owners, and recorded unknowns.</dd>
            </div>
            <div>
              <dt>Gaps and priorities</dt>
              <dd>
                Visibility, ownership, policy, and approval gaps with proposed controls and
                responsible owners.
              </dd>
            </div>
            <div>
              <dt>PRX fit and next steps</dt>
              <dd>
                Supported workflows to test with PRX, controls needed elsewhere, and a prioritized
                next-step plan.
              </dd>
            </div>
          </dl>
        </div>
      </section>
      <section class="assessment-section">
        <div class="assessment-frame">
          <h2>From discovery to fit</h2>
          <ol class="assessment-journey">
            <li>
              <h3>Discovery</h3>
              <p>Discuss your workflows, stakeholders, and the decisions you need to make.</p>
            </li>
            <li>
              <h3>On-site assessment</h3>
              <p>
                Agree participants, systems, exclusions, authorized access, and sensitive-data
                handling before review.
              </p>
            </li>
            <li>
              <h3>Findings and control plan</h3>
              <p>Review the inventory, access map, gaps, and control priorities with your team.</p>
            </li>
            <li>
              <h3>Fit-based pilot</h3>
              <p>
                Where PRX fits, test a limited workflow with named admins and agreed success checks.
              </p>
              <a href="/pilot/" class="prx-link-cta">
                Explore a PRX pilot
              </a>
            </li>
          </ol>
        </div>
      </section>
      <section class="assessment-section assessment-proof">
        <div class="assessment-frame assessment-split">
          <div>
            <h2>Where PRX fits</h2>
            <p>
              The human-led assessment reviews the agreed toolchain. PRX provides technical
              visibility and policy controls for supported traffic routed through its gateway.
            </p>
            <p>
              Direct or unsupported traffic remains outside that boundary. The assessment may
              recommend other controls or conclude that PRX is not a fit. It does not establish
              compliance certification.
            </p>
            <a class="prx-link-cta" href="/how-it-works/">
              Understand the gateway
            </a>
            <p>
              <a class="prx-link-cta" href="/security/">
                Review security and data handling
              </a>
            </p>
          </div>
          <figure>
            <img
              src="/screenshots/prx-dashboard.png"
              alt="PRX dashboard with device profiles, inspected requests, and a denied policy decision"
              width="1280"
              height="800"
              loading="lazy"
            />
            <figcaption>
              Review device profiles and inspected requests in PRX.{" "}
              <a href="/demo/">Explore the Demo</a>.
            </figcaption>
          </figure>
        </div>
      </section>
      <CtaSection />
    </Layout>
  );
}
