import { FormEvent } from "react";
import { Copy, KeyRound, Plus, Save } from "lucide-react";
import { Button, InlineNotification, Select, SelectItem, TextArea, TextInput } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { AccessSectionHeading } from "./AccessSectionHeading";
import { ClientList } from "./ClientList";
import type { GatewayClient } from "../model/routingApi";

export type GatewayClientDraft = {
  name: string;
  notes: string;
};

type GatewayClientSectionProps = {
  clients: GatewayClient[];
  clientDraft: GatewayClientDraft;
  clientEdit: GatewayClient | null;
  createdKey?: string;
  isCreatingClient: boolean;
  isRevokingClient: boolean;
  isSavingClient: boolean;
  onClearCreatedKey: () => void;
  onClientDraftChange: (draft: GatewayClientDraft) => void;
  onClientEditChange: (client: GatewayClient | null) => void;
  onCreateClient: (event: FormEvent<HTMLFormElement>) => void;
  onPatchClient: (client: GatewayClient) => void;
  onRevokeClient: (clientID: string) => void;
  onSaveClient: (event: FormEvent<HTMLFormElement>) => void;
};

export function GatewayClientSection({
  clients,
  clientDraft,
  clientEdit,
  createdKey,
  isCreatingClient,
  isRevokingClient,
  isSavingClient,
  onClearCreatedKey,
  onClientDraftChange,
  onClientEditChange,
  onCreateClient,
  onPatchClient,
  onRevokeClient,
  onSaveClient
}: GatewayClientSectionProps) {
  const { t } = useI18n();

  return (
    <section className="access-section">
      <AccessSectionHeading title={t("access.clients")} icon={KeyRound} />
      {createdKey ? (
        <div className="one-time-key">
          <InlineNotification kind="warning" lowContrast hideCloseButton title={t("access.keyCreated")} subtitle={t("access.keyCreatedHelp")} />
          <code>{createdKey}</code>
          <div className="kg-inline-actions">
            <Button size="sm" kind="secondary" renderIcon={Copy} onClick={() => void navigator.clipboard?.writeText(createdKey)}>
              {t("access.copyKey")}
            </Button>
            <Button size="sm" kind="ghost" onClick={onClearCreatedKey}>
              {t("access.hideKey")}
            </Button>
          </div>
        </div>
      ) : null}

      <form className="config-form" onSubmit={onCreateClient}>
        <div className="form-grid">
          <TextInput id="gateway-client-name" labelText={t("access.clientName")} value={clientDraft.name} onChange={(event) => onClientDraftChange({ ...clientDraft, name: event.target.value })} />
          <TextInput id="gateway-client-notes" labelText={t("access.notes")} value={clientDraft.notes} onChange={(event) => onClientDraftChange({ ...clientDraft, notes: event.target.value })} />
        </div>
        <Button type="submit" renderIcon={Plus} disabled={isCreatingClient || Boolean(createdKey) || clientDraft.name.trim().length === 0}>
          {isCreatingClient ? t("access.creatingClient") : t("access.createClient")}
        </Button>
      </form>

      <ClientList clients={clients} isRevoking={isRevokingClient} isSaving={isSavingClient} onEdit={onClientEditChange} onPatch={onPatchClient} onRevoke={onRevokeClient} />

      {clientEdit ? (
        <form className="config-form access-editor" onSubmit={onSaveClient}>
          <div className="form-grid">
            <TextInput id="gateway-client-edit-name" labelText={t("access.clientName")} value={clientEdit.name} onChange={(event) => onClientEditChange({ ...clientEdit, name: event.target.value })} />
            <Select id="gateway-client-edit-status" labelText={t("access.status")} value={clientEdit.status} onChange={(event) => onClientEditChange({ ...clientEdit, status: event.target.value as GatewayClient["status"] })}>
              <SelectItem value="enabled" text={t("access.status.enabled")} />
              <SelectItem value="disabled" text={t("access.status.disabled")} />
              <SelectItem value="revoked" text={t("access.status.revoked")} />
            </Select>
            <TextArea id="gateway-client-edit-notes" className="wide-field" labelText={t("access.notes")} value={clientEdit.notes ?? ""} onChange={(event) => onClientEditChange({ ...clientEdit, notes: event.target.value })} rows={3} />
          </div>
          <div className="kg-inline-actions">
            <Button type="submit" renderIcon={Save} disabled={isSavingClient || clientEdit.name.trim().length === 0}>
              {isSavingClient ? t("routing.saving") : t("access.saveClient")}
            </Button>
            <Button type="button" kind="ghost" onClick={() => onClientEditChange(null)}>
              {t("access.cancel")}
            </Button>
          </div>
        </form>
      ) : null}
    </section>
  );
}
