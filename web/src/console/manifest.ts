export type ConsoleDomain = "dashboard" | "traffic" | "policies" | "routing" | "storage";

export type ConsoleTab =
  | "overview"
  | "events"
  | "rule-library"
  | "activation"
  | "route-mapping"
  | "access-limits"
  | "providers"
  | "spool";

export type ConsolePageKey =
  | "dashboard.overview"
  | "traffic.events"
  | "policies.rule-library"
  | "policies.activation"
  | "routing.route-mapping"
  | "routing.access-limits"
  | "routing.providers"
  | "storage.spool";

export type ConsoleMetricKey = "bundles" | "active" | "routes" | "providers" | "events" | "spool";
export type ConsoleLabelKey = `nav.${ConsoleDomain}` | `nav.${ConsoleTab}` | `metric.${ConsoleMetricKey}`;
export type ConsolePageTitleKey =
  | "dashboard.title"
  | "traffic.title"
  | "policy.title"
  | "runtime.title"
  | "routing.title"
  | "access.title"
  | "provider.title"
  | "runtime.eventSpool";
export type ConsolePageKickerKey =
  | "dashboard.kicker"
  | "traffic.kicker"
  | "policy.kicker"
  | "runtime.kicker"
  | "routing.kicker"
  | "access.kicker"
  | "provider.kicker"
  | "nav.storage";

export type ConsoleDestination = {
  domain: ConsoleDomain;
  tab: ConsoleTab;
};

type ConsoleTabDefinition = {
  tab: ConsoleTab;
  labelKey: `nav.${ConsoleTab}`;
  pageKey: ConsolePageKey;
};

type ConsoleDomainDefinition = {
  domain: ConsoleDomain;
  labelKey: `nav.${ConsoleDomain}`;
  tabs: readonly [ConsoleTabDefinition, ...ConsoleTabDefinition[]];
};

export type ConsolePageDefinition = {
  domain: ConsoleDomain;
  tab: ConsoleTab;
  pageKey: ConsolePageKey;
  navLabelKey: `nav.${ConsoleTab}`;
  titleKey: ConsolePageTitleKey;
  kickerKey: ConsolePageKickerKey;
};

export const consoleManifest = [
  {
    domain: "dashboard",
    labelKey: "nav.dashboard",
    tabs: [{ tab: "overview", labelKey: "nav.overview", pageKey: "dashboard.overview" }]
  },
  {
    domain: "traffic",
    labelKey: "nav.traffic",
    tabs: [{ tab: "events", labelKey: "nav.events", pageKey: "traffic.events" }]
  },
  {
    domain: "policies",
    labelKey: "nav.policies",
    tabs: [
      { tab: "rule-library", labelKey: "nav.rule-library", pageKey: "policies.rule-library" },
      { tab: "activation", labelKey: "nav.activation", pageKey: "policies.activation" }
    ]
  },
  {
    domain: "routing",
    labelKey: "nav.routing",
    tabs: [
      { tab: "route-mapping", labelKey: "nav.route-mapping", pageKey: "routing.route-mapping" },
      { tab: "access-limits", labelKey: "nav.access-limits", pageKey: "routing.access-limits" },
      { tab: "providers", labelKey: "nav.providers", pageKey: "routing.providers" }
    ]
  },
  {
    domain: "storage",
    labelKey: "nav.storage",
    tabs: [{ tab: "spool", labelKey: "nav.spool", pageKey: "storage.spool" }]
  }
] as const satisfies readonly [ConsoleDomainDefinition, ...ConsoleDomainDefinition[]];

export const consolePageKeys = {
  dashboardOverview: "dashboard.overview",
  trafficEvents: "traffic.events",
  policyRuleLibrary: "policies.rule-library",
  policyActivation: "policies.activation",
  routingRouteMapping: "routing.route-mapping",
  routingAccessLimits: "routing.access-limits",
  routingProviders: "routing.providers",
  storageSpool: "storage.spool"
} as const satisfies Record<string, ConsolePageKey>;

export const consolePageDefinitions = [
  {
    domain: "dashboard",
    tab: "overview",
    pageKey: "dashboard.overview",
    navLabelKey: "nav.overview",
    titleKey: "dashboard.title",
    kickerKey: "dashboard.kicker"
  },
  {
    domain: "traffic",
    tab: "events",
    pageKey: "traffic.events",
    navLabelKey: "nav.events",
    titleKey: "traffic.title",
    kickerKey: "traffic.kicker"
  },
  {
    domain: "policies",
    tab: "rule-library",
    pageKey: "policies.rule-library",
    navLabelKey: "nav.rule-library",
    titleKey: "policy.title",
    kickerKey: "policy.kicker"
  },
  {
    domain: "policies",
    tab: "activation",
    pageKey: "policies.activation",
    navLabelKey: "nav.activation",
    titleKey: "runtime.title",
    kickerKey: "runtime.kicker"
  },
  {
    domain: "routing",
    tab: "route-mapping",
    pageKey: "routing.route-mapping",
    navLabelKey: "nav.route-mapping",
    titleKey: "routing.title",
    kickerKey: "routing.kicker"
  },
  {
    domain: "routing",
    tab: "access-limits",
    pageKey: "routing.access-limits",
    navLabelKey: "nav.access-limits",
    titleKey: "access.title",
    kickerKey: "access.kicker"
  },
  {
    domain: "routing",
    tab: "providers",
    pageKey: "routing.providers",
    navLabelKey: "nav.providers",
    titleKey: "provider.title",
    kickerKey: "provider.kicker"
  },
  {
    domain: "storage",
    tab: "spool",
    pageKey: "storage.spool",
    navLabelKey: "nav.spool",
    titleKey: "runtime.eventSpool",
    kickerKey: "nav.storage"
  }
] as const satisfies readonly ConsolePageDefinition[];

export const defaultDestination: ConsoleDestination = manifestDestination(consoleManifest[0].domain, consoleManifest[0].tabs[0].tab);

export const consoleDomains = consoleManifest;

export function domainTabs(domain: ConsoleDomain) {
  return findDomain(domain).tabs;
}

export function manifestPageKey(destination: ConsoleDestination): ConsolePageKey {
  return findTab(destination.domain, destination.tab).pageKey;
}

export function consolePageDefinition(destination: ConsoleDestination): ConsolePageDefinition {
  const definition = consolePageDefinitions.find((entry) => entry.domain === destination.domain && entry.tab === destination.tab);
  if (!definition) throw new Error(`Unknown console page definition: ${destination.domain}/${destination.tab}`);
  return definition;
}

export function firstTabForDomain(domain: ConsoleDomain): ConsoleTab {
  return findDomain(domain).tabs[0].tab;
}

export function hasDomain(value: string | undefined): value is ConsoleDomain {
  return consoleManifest.some((entry) => entry.domain === value);
}

export function hasTab(domain: ConsoleDomain, tab: string | undefined): tab is ConsoleTab {
  return domainTabs(domain).some((entry) => entry.tab === tab);
}

function findDomain(domain: ConsoleDomain) {
  const entry = consoleManifest.find((candidate) => candidate.domain === domain);
  if (!entry) throw new Error(`Unknown console domain: ${domain}`);
  return entry;
}

function findTab(domain: ConsoleDomain, tab: ConsoleTab) {
  const entry = findDomain(domain).tabs.find((candidate) => candidate.tab === tab);
  if (!entry) throw new Error(`Unknown console tab: ${domain}/${tab}`);
  return entry;
}

function manifestDestination(domain: ConsoleDomain, tab: ConsoleTab): ConsoleDestination {
  return { domain, tab };
}
