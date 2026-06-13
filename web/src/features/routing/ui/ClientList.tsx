import { Button, Tag } from "@carbon/react";

import { useI18n } from "platform/i18n";
import type { GatewayClient } from "../model/routingApi";

export function ClientList({
  clients,
  isRevoking,
  isSaving,
  onEdit,
  onPatch,
  onRevoke
}: {
  clients: GatewayClient[];
  isRevoking: boolean;
  isSaving: boolean;
  onEdit: (client: GatewayClient) => void;
  onPatch: (client: GatewayClient) => void;
  onRevoke: (clientID: string) => void;
}) {
  const { t } = useI18n();

  return (
    <div className="compact-list">
      {clients.length === 0 ? (
        <p className="kg-muted">{t("access.clientsEmpty")}</p>
      ) : (
        clients.map((client) => {
          const nextStatus = client.status === "enabled" ? "disabled" : "enabled";
          return (
            <div className="access-row" key={client.id}>
              <div className="kg-stack">
                <strong>{client.name}</strong>
                <small>
                  {client.id}
                  {client.key_prefix ? ` · ${client.key_prefix}` : ""}
                </small>
              </div>
              <Tag type={client.status === "enabled" ? "green" : client.status === "revoked" ? "red" : "gray"}>
                {t(`access.status.${client.status}`)}
              </Tag>
              <div className="kg-inline-actions">
                <Button size="sm" kind="ghost" onClick={() => onEdit(client)}>
                  {t("access.edit")}
                </Button>
                <Button size="sm" kind="ghost" disabled={client.status === "revoked" || isSaving} onClick={() => onPatch({ ...client, status: nextStatus })}>
                  {client.status === "enabled" ? t("access.disable") : t("access.enable")}
                </Button>
                <Button size="sm" kind="danger--ghost" disabled={client.status === "revoked" || isRevoking} onClick={() => onRevoke(client.id)}>
                  {t("access.revoke")}
                </Button>
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
