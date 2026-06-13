import { Button, Tag, Tile } from "@carbon/react";
import { MonitorUp, ShieldCheck } from "lucide-react";

import type { ConsolePostureSummary } from "shared/types/console";
import type { PageHeading } from "shared/ui/PageHeading";
import { PanelHeader } from "shared/ui/PanelHeader";
import { healthLabelKey, healthTagType } from "shared/ui/status";
import { TrafficSummary } from "shared/ui/TrafficSummary";

type DashboardPostureSectionProps = {
  heading: PageHeading;
  onOpenTraffic: () => void;
  summary: ConsolePostureSummary;
  t: (key: string) => string;
};

export function DashboardPostureSection({ heading, onOpenTraffic, summary, t }: DashboardPostureSectionProps) {
  const resolvedSpoolDepth = summary.spoolDepth === null ? t("common.unknown") : String(summary.spoolDepth);

  return (
    <>
      <Tile className="kg-panel span-3 kg-overview-panel">
        <PanelHeader kicker={heading.kicker} title={heading.title} icon={<ShieldCheck aria-hidden="true" />} />
        <div className="kg-panel-body kg-dashboard-section-body">
          <div className="overview-grid">
            <div className="overview-card">
              <span>{t("dashboard.apiStatus")}</span>
              <Tag type={summary.healthIsError ? "red" : healthTagType(summary.healthState)}>
                {summary.healthIsLoading ? t("health.checking") : summary.healthIsError ? t("health.unavailable") : t(healthLabelKey(summary.healthState))}
              </Tag>
            </div>
            <div className="overview-card">
              <span>{t("dashboard.activePolicies")}</span>
              <strong>{summary.activeKeysCount}</strong>
            </div>
            <div className="overview-card">
              <span>{t("dashboard.policyBundles")}</span>
              <strong>{summary.policyBundleCount}</strong>
            </div>
            <div className="overview-card">
              <span>{t("dashboard.routeMappings")}</span>
              <strong>{summary.mappingCount}</strong>
            </div>
            <div className="overview-card">
              <span>{t("dashboard.verdictProviders")}</span>
              <strong>{summary.providerCount}</strong>
            </div>
            <div className="overview-card">
              <span>{t("dashboard.spoolDepth")}</span>
              <strong>{resolvedSpoolDepth}</strong>
            </div>
          </div>
        </div>
      </Tile>

      <Tile className="kg-panel span-2 kg-dashboard-traffic-panel">
        <PanelHeader kicker={t("dashboard.trafficKicker")} title={t("dashboard.trafficTitle")} icon={<MonitorUp aria-hidden="true" />} />
        <div className="kg-panel-body kg-dashboard-section-body kg-dashboard-traffic-panel__body">
          <TrafficSummary
            labels={{
              aria: t("traffic.summaryAria"),
              blocked: t("traffic.blocked"),
              fallbacks: t("traffic.fallbacks"),
              total: t("traffic.total"),
              upstreamErrors: t("traffic.upstreamErrors")
            }}
            summary={summary.trafficSummary}
          />
          <Button size="sm" onClick={onOpenTraffic}>
            {t("dashboard.openTraffic")}
          </Button>
        </div>
      </Tile>
    </>
  );
}
