import {
  Database,
  ListChecks,
  MonitorUp,
  RadioTower,
  Route,
  ShieldCheck,
  type LucideIcon
} from "lucide-react";

import type { ConsolePostureSummary } from "shared/types/console";
import type { ConsoleDestination, ConsoleMetricKey } from "./manifest";

export type ConsoleMetricDefinition = {
  key: ConsoleMetricKey;
  labelKey: `metric.${ConsoleMetricKey}`;
  destination: ConsoleDestination;
  icon: LucideIcon;
  selectValue: (summary: ConsolePostureSummary, unknownLabel: string) => string;
};

export const consoleMetricDefinitions = [
  {
    key: "bundles",
    labelKey: "metric.bundles",
    destination: { domain: "policies", tab: "rule-library" },
    icon: ShieldCheck,
    selectValue: (summary) => String(summary.policyBundleCount)
  },
  {
    key: "active",
    labelKey: "metric.active",
    destination: { domain: "policies", tab: "activation" },
    icon: ListChecks,
    selectValue: (summary) => String(summary.activeKeysCount)
  },
  {
    key: "routes",
    labelKey: "metric.routes",
    destination: { domain: "routing", tab: "route-mapping" },
    icon: Route,
    selectValue: (summary) => String(summary.mappingCount)
  },
  {
    key: "providers",
    labelKey: "metric.providers",
    destination: { domain: "routing", tab: "providers" },
    icon: RadioTower,
    selectValue: (summary) => String(summary.providerCount)
  },
  {
    key: "events",
    labelKey: "metric.events",
    destination: { domain: "traffic", tab: "events" },
    icon: MonitorUp,
    selectValue: (summary) => String(summary.trafficSummary.total)
  },
  {
    key: "spool",
    labelKey: "metric.spool",
    destination: { domain: "storage", tab: "spool" },
    icon: Database,
    selectValue: (summary, unknownLabel) => (summary.spoolDepth === null ? unknownLabel : String(summary.spoolDepth))
  }
] as const satisfies readonly ConsoleMetricDefinition[];

export function consoleMetricDefinition(metricKey: ConsoleMetricKey): ConsoleMetricDefinition {
  const definition = consoleMetricDefinitions.find((entry) => entry.key === metricKey);
  if (!definition) throw new Error(`Unknown console metric definition: ${metricKey}`);
  return definition;
}

export const consoleMetricDestinations = Object.fromEntries(
  consoleMetricDefinitions.map((definition) => [definition.key, definition.destination])
) as Record<ConsoleMetricKey, ConsoleDestination>;
