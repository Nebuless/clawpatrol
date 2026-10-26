import { Button } from "./Button";

const links = [["Product", "/"], ["How it works", "/how-it-works/"], ["Security", "/security/"], ["Deployment", "/deployment/"], ["Docs", "/docs/"], ["Dashboard", "/demo/"]];
function Links() {
  return <>{links.map(([label, href]) => <a href={href} key={href}>{label}</a>)}</>;
}
export function Header() {
  return <header class="frosted-header w-full py-5">
    <nav aria-label="Main" class="site-nav max-w-6xl mx-auto px-6 sm:px-8">
      <a href="/" aria-label="PRX home" class="prx-wordmark"><img src="/prx-icon.svg" alt="" /><span class="prx-wm-name">PRX</span><span class="prx-wm-by">Nebuless</span></a>
      <div class="site-nav-desktop"><Links /><Button href="/pilot/" size="sm">Book a pilot</Button></div>
      <details class="site-nav-mobile"><summary>Menu</summary><div class="site-nav-disclosure"><Links /><Button href="/pilot/" size="sm">Book a pilot</Button></div></details>
    </nav>
  </header>;
}
