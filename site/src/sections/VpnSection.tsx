import { useEffect, useState } from "preact/hooks";

// Sits right under the hero. The pitch: wrapping an agent with
// PRX takes one word — no SDK, no rewrite. A dashboard still is
// the visual anchor; copy and command line are deliberately
// lightweight so they don't compete with it.

const AGENTS = ["codex", "claude", "gemini", "cursor-agent", "aider"];
const TYPE_MS = 90;
const DELETE_MS = 45;
const HOLD_MS = 1400;

// Watches `prefers-reduced-motion` and stays in sync if the user
// toggles it. Starts false on first render so SSR matches; the
// effect-driven update means a single-frame flash of the animated
// state for reduced-motion users on hydration, which is acceptable.
function useReducedMotion() {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
    setReduced(mq.matches);
    const onChange = () => setReduced(mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);
  return reduced;
}

// Loops through AGENTS, typing each name, holding, deleting, moving
// to the next. min-w on the wrapper keeps the command line from
// reflowing as the name length changes. Respects
// `prefers-reduced-motion` by short-circuiting to a static name.
function AgentTypewriter() {
  const reduced = useReducedMotion();
  const [i, setI] = useState(0);
  const [text, setText] = useState("");
  const [phase, setPhase] = useState<"typing" | "holding" | "deleting">(
    "typing",
  );

  useEffect(() => {
    if (reduced) {
      setText(AGENTS[0]);
      return;
    }
    const target = AGENTS[i];
    let t: ReturnType<typeof setTimeout>;
    if (phase === "typing") {
      if (text.length < target.length) {
        t = setTimeout(
          () => setText(target.slice(0, text.length + 1)),
          TYPE_MS,
        );
      } else {
        t = setTimeout(() => setPhase("holding"), HOLD_MS);
      }
    } else if (phase === "holding") {
      t = setTimeout(() => setPhase("deleting"), 0);
    } else {
      if (text.length > 0) {
        t = setTimeout(() => setText(text.slice(0, -1)), DELETE_MS);
      } else {
        setI((n) => (n + 1) % AGENTS.length);
        setPhase("typing");
      }
    }
    return () => clearTimeout(t);
  }, [text, phase, i, reduced]);

  return (
    <span class="inline-block min-w-[13ch] text-left text-canvas">
      {text}
      {!reduced && (
        <span class="animate-pulse [animation-duration:0.8s]">_</span>
      )}
    </span>
  );
}

export function VpnSection() {
  return (
    <section class="bg-navy-700 py-32 sm:py-44 text-canvas">
      <div class="max-w-6xl mx-auto px-6 sm:px-8">
        <div class="grid md:grid-cols-[2fr_3fr] gap-12 md:gap-12 lg:gap-16 items-center w-full">
          <div class="min-w-0 flex flex-col items-center md:items-start text-center md:text-left">
            <h3 class="text-3xl sm:text-4xl md:text-5xl font-display text-balance mb-3 text-canvas">
              Works with the agents <span class="text-rust">your team already uses</span>
            </h3>
            <p class="text-base text-canvas/70 mb-6 text-pretty">
              Prefix the agent's command with{" "}
              <code class="font-mono text-canvas">nebprx run</code>. Same
              workflow. Every request PRX inspects is recorded.
            </p>
            <div class="font-mono text-sm text-canvas/80">
              <span class="text-canvas/40">$</span> nebprx run{" "}
              <AgentTypewriter />
            </div>
          </div>
          <div class="flex justify-center md:justify-end">
            <img
              src="/screenshots/prx-dashboard.png"
              alt="PRX dashboard: live overview of devices and agent requests"
              class="squircle-lg border-1.5 border-navy-500 block w-full max-w-xl"
            />
          </div>
        </div>
      </div>
    </section>
  );
}
