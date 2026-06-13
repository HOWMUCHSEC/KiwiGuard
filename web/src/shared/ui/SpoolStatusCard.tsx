import { Tag } from "@carbon/react";

import type { SpoolStatusSnapshot } from "shared/types/spool";
import { formatAge, formatBytes } from "./format";
import { spoolLabel, spoolState, spoolTagType } from "./status";

type Translate = (key: string, values?: Record<string, string | number>) => string;

type SpoolStatusCardProps = {
  isError: boolean;
  isLoading: boolean;
  spool?: SpoolStatusSnapshot;
  t: Translate;
  title: string;
};

export function SpoolStatusCard({ isError, isLoading, spool, t, title }: SpoolStatusCardProps) {
  return (
    <div className="spool-status-card" data-state={spoolState(spool?.status)}>
      <div className="kg-subsection-header">
        <p className="kg-kicker">{title}</p>
        <Tag type={spoolTagType(spool?.status)}>{isLoading ? t("common.loading") : isError ? t("common.unavailable") : t(spoolLabel(spool?.status))}</Tag>
      </div>
      <dl>
        <div>
          <dt>{t("runtime.depth")}</dt>
          <dd>{spool?.depth ?? 0}</dd>
        </div>
        <div>
          <dt>{t("runtime.storage")}</dt>
          <dd>
            {formatBytes(spool?.bytes ?? 0)} / {formatBytes(spool?.max_bytes ?? 0)}
          </dd>
        </div>
        <div>
          <dt>{t("runtime.oldest")}</dt>
          <dd>{formatAge(spool?.oldest_age_seconds ?? 0, t("common.none"))}</dd>
        </div>
        <div>
          <dt>{t("runtime.overflow")}</dt>
          <dd>{spool?.overflow_count ?? 0}</dd>
        </div>
      </dl>
      {spool?.reason ? <code>{spool.reason}</code> : null}
    </div>
  );
}
