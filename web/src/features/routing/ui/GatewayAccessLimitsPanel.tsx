import { FormEvent, useEffect, useMemo, useState } from "react";

import type { GatewayClient, GatewayClientRouteLimit, GatewayRouteLimit } from "../model/routingApi";
import { useI18n } from "platform/i18n";
import { AccessError } from "./AccessError";
import { buildEffectiveLimits, starterRouteLimit } from "../model/accessLimitUtils";
import { GatewayClientOverridesSection } from "./GatewayClientOverridesSection";
import { GatewayClientSection, type GatewayClientDraft } from "./GatewayClientSection";
import { GatewayDefaultLimitsSection } from "./GatewayDefaultLimitsSection";

export type GatewayAccessLimitsPanelProps = {
  clients: GatewayClient[];
  routeLimits: GatewayRouteLimit[];
  clientRouteLimits: GatewayClientRouteLimit[];
  selectedClientID: string;
  createdKey?: string;
  error?: unknown;
  isCreatingClient: boolean;
  isSavingClient: boolean;
  isRevokingClient: boolean;
  isSavingRouteLimit: boolean;
  isSavingClientLimit: boolean;
  isDeletingClientLimit: boolean;
  onClearCreatedKey: () => void;
  onSelectedClientChange: (clientID: string) => void;
  onCreateClient: (input: { name: string; notes?: string }) => void;
  onPatchClient: (client: GatewayClient) => void;
  onRevokeClient: (clientID: string) => void;
  onSaveRouteLimit: (limit: GatewayRouteLimit) => void;
  onSaveClientLimit: (limit: GatewayClientRouteLimit) => void;
  onDeleteClientLimit: (clientID: string, routeKey: string) => void;
};

export function GatewayAccessLimitsPanel({
  clients,
  routeLimits,
  clientRouteLimits,
  selectedClientID,
  createdKey,
  error,
  isCreatingClient,
  isSavingClient,
  isRevokingClient,
  isSavingRouteLimit,
  isSavingClientLimit,
  isDeletingClientLimit,
  onClearCreatedKey,
  onSelectedClientChange,
  onCreateClient,
  onPatchClient,
  onRevokeClient,
  onSaveRouteLimit,
  onSaveClientLimit,
  onDeleteClientLimit
}: GatewayAccessLimitsPanelProps) {
  const { t } = useI18n();
  const [clientDraft, setClientDraft] = useState<GatewayClientDraft>({ name: "", notes: "" });
  const [clientEdit, setClientEdit] = useState<GatewayClient | null>(null);
  const [routeLimitDraft, setRouteLimitDraft] = useState<GatewayRouteLimit>(starterRouteLimit);
  const [clientLimitDraft, setClientLimitDraft] = useState<GatewayClientRouteLimit>({
    ...starterRouteLimit,
    client_id: selectedClientID
  });
  const selectedClient = clients.find((client) => client.id === selectedClientID);
  const effectiveLimits = useMemo(
    () => buildEffectiveLimits(routeLimits, clientRouteLimits),
    [clientRouteLimits, routeLimits]
  );

  useEffect(() => {
    setClientLimitDraft((limit) => ({ ...limit, client_id: selectedClientID }));
  }, [selectedClientID]);

  useEffect(() => {
    if (createdKey) setClientDraft({ name: "", notes: "" });
  }, [createdKey]);

  function trimValue(value: string) {
    return value.trim();
  }

  function hasText(value: string) {
    return trimValue(value).length > 0;
  }

  function optionalText(value: string) {
    const trimmedValue = trimValue(value);
    return trimmedValue.length > 0 ? trimmedValue : undefined;
  }

  function createClient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = trimValue(clientDraft.name);
    if (!hasText(name) || createdKey || isCreatingClient) return;
    onCreateClient({ name, notes: optionalText(clientDraft.notes) });
  }

  function saveClient(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!clientEdit || !hasText(clientEdit.name)) return;
    onPatchClient({
      ...clientEdit,
      name: trimValue(clientEdit.name),
      notes: clientEdit.notes ? optionalText(clientEdit.notes) : undefined
    });
  }

  function saveRouteLimit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const routeKey = trimValue(routeLimitDraft.route_key);
    if (!hasText(routeKey)) return;
    onSaveRouteLimit({ ...routeLimitDraft, route_key: routeKey });
  }

  function saveClientLimit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const clientID = trimValue(selectedClientID);
    const routeKey = trimValue(clientLimitDraft.route_key);
    if (!hasText(clientID) || !hasText(routeKey)) return;
    onSaveClientLimit({ ...clientLimitDraft, client_id: clientID, route_key: routeKey });
  }

  return (
    <div className="access-console">
      {error ? <AccessError title={t("access.issue")} fallback={t("access.saveFailed")} error={error} /> : null}

      <GatewayClientSection
        clients={clients}
        clientDraft={clientDraft}
        clientEdit={clientEdit}
        createdKey={createdKey}
        isCreatingClient={isCreatingClient}
        isRevokingClient={isRevokingClient}
        isSavingClient={isSavingClient}
        onClearCreatedKey={onClearCreatedKey}
        onClientDraftChange={setClientDraft}
        onClientEditChange={setClientEdit}
        onCreateClient={createClient}
        onPatchClient={onPatchClient}
        onRevokeClient={onRevokeClient}
        onSaveClient={saveClient}
      />

      <GatewayDefaultLimitsSection
        routeLimitDraft={routeLimitDraft}
        routeLimits={routeLimits}
        isSavingRouteLimit={isSavingRouteLimit}
        onRouteLimitDraftChange={setRouteLimitDraft}
        onSaveRouteLimit={saveRouteLimit}
      />

      <GatewayClientOverridesSection
        clients={clients}
        clientLimitDraft={clientLimitDraft}
        clientRouteLimits={clientRouteLimits}
        effectiveLimits={effectiveLimits}
        isDeletingClientLimit={isDeletingClientLimit}
        isSavingClientLimit={isSavingClientLimit}
        selectedClient={selectedClient}
        selectedClientID={selectedClientID}
        onClientLimitDraftChange={setClientLimitDraft}
        onDeleteClientLimit={onDeleteClientLimit}
        onSaveClientLimit={saveClientLimit}
        onSelectedClientChange={onSelectedClientChange}
      />
    </div>
  );
}
