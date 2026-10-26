import { useEffect, useRef, useState } from "preact/hooks";
import { Button } from "../components/Button";
import { SectionLabel } from "../components/SectionLabel";

// The dashboard SPA is built into /demo/ and reads bundled fixtures.

const DEMO_URL = "/demo/";

// Load the dashboard near the viewport so its activity stream starts
// as visitors reach this section. The margin allows time to bootstrap.
function LazyDemoIframe() {
  const ref = useRef<HTMLIFrameElement | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (loaded || !ref.current) return;
    const el = ref.current;
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          setLoaded(true);
          io.disconnect();
        }
      },
      { rootMargin: "400px 0px" },
    );
    io.observe(el);
    return () => io.disconnect();
  }, [loaded]);

  return (
    <iframe
      ref={ref}
      src={loaded ? DEMO_URL : undefined}
      title="PRX dashboard"
      class="h-144 w-full border-3 border-t-24 border-navy squircle-lg shadow-[4px_6px_0_0_var(--color-canvas-300)] bg-canvas-muted"
      width="100%"
    />
  );
}

export function DemoSection() {
  return (
    <section class="pt-20 pb-16 sm:pt-32 sm:pb-28">
      <div class="max-w-5xl mx-auto px-6 sm:px-8">
        <SectionLabel>Take a tour</SectionLabel>
        <h3 class="text-3xl sm:text-4xl lg:text-5xl font-display text-center text-balance">
          Click around the operator dashboard.
        </h3>
        <p class="text-center max-w-2xl mx-auto mb-12 mt-4 text-text-muted">
          Open any request to see exactly
          what the agent sent and what PRX decided.
        </p>
      </div>

      <div class="max-w-4xl mx-auto px-6 sm:px-8 mb-8">
        <LazyDemoIframe />
      </div>

      <div class="text-center">
        <Button
          href={DEMO_URL}
          size="md"
          target="_blank"
          rel="noopener noreferrer"
        >
          Open the dashboard
          <svg
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2.5"
            stroke-linecap="round"
            stroke-linejoin="round"
            class="inline-block w-3.5 h-3.5 ml-1.5 align-middle"
            aria-hidden="true"
          >
            <path d="M 7 17 L 17 7" />
            <path d="M 9 7 L 17 7 L 17 15" />
          </svg>
        </Button>
      </div>
    </section>
  );
}
