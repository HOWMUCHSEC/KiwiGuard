import { FormEvent } from "react";
import { Save } from "lucide-react";
import { Select, SelectItem } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { AccessSectionHeading } from "./AccessSectionHeading";
import { EffectiveLimitList } from "./EffectiveLimitList";
import { LimitEditor } from "./LimitEditor";
import { RouteLimitList } from "./RouteLimitList";
import type { EffectiveLimit } from "../model/accessLimitUtils";
import type { GatewayClient, GatewayClientRouteLimit } from "../model/routingApi";

type GatewayClientOverridesSectionProps = {
  clients: GatewayClient[];
  clientLimitDraft: GatewayClientRouteLimit;
  clientRouteLimits: GatewayClientRouteLimit[];
  effectiveLimits: EffectiveLimit[];
  isDeletingClientLimit: boolean;
  isSavingClientLimit: boolean;
  selectedClient: GatewayClient | undefined;
  selectedClientID: string;
  onClientLimitDraftChange: (limit: GatewayClientRouteLimit) => void;
  onDeleteClientLimit: (clientID: string, routeKey: string) => void;
  onSaveClientLimit: (event: FormEvent<HTMLFormElement>) => void;
  onSelectedClientChange: (clientID: string) => void;
};

export function GatewayClientOverridesSection({
  clients,
  clientLimitDraft,
  clientRouteLimits,
  effectiveLimits,
  isDeletingClientLimit,
  isSavingClientLimit,
  selectedClient,
  selectedClientID,
  onClientLimitDraftChange,
  onDeleteClientLimit,
  onSaveClientLimit,
  onSelectedClientChange
}: GatewayClientOverridesSectionProps) {
  const { t } = useI18n();

  return (
    <section className="access-section">
      <AccessSectionHeading title={t("access.clientOverrides")} icon={Save} />
      <Select id="client-limit-select" labelText={t("access.client")} value={selectedClientID} onChange={(event) => onSelectedClientChange(event.target.value)}>
        <SelectItem value="" text={t("access.selectClient")} />
        {clients.map((client) => (
          <SelectItem key={client.id} value={client.id} text={`${client.name} (${t(`access.status.${client.status}`)})`} />
        ))}
      </Select>
      {selectedClient ? (
        <>
          <LimitEditor
            idPrefix="client-route-limit"
            limit={clientLimitDraft}
            isPending={isSavingClientLimit}
            saveLabel={t("access.saveOverride")}
            savingLabel={t("routing.saving")}
            onChange={(limit) => onClientLimitDraftChange({ ...limit, client_id: selectedClient.id })}
            onSubmit={onSaveClientLimit}
          />
          <RouteLimitList
            items={clientRouteLimits}
            empty={t("access.overridesEmpty")}
            deleteIsPending={isDeletingClientLimit}
            onDelete={(limit) => onDeleteClientLimit((limit as GatewayClientRouteLimit).client_id, limit.route_key)}
            onEdit={(limit) => onClientLimitDraftChange({ ...(limit as GatewayClientRouteLimit), client_id: selectedClient.id })}
          />
          <EffectiveLimitList items={effectiveLimits} />
        </>
      ) : (
        <p className="kg-muted">{t("access.selectClientHelp")}</p>
      )}
    </section>
  );
}
