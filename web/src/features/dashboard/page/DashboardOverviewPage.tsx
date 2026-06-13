import type { ConsolePostureSummary } from "shared/types/console";
import type { PageHeading } from "shared/ui/PageHeading";
import { DashboardConfigSection } from "../ui/DashboardConfigSection";
import { DashboardPostureSection } from "../ui/DashboardPostureSection";
import { useDashboardOverviewData } from "../model/useDashboardOverviewData";
import "./dashboard.css";

type DashboardOverviewPageProps = {
  heading: PageHeading;
  onOpenTraffic: () => void;
  summary: ConsolePostureSummary;
  t: (key: string) => string;
};

export function DashboardOverviewPage({ heading, onOpenTraffic, summary, t }: DashboardOverviewPageProps) {
  const pageData = useDashboardOverviewData();

  return (
    <>
      <DashboardPostureSection heading={heading} onOpenTraffic={onOpenTraffic} summary={summary} t={t} />
      <DashboardConfigSection mappingCount={summary.mappingCount} providerCount={summary.providerCount} snapshotHash={pageData.snapshotHash} t={t} />
    </>
  );
}
