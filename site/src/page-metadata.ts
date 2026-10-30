import { SITE_TITLE } from "./copy";

// Build configuration reads metadata without loading the UI's JSX runtime.
export const pageMetadata = [
  {
    path: "/",
    title: SITE_TITLE,
    description:
      "On-site AI toolchain assessments for credit unions and small/midsize businesses. Map tools, ownership, data access, and controls before a fit-based PRX pilot.",
  },
  {
    path: "/assessment/",
    title: "Assess your AI toolchain — PRX by Nebuless",
    description:
      "Review coding agents, automations, and everyday assistants through a human-led on-site assessment. Agree access boundaries, control priorities, and PRX fit.",
  },
  {
    path: "/how-it-works/",
    title: "How it works — PRX by Nebuless",
    description:
      "See how PRX checks agent requests, injects credentials, and holds risky actions for approval before they reach your services.",
  },
  {
    path: "/security/",
    title: "Security — PRX by Nebuless",
    description:
      "Review PRX security controls, credential isolation, data handling, telemetry, and your responsibilities as a self-hosted gateway operator.",
  },
  {
    path: "/deployment/",
    title: "Deployment — PRX by Nebuless",
    description:
      "Deploy PRX gateway services on Linux or macOS. Connect supported agent workflows over WireGuard or Tailscale, and manage backups and upgrades.",
  },
  {
    path: "/pilot/",
    title: "A fit-based PRX pilot — PRX by Nebuless",
    description:
      "After assessment, test one supported workflow with a scoped PRX pilot. Agree admins, policy, approvals, request handling, and success checks.",
  },
] as const;

export function normalizePagePath(path: string) {
  return path.replace(/\/index\.html$/, "/").replace(/\/?$/, "/");
}

export function pageMetadataForPath(path: string) {
  return pageMetadata.find((page) => page.path === normalizePagePath(path));
}
