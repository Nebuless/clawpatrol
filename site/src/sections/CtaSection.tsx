import { Button } from "../components/Button.tsx";
import { assessmentEmail } from "../lib/contact.ts";

export function CtaSection() {
  return (
    <section class="assessment-section assessment-close">
      <div class="assessment-frame assessment-split">
        <div>
          <h2>Start with your toolchain</h2>
          <p>
            Tell us what your team uses and which access decisions need attention. We will discuss
            an on-site assessment and agree scope and timing together.
          </p>
        </div>
        <div>
          <Button href={assessmentEmail} size="lg">
            Discuss an assessment
          </Button>
          <p>
            <a href="mailto:pilot@nebuless.cloud">pilot@nebuless.cloud</a>
          </p>
          <a class="prx-link-cta" href="/pilot/">
            Already have a scoped workflow? Explore a pilot.
          </a>
        </div>
      </div>
    </section>
  );
}
