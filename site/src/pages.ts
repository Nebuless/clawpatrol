import { Landing } from "./Landing";
import { Assessment } from "./pages/Assessment";
import { normalizePagePath, pageMetadata } from "./page-metadata";
import { HowItWorks, Security, Deployment, Pilot } from "./pages/ProductPages";

const components = {
  "/": Landing,
  "/assessment/": Assessment,
  "/how-it-works/": HowItWorks,
  "/security/": Security,
  "/deployment/": Deployment,
  "/pilot/": Pilot,
};

export const pages = pageMetadata.map((page) => ({ ...page, component: components[page.path] }));

export function pageForPath(path: string) {
  const normalized = normalizePagePath(path);
  return pages.find((page) => page.path === normalized);
}
