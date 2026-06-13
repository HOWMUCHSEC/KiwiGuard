import { Database } from "lucide-react";
import { Tile } from "@carbon/react";

import { useStoragePageData } from "../model/useStoragePageData";
import { useI18n } from "platform/i18n";
import { FactList } from "shared/ui/FactList";
import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";
import { SpoolStatusCard } from "shared/ui/SpoolStatusCard";

type StoragePageProps = {
  heading: PageHeading;
};

export function StoragePage({ heading }: StoragePageProps) {
  const { t } = useI18n();
  const pageData = useStoragePageData();
  const facts = [
    {
      label: t("policy.version"),
      value: pageData.version
    },
    {
      label: t("runtime.snapshot"),
      value: pageData.snapshotHash ?? t("common.none")
    }
  ];

  return (
    <Tile className="kg-panel">
      <PanelHeader kicker={heading.kicker} title={heading.title} icon={<Database aria-hidden="true" />} />
      <div className="kg-panel-body kg-stack">
        <FactList items={facts} />
        <SpoolStatusCard
          isError={pageData.spoolStatus.isError}
          isLoading={pageData.spoolStatus.isLoading}
          spool={pageData.spool}
          t={t}
          title={heading.title}
        />
      </div>
    </Tile>
  );
}
