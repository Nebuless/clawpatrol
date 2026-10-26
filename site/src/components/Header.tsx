import { Button } from "./Button";

export function Header() {
  return (
    <header class="frosted-header w-full py-5">
      <nav className="max-w-6xl mx-auto px-6 sm:px-8 flex flex-wrap sm:justify-between gap-y-2 items-center justify-center gap-16">
        <a
          href="/"
          aria-label="PRX home"
          class="text-2xl
        font-black font-display hover:text-rust"
        >
          <span class="prx-wordmark">
            <img src="/prx-icon.svg" alt="" />
            <span class="prx-wm-name">PRX</span>
            <span class="prx-wm-by">Nebuless</span>
          </span>
        </a>
        <div class="flex flex-wrap items-center gap-4 sm:gap-8 text-sm justify-center sm:justify-start">
          <a
            href="/docs/"
            class="transition-colors font-mono uppercase
          underline underline-offset-4 hover:text-rust"
          >
            Docs
          </a>
          <a
            href="/demo/"
            class="transition-colors font-mono uppercase
          underline underline-offset-4 hover:text-rust"
          >
            Demo
          </a>
          <Button
            href="/#book-a-pilot"
            size="sm"
            class="inline-flex items-center gap-2 !outline-navy-900"
          >
            Book a pilot
          </Button>
        </div>
      </nav>
    </header>
  );
}
