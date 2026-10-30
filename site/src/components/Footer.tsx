export function Footer() {
  return (
    <footer class="px-6 sm:px-8 py-12 sm:py-16 text-xs font-mono text-canvas bg-navy-700">
      <div class="w-full max-w-6xl mx-auto flex flex-col sm:flex-row gap-8 sm:items-center sm:justify-between">
        <div class="space-y-4">
          <p>PRX is a Nebuless product.</p>
          <p>Open-source under the MIT license.</p>
        </div>
        <nav aria-label="Footer" class="flex flex-wrap gap-x-6 gap-y-4">
          <a href="/assessment/">Assessment</a>
          <a href="/pilot/">PRX pilot</a>
          <a href="/demo/">Demo</a>
          <a href="https://github.com/Nebuless/clawpatrol/blob/main/SECURITY.md">
            Security reports
          </a>
          <a href="https://github.com/Nebuless/clawpatrol">Source</a>
          <a href="https://github.com/Nebuless/clawpatrol/blob/main/LICENSE.md">License</a>
          <a href="/docs/">Docs</a>
        </nav>
      </div>
    </footer>
  );
}
