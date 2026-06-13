import { Trash2 } from "lucide-react";
import { Button, Tag } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { limitSummary } from "../model/accessLimitUtils";
import type { GatewayRouteLimit } from "../model/routingApi";

export function RouteLimitList({
  items,
  empty,
  deleteIsPending,
  onDelete,
  onEdit
}: {
  items: GatewayRouteLimit[];
  empty: string;
  deleteIsPending?: boolean;
  onDelete?: (limit: GatewayRouteLimit) => void;
  onEdit: (limit: GatewayRouteLimit) => void;
}) {
  const { t } = useI18n();

  return (
    <div className="compact-list">
      {items.length === 0 ? (
        <p className="kg-muted">{empty}</p>
      ) : (
        items.map((limit) => (
          <div className="access-row" key={`${"client_id" in limit ? limit.client_id : "default"}-${limit.route_key}`}>
            <div className="kg-stack">
              <strong>{limit.route_key}</strong>
              <small>{limitSummary(limit, t)}</small>
            </div>
            <Tag type={limit.enabled ? "green" : "gray"}>{limit.enabled ? t("common.enabled") : t("common.disabled")}</Tag>
            <div className="kg-inline-actions">
              <Button size="sm" kind="ghost" onClick={() => onEdit(limit)}>
                {t("access.edit")}
              </Button>
              {onDelete ? (
                <Button size="sm" kind="danger--ghost" renderIcon={Trash2} disabled={deleteIsPending} onClick={() => onDelete(limit)}>
                  {t("access.delete")}
                </Button>
              ) : null}
            </div>
          </div>
        ))
      )}
    </div>
  );
}
