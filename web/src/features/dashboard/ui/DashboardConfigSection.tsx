import { Tile } from "@carbon/react";
import { Route } from "lucide-react";

import { FactList } from "shared/ui/FactList";
import { PanelHeader } from "shared/ui/PanelHeader";
import { shortHash } from "shared/ui/format";

type DashboardConfigSectionProps = {
  mappingCount: number;
  providerCount: number;
  snapshotHash?: string;
  t: (key: string) => string;
};

export function DashboardConfigSection({ mappingCount, providerCount, snapshotHash, t }: DashboardConfigSectionProps) {
  const items = [
    {
      label: t("runtime.snapshot"),
      value: snapshotHash ? shortHash(snapshotHash, t("common.none")) : t("common.none")
    },
    {
      label: t("dashboard.routeMappings"),
      value: mappingCount
    },
    {
      label: t("dashboard.verdictProviders"),
      value: providerCount
    }
  ];

  return (
    <Tile className="kg-panel kg-dashboard-config-panel">
      <PanelHeader kicker={t("dashboard.configKicker")} title={t("dashboard.configTitle")} icon={<Route aria-hidden="true" />} />
      <div className="kg-panel-body">
        <FactList items={items} divided />
      </div>
    </Tile>
  );
}
