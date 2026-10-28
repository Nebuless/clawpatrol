import { Button } from "../components/Button";
import { IsometricStack } from "../components/IsometricStack";

// Single source of truth for the hero H1 and the page <title>.
// vite.config.ts uses SITE_TITLE in a transformIndexHtml hook, and
// docs-render.ts uses SITE_TITLE for prerender meta tags. Change
// here and all three surfaces stay in lockstep.
export const HERO_H1 = "The AI agent gateway for credit unions";
export const SITE_TITLE = `PRX by Nebuless - ${HERO_H1}`;

export function HeroSection() {
  return (
    <section
      class="prx-hero iso-stack-host max-w-6xl mx-auto px-6 sm:px-8
      py-16 sm:py-28"
    >
      <div class="grid md:grid-cols-2 gap-12 md:gap-12 lg:gap-16 items-center w-full">
        <div class="order-2 md:order-1 min-w-0 flex flex-col items-center md:items-start text-center md:text-left relative z-10 prx-glass-panel">
          <h1 class="text-4xl sm:text-5xl md:text-5.5xl lg:text-6xl lg:text-[4rem] mb-6 font-display text-balance text-text leading-none">
            Let your team use AI agents without handing them the keys
          </h1>
          <p class="text-sm md:text-base mb-6 max-w-2xl font-sans font-bold uppercase text-text text-balance">
            The AI agent gateway for{" "}
            <span className="text-rust">credit unions and community banks</span>
          </p>
          <p class="mb-10 max-w-2xl text-text-muted text-balance">
            PRX runs inside your network. Agents never see your credentials,
            the actions you flag wait for a person to approve them, and every
            request PRX inspects lands in an audit log you can export.
          </p>
          <div class="flex flex-wrap items-center gap-6 justify-center md:justify-start">
            <Button href="/pilot/" size="lg">
              Book a pilot
            </Button>
            <a href="/demo/" class="prx-link-cta">
              See the live demo
            </a>
          </div>
        </div>
        <div class="order-1 md:order-2 flex justify-center sticky top-50 z-0">
          <IsometricStack class="h-[calc(100dvh-4rem)] w-auto md:h-auto md:w-full md:max-w-56 lg:max-w-64" />
        </div>
      </div>
    </section>
  );
}
