import { Button } from "../components/Button.tsx";
import { assessmentEmail } from "../lib/contact.ts";

import { HERO_H1 } from "../copy";

export function HeroSection() {
  return (
    <section class="prx-hero assessment-hero assessment-frame">
      <div class="assessment-split">
        <div>
          <h1>{HERO_H1}</h1>
          <p class="assessment-lead">
            An on-site AI toolchain assessment for credit unions and small/midsize businesses.
          </p>
          <p>
            Map coding agents, automations, and everyday assistants. Give admins a plan for
            visibility, credential ownership, data access, and approvals.
          </p>
          <div class="assessment-actions">
            <Button href={assessmentEmail} size="lg">
              Discuss an assessment
            </Button>
            <a href="/assessment/" class="prx-link-cta">
              What we review
            </a>
          </div>
          <p class="assessment-note">Agree scope and timing together.</p>
        </div>
        <figure class="assessment-product">
          <a href="/demo/" aria-label="Explore the PRX Demo">
            <img
              src="/screenshots/prx-dashboard.png"
              alt="PRX operator dashboard showing inspected requests and policy decisions"
              width="1280"
              height="800"
              fetchPriority="high"
            />
          </a>
          <figcaption>
            PRX gives admins visibility over supported traffic routed through the gateway.{" "}
            <a href="/demo/">Demo</a>
          </figcaption>
        </figure>
      </div>
    </section>
  );
}
