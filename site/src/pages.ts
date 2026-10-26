import { Landing } from "./Landing";
import { SITE_TITLE } from "./sections/HeroSection";
import { HowItWorks, Security, Deployment, Pilot } from "./pages/ProductPages";

export const pages = [
  { path: "/", title: SITE_TITLE, description: "PRX runs inside your network. Agents never see your credentials, risky actions wait for approval, and inspected requests land in an exportable audit log.", component: Landing },
  { path: "/how-it-works/", title: "How it works — PRX by Nebuless", description: "See how PRX checks agent requests, injects credentials, and holds risky actions for approval before they reach your services.", component: HowItWorks },
  { path: "/security/", title: "Security — PRX by Nebuless", description: "Review PRX security controls, credential isolation, data handling, telemetry, and your responsibilities as a self-hosted gateway operator.", component: Security },
  { path: "/deployment/", title: "Deployment — PRX by Nebuless", description: "Deploy PRX on Linux, macOS, and Windows. Choose a gateway host, connect agents over WireGuard or Tailscale, and manage backups and upgrades.", component: Deployment },
  { path: "/pilot/", title: "Book a pilot — PRX by Nebuless", description: "Start a PRX pilot with one gateway inside your network and one team's agents. Test credential isolation, policy, approvals, and request review.", component: Pilot },
];

export function pageForPath(path: string) {
  const normalized = path.replace(/\/index\.html$/, "/").replace(/\/?$/, "/");
  return pages.find((page) => page.path === normalized);
}
