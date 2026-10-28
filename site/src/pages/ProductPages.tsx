import type { ComponentChildren } from "preact";
import { Layout } from "../Layout";
import { Button } from "../components/Button";
import { FlowDiagram } from "../components/FlowDiagram";
import { InstallTerminal } from "../components/InstallTerminal";
import { SectionLabel } from "../components/SectionLabel";
import { DotField } from "../components/DotField";

function Intro({ label, title, children }: { label: string; title: string; children: ComponentChildren }) {
  return <section class="prx-hero product-intro"><div class="prx-glass-panel">
    <SectionLabel>{label}</SectionLabel><h1>{title}</h1><p>{children}</p>
  </div></section>;
}
function Section({ label, children, dark = false, id }: { label: string; children: ComponentChildren; dark?: boolean; id?: string }) {
  return <section id={id} class={dark ? "product-section bg-navy-700 text-canvas" : "product-section"}>
    <div class="product-inner"><SectionLabel>{label}</SectionLabel>{children}</div>
  </section>;
}
function Item({ title, children }: { title: string; children: ComponentChildren }) {
  return <div class="product-item"><h3>{title}</h3><div>{children}</div></div>;
}
function PilotLink() {
  return <div class="product-next"><Button href="/pilot/" size="lg">Book a pilot</Button><a class="prx-link-cta" href="/docs/">Read the docs</a></div>;
}

export function HowItWorks() {
  return <Layout>
    <Intro label="How it works" title="One gateway. A clear boundary.">Your agents use the tools they need. PRX checks the request and adds the service credentials on the way out.</Intro>
    <Section label="The request path"><div class="product-split">
      <div><h2>Credentials stay at the gateway.</h2><p>Route agent traffic through PRX. Policy runs before a request reaches the service. Credentials are injected at PRX, not placed in the agent's environment.</p><p>Assign profiles to each device to control which credentials it can use.</p><a class="prx-link-cta" href="/docs/credentials/">Credential scoping</a></div>
      <FlowDiagram />
    </div></Section>
    <Section label="Three outcomes" dark><div class="product-grid">
      <Item title="Allow"><p>PRX adds the scoped credentials and forwards the request.</p></Item>
      <Item title="Deny"><p>The request stops at PRX. The service does not receive it.</p></Item>
      <Item title="Hold for approval"><p>The request waits for a decision. Approval lets it proceed. Denial or expiry stops it.</p></Item>
    </div><a class="product-doc-link" href="/docs/rules/">Write your rules</a></Section>
    <Section label="Who decides"><div class="product-grid product-grid-two">
      <Item title="A person, when it matters"><p>Review pending requests in the dashboard or through Slack. Decide before the action reaches the service.</p></Item>
      <Item title="An optional AI judge"><p>Use an Anthropic or OpenAI model to assess a request against your policy. Keep people in the approval path where you need them.</p><a class="prx-link-cta" href="/security/#data-handling">What the judge receives</a></Item>
    </div></Section>
    <DotField />
    <Section label="Connect your agents"><div class="product-grid product-grid-two">
      <Item title="One command or a whole machine"><p>PRX runs on Linux, macOS, and Windows.</p><p>Wrap a command with <code>nebprx run</code>, or choose whole-machine routing. Connect over WireGuard or Tailscale.</p><a class="prx-link-cta" href="/deployment/">Choose your deployment</a></Item>
      <Item title="The traffic your tools use"><p>Apply policy to HTTPS APIs, Postgres, ClickHouse native, Kubernetes requests, and SSH channel requests.</p><a class="prx-link-cta" href="/docs/architecture/">See the architecture</a></Item>
    </div></Section>
    <Section label="What PRX doesn't do" dark><h2>A gateway, not an agent sandbox.</h2><p>Protect the gateway host. Block direct routes to protected systems. PRX does not isolate the agent's local files, environment, or operating-system privileges.</p><a class="product-doc-link" href="/docs/security-model/">Read the security model</a></Section>
    <PilotLink />
  </Layout>;
}

export function Security() {
  return <Layout>
    <Intro label="Security" title="Know the boundary. Own the controls.">PRX runs on infrastructure you control. Review how it handles credentials, requests, and the systems it connects to.</Intro>
    <Section label="Assurance"><p>PRX runs as a server/service on Linux, macOS, and Windows.</p><p class="product-assurance"><span class="font-mono text-sm">2026-10-26</span><br />No SOC 2 report or independent penetration test is available yet.</p><p>PRX is MIT licensed. Review the <a href="https://github.com/Nebuless/clawpatrol">source</a> and <a href="/docs/security-model/">security model</a> against your requirements.</p></Section>
    <Section label="Controls" dark><div class="product-grid product-grid-two">
      <Item title="Credential isolation"><p>Credentials are held by PRX. Injection is scoped to a device's assigned profiles.</p><a href="/docs/credentials/">Credentials and profiles</a></Item>
      <Item title="TLS inspection"><p>Agents trust a gateway-held CA. PRX terminates inspected HTTPS traffic and opens a separate connection to the service.</p><a href="/docs/security-model/">CA trust and inspection</a></Item>
      <Item title="Dashboard access"><p>Bcrypt password hashing, server-side sessions, and CSRF checks protect operator access. Restrict dashboard access and protect its transport.</p><a href="/docs/configure-gateway/">Dashboard configuration</a></Item>
      <Item title="Plugin sandboxing"><p>External plugins run with OS sandboxing by default. Review grants carefully. Privileged plugins and disabled sandboxes require full trust.</p><a href="/docs/plugins/">Plugin permissions</a></Item>
    </div></Section>
    <Section label="Harden the host"><div class="product-grid">
      <Item title="Separate the gateway"><p>Run PRX on its own host. Keep agents away from its state, binary, and administrator access.</p></Item>
      <Item title="Restrict state access"><p>Stored secrets are protected by file permissions, not application-level encryption. Limit access to the state directory.</p></Item>
      <Item title="Encrypt disks and backups"><p>The state includes credentials and private keys. Use disk and backup encryption and restrict who can restore it.</p></Item>
    </div><a class="prx-link-cta" href="/docs/security-model/">Hardening guidance</a></Section>
    <DotField />
    <Section label="Data handling" id="data-handling"><h2>What stays. What leaves.</h2>
      <div class="product-data">
        <Item title="On your gateway"><p>PRX stores request records, sampled request and response bodies, redacted headers, credentials, and gateway keys. Request retention defaults to 30 days and is configurable. Credentials and keys remain until removed.</p><p>Body capture is sampled and bounded. Bodies can contain sensitive data. Header redaction is not general personal-data filtering.</p><a href="/docs/config-reference/">Retention and capture settings</a></Item>
        <Item title="Services you connect"><p>Requests go to the services your agents call. OAuth and Tailscale also contact their providers when used. Slack and Signal notifications send request context to those services.</p></Item>
        <Item title="The optional AI judge"><p>The configured provider receives your policy, method, host, path, user-agent, and up to 4,000 characters of body. Review provider terms before sending sensitive requests.</p><a href="/docs/rules/">Approval configuration</a></Item>
        <Item title="Optional OpenTelemetry export"><p>Export goes to the collector you configure. GenAI telemetry is a separate opt-in. Message-content export is off by default and can include prompts and completions when enabled.</p><a href="/docs/config-reference/">Export settings</a></Item>
        <Item title="Release and plugin checks"><p>The gateway checks for available releases. Plugin update checks contact GitHub independently of usage telemetry. Updates are operator-initiated.</p><a href="/docs/plugins/">Plugin updates</a></Item>
        <Item title="Usage telemetry"><p>Released gateways send usage telemetry to <code>prx.nebuless.cloud</code>: a persistent instance ID, version and git SHA, OS, architecture, Go version, uptime, device count, transport, and recent action and byte totals.</p><p>No request paths, hostnames, bodies, headers, rules, profile names, emails, or credentials are included. The persistent ID links reports over time. The receiving service sees the connection's source IP.</p><p>Disable gateway usage and release checks with <code>telemetry = false</code>, <code>NEBPRX_TELEMETRY=0</code>, or <code>DO_NOT_TRACK=1</code> before starting the gateway.</p><p>The installer reports a per-run install ID, outcome, OS, architecture, requested version, source-build status, and failure reason. Disable installer reporting separately with <code>NEBPRX_NO_TELEMETRY=1</code>.</p></Item>
      </div>
    </Section>
    <Section label="Your responsibilities" dark><h2>Keep the boundary intact.</h2><p>Secure the host and its backups. Limit service credentials. Review policy, approval providers, and captured data. Restrict direct access to protected services and test your rules before rollout.</p><p>PRX does not replace your host security, network controls, or operational review.</p></Section>
    <Section label="Report a vulnerability"><h2>Use private reporting.</h2><p>Follow the repository's security policy for supported versions and reporting instructions.</p><a class="prx-link-cta" href="https://github.com/Nebuless/clawpatrol/blob/main/SECURITY.md">Security reporting policy</a></Section>
  </Layout>;
}

export function Deployment() {
  return <Layout>
    <Intro label="Deployment" title="Your network. Your gateway.">Start with one host and one team's agents. Keep the credentials and policy under your control.</Intro>
    <Section label="Choose the boundary"><div class="product-grid product-grid-two">
      <Item title="A separate gateway host"><p>Recommended. Run PRX on a host the agents cannot administer. Route their traffic through it over WireGuard or Tailscale.</p></Item>
      <Item title="On the same machine"><p>Use separate UNIX users. Agents must not have root-equivalent access or passwordless sudo. Protect the gateway's state directory and binary from the agent user.</p></Item>
    </div><a class="prx-link-cta" href="/docs/security-model/">Local-mode requirements</a></Section>
    <Section label="What you need" dark><div class="product-grid">
      <Item title="A supported host"><p>Linux, macOS, and Windows.</p></Item>
      <Item title="A network path"><p>A tailnet or WireGuard connection between agents and the gateway. Configure tailnet ACLs and exit-node access when using Tailscale.</p></Item>
      <Item title="The right ports"><p>Allow the configured WireGuard UDP listener when using WireGuard. Limit dashboard access to operators. Give the gateway access to the services it protects.</p><a href="/docs/configure-gateway/">Listener and port configuration</a></Item>
    </div></Section>
    <Section label="Install"><h2>Start with the CLI.</h2><div class="product-install"><InstallTerminal variant="expanded" /></div><p>Configure the gateway, add credentials and profiles, then onboard an agent device. Onboarding establishes the connection and CA trust needed for HTTPS inspection.</p><a class="prx-link-cta" href="/docs/getting-started/">Installation and onboarding</a></Section>
    <DotField />
    <Section label="Route the work"><div class="product-grid product-grid-two">
      <Item title="Wrap one command"><p>Use <code>nebprx run</code> for an agent process and its traffic. Linux uses network namespaces. macOS client capture is handled by PRX on macOS.</p></Item>
      <Item title="Route a whole machine"><p>Choose whole-machine mode over WireGuard or Tailscale. Configure routing explicitly rather than assuming a joined device routes everything.</p></Item>
    </div><a class="prx-link-cta" href="/docs/architecture/">Transport and routing details</a></Section>
    <Section label="Operate it" dark><div class="product-grid product-grid-two">
      <Item title="Back up the state"><p>Back up the configured state directory, including the database and gateway configuration. Use a consistent database backup or stop the gateway during capture. Encrypt backups and restrict restore access.</p></Item>
      <Item title="Upgrade on your terms"><p>The gateway tells you when a release is available. You choose when to upgrade. Back up state before changing versions.</p><a href="/docs/cli/">CLI reference</a></Item>
    </div><p class="product-callout">PRX uses a single gateway. There is no built-in high availability.</p></Section>
    <PilotLink />
  </Layout>;
}

export function Pilot() {
  return <Layout>
    <Intro label="Pilot" title="One team. One gateway. Real work.">Put PRX inside your network with one team's agents. Test the boundary on the services and actions that matter to you.</Intro>
    <Section label="Start small"><div class="product-grid product-grid-two">
      <Item title="What you provide"><p>A gateway host, a network path for your agents, scoped service credentials, and someone who can review policy and approvals. Choose the team's first workflow.</p></Item>
      <Item title="What we provide"><p>Help with gateway setup, the first policy, and approval routing. Work through the results together and identify what needs to change before expanding.</p></Item>
    </div></Section>
    <Section label="What success looks like" dark><ol class="product-checks">
      <li>Agents work without seeing the service credentials.</li>
      <li>A risky request is blocked before it reaches the service.</li>
      <li>A request is held, reviewed, and approved.</li>
      <li>A recorded request is downloaded for review.</li>
    </ol></Section>
    <Section label="The next step"><h2>Bring one workflow.</h2><p>Tell us which agents you use, which services they need, and which actions should wait for a person. Agree on scope and timing together.</p><div class="product-next product-next-inline"><Button href="mailto:pilot@nebuless.cloud" size="lg">Book a pilot</Button><a class="prx-link-cta" href="/deployment/">Review deployment needs</a></div><p class="font-mono text-sm">pilot@nebuless.cloud</p></Section>
  </Layout>;
}
