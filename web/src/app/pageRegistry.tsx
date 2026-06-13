import type { ReactNode } from "react";

import {
  consolePageDefinition,
  consolePageKey,
  consolePageKeys,
  metricDestinations,
  type ConsoleDestination,
  type ConsolePageKey
} from "console/routes";
import { DashboardOverviewPage } from "features/dashboard/public";
import { PoliciesPage, type PoliciesTab } from "features/policies/public";
import { RoutingPage } from "features/routing/public";
import { StoragePage } from "features/storage/public";
import { TrafficEventsPage } from "features/traffic/public";
import type { ConsolePostureSummary } from "shared/types/console";
import type { PageHeading } from "shared/ui/PageHeading";

type RenderPageContext = {
  destination: ConsoleDestination;
  navigate: (destination: ConsoleDestination) => void;
  summary: ConsolePostureSummary;
  t: (key: string) => string;
};

type PageRenderer = (context: RenderPageContext) => ReactNode;

const routingSection = (destination: ConsoleDestination): "route-mapping" | "access-limits" | "providers" =>
  destination.tab === "providers" || destination.tab === "access-limits" ? destination.tab : "route-mapping";

const policiesTab = (destination: ConsoleDestination): PoliciesTab => (destination.tab === "activation" ? "activation" : "rule-library");

const pageHeading = (context: RenderPageContext, destination: ConsoleDestination = context.destination): PageHeading => {
  const definition = consolePageDefinition(destination);
  return {
    kicker: context.t(definition.kickerKey),
    title: context.t(definition.titleKey)
  };
};

const routingPageProps = (context: RenderPageContext) => ({
  accessHeading: pageHeading(context, { domain: "routing", tab: "access-limits" }),
  activeSection: routingSection(context.destination),
  mappingHeading: pageHeading(context, { domain: "routing", tab: "route-mapping" }),
  providerHeading: pageHeading(context, { domain: "routing", tab: "providers" })
});

const pageRegistry = {
  [consolePageKeys.dashboardOverview]: (context) => (
    <DashboardOverviewPage heading={pageHeading(context)} onOpenTraffic={() => context.navigate(metricDestinations.events)} summary={context.summary} t={context.t} />
  ),
  [consolePageKeys.trafficEvents]: (context) => <TrafficEventsPage heading={pageHeading(context)} t={context.t} />,
  [consolePageKeys.policyRuleLibrary]: (context) => <PoliciesPage heading={pageHeading(context)} tab={policiesTab(context.destination)} />,
  [consolePageKeys.policyActivation]: (context) => <PoliciesPage heading={pageHeading(context)} tab={policiesTab(context.destination)} />,
  [consolePageKeys.routingRouteMapping]: (context) => <RoutingPage {...routingPageProps(context)} />,
  [consolePageKeys.routingAccessLimits]: (context) => <RoutingPage {...routingPageProps(context)} />,
  [consolePageKeys.routingProviders]: (context) => <RoutingPage {...routingPageProps(context)} />,
  [consolePageKeys.storageSpool]: (context) => <StoragePage heading={pageHeading(context)} />
} satisfies Record<ConsolePageKey, PageRenderer>;

export function renderConsolePage(context: RenderPageContext) {
  return pageRegistry[consolePageKey(context.destination)](context);
}
