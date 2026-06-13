import {
  consoleDomains,
  consolePageDefinition,
  consolePageDefinitions,
  consolePageKeys,
  defaultDestination,
  domainTabs,
  firstTabForDomain,
  hasDomain,
  hasTab,
  manifestPageKey,
  type ConsoleDestination,
  type ConsoleDomain,
  type ConsolePageKey,
  type ConsoleTab
} from "./manifest";
import { consoleMetricDefinition, consoleMetricDefinitions, consoleMetricDestinations, type ConsoleMetricDefinition } from "./metrics";

export {
  consoleDomains,
  consoleMetricDefinition,
  consoleMetricDefinitions,
  consoleMetricDestinations as metricDestinations,
  consolePageDefinition,
  consolePageDefinitions,
  consolePageKeys,
  defaultDestination,
  domainTabs,
  type ConsoleDestination,
  type ConsoleDomain,
  type ConsoleMetricDefinition,
  type ConsolePageKey,
  type ConsoleTab
};
export type { ConsoleMetricKey } from "./manifest";

export function consolePageKey(destination: ConsoleDestination): ConsolePageKey {
  return manifestPageKey(destination);
}

export function destinationFromHash(hash: string): ConsoleDestination {
  const clean = hash.replace(/^#\/?/, "").replace(/\?.*$/, "");
  const [domainValue, tabValue] = clean.split("/");
  if (!hasDomain(domainValue)) return defaultDestination;
  if (!hasTab(domainValue, tabValue)) return { domain: domainValue, tab: firstTabForDomain(domainValue) };
  return { domain: domainValue, tab: tabValue };
}

export function destinationHash(destination: ConsoleDestination) {
  return `#/${destination.domain}/${destination.tab}`;
}
