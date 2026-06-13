import { Tag } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { limitSummary, type EffectiveLimit } from "../model/accessLimitUtils";

export function EffectiveLimitList({ items }: { items: EffectiveLimit[] }) {
  const { t } = useI18n();

  return (
    <div className="effective-limits">
      <h4>{t("access.effectiveLimits")}</h4>
      <div className="compact-list">
        {items.length === 0 ? (
          <p className="kg-muted">{t("access.effectiveLimitsEmpty")}</p>
        ) : (
          items.map((item) => (
            <div className="access-row" key={item.route_key}>
              <div className="kg-stack">
                <strong>{item.route_key}</strong>
                <small>{limitSummary(item.limit, t)}</small>
              </div>
              <Tag type={item.source === "override" ? "blue" : "gray"}>{item.source === "override" ? t("access.override") : t("access.default")}</Tag>
              <code>{item.limit.enabled ? t("common.enabled") : t("common.disabled")}</code>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
