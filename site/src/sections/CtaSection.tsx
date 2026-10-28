import { Button } from "../components/Button";
import { InstallTerminal } from "../components/InstallTerminal";

export function CtaSection() {
  return (
    <section
      class="max-w-5xl mx-auto px-6 sm:px-8
      py-32 md:py-64 text-center"
    >
      <img
        src="/prx-icon.svg"
        alt=""
        aria-hidden="true"
        class="size-24 mx-auto mb-8"
      />
      <h2 id="book-a-pilot" class="text-3xl sm:text-5xl mb-4 font-display">Start with a pilot</h2>
      <p
        class="max-w-lg mx-auto text-base sm:text-lg
         mb-4 text-text-muted"
      >
        Run PRX on one server inside your network with one team's agents. The
        source is{" "}
        <a
          href="https://github.com/Nebuless/clawpatrol/blob/main/LICENSE.md"
          class="underline underline-offset-2"
        >
          MIT licensed
        </a>
        , so your security team can review every line.
      </p>
      <div class="mb-8 mt-16 flex justify-center">
        <InstallTerminal variant="expanded" />
      </div>
      <Button
        href="/pilot/"
        size="lg"
      >
        Book a pilot
      </Button>
    </section>
  );
}
