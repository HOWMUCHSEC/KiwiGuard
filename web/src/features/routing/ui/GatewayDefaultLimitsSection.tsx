import { FormEvent } from "react";
import { ShieldCheck } from "lucide-react";

import { useI18n } from "platform/i18n";
import { AccessSectionHeading } from "./AccessSectionHeading";
import { LimitEditor } from "./LimitEditor";
import { RouteLimitList } from "./RouteLimitList";
import type { GatewayRouteLimit } from "../model/routingApi";

type GatewayDefaultLimitsSectionProps = {
  routeLimitDraft: GatewayRouteLimit;
  routeLimits: GatewayRouteLimit[];
  isSavingRouteLimit: boolean;
  onRouteLimitDraftChange: (limit: GatewayRouteLimit) => void;
  onSaveRouteLimit: (event: FormEvent<HTMLFormElement>) => void;
};

export function GatewayDefaultLimitsSection({
  routeLimitDraft,
  routeLimits,
  isSavingRouteLimit,
  onRouteLimitDraftChange,
  onSaveRouteLimit
}: GatewayDefaultLimitsSectionProps) {
  const { t } = useI18n();

  return (
    <section className="access-section">
      <AccessSectionHeading title={t("access.defaultLimits")} icon={ShieldCheck} />
      <LimitEditor
        idPrefix="default-route-limit"
        limit={routeLimitDraft}
        isPending={isSavingRouteLimit}
        saveLabel={t("access.saveRouteLimit")}
        savingLabel={t("routing.saving")}
        onChange={onRouteLimitDraftChange}
        onSubmit={onSaveRouteLimit}
      />
      <RouteLimitList items={routeLimits} empty={t("access.defaultLimitsEmpty")} onEdit={onRouteLimitDraftChange} />
    </section>
  );
}
