export function Footer() {
  return (
    <footer
      class="px-6 sm:px-8 py-12 sm:py-16 text-xs
        font-mono text-canvas bg-navy-700"
    >
      <div
        className="w-full max-w-6xl mx-auto flex flex-col
        sm:flex-row gap-6 sm:gap-8
        sm:items-center sm:justify-between"
      >
        <div className="space-y-4">
          <p>
            Open-source under the{" "}
            <a
              href="https://github.com/Nebuless/clawpatrol/blob/main/LICENSE.md"
              class="underline underline-offset-4
                hover:text-rust"
            >
              MIT license
            </a>
            .
          </p>
          <p class="max-w-sm ">
            PRX is a Nebuless product.
          </p>
        </div>
        <nav aria-label="Footer" class="flex flex-wrap gap-x-6 gap-y-2">
          <a
            href="/docs/"
            class="underline underline-offset-4
              hover:text-rust"
          >
            Docs
          </a>
          <a
            href="https://github.com/Nebuless/clawpatrol"
            class="underline underline-offset-4
              hover:text-rust"
          >
            Source
          </a>
          <a
            href="https://github.com/Nebuless/clawpatrol/blob/main/LICENSE.md"
            class="underline underline-offset-4
              hover:text-rust"
          >
            License
          </a>
        </nav>
      </div>
    </footer>
  );
}
