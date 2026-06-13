import { FormEvent } from "react";
import { Save } from "lucide-react";
import { Button, TextInput, Toggle } from "@carbon/react";

import { useI18n } from "platform/i18n";
import { numberValue } from "../model/accessLimitUtils";
import type { GatewayRouteLimit } from "../model/routingApi";

export function LimitEditor({
  idPrefix,
  limit,
  isPending,
  saveLabel,
  savingLabel,
  onChange,
  onSubmit
}: {
  idPrefix: string;
  limit: GatewayRouteLimit;
  isPending: boolean;
  saveLabel: string;
  savingLabel: string;
  onChange: (limit: GatewayRouteLimit) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}) {
  const { t } = useI18n();

  return (
    <form className="config-form limit-editor" onSubmit={onSubmit}>
      <div className="form-grid">
        <TextInput id={`${idPrefix}-route`} labelText={t("routing.route")} value={limit.route_key} onChange={(event) => onChange({ ...limit, route_key: event.target.value })} />
        <TextInput id={`${idPrefix}-requests`} labelText={t("access.requestsPerWindow")} value={String(limit.requests_per_window)} inputMode="numeric" onChange={(event) => onChange({ ...limit, requests_per_window: numberValue(event.target.value) })} />
        <TextInput id={`${idPrefix}-window`} labelText={t("access.windowSeconds")} value={String(limit.window_seconds)} inputMode="numeric" onChange={(event) => onChange({ ...limit, window_seconds: numberValue(event.target.value) })} />
        <TextInput id={`${idPrefix}-concurrent`} labelText={t("access.maxConcurrent")} value={String(limit.max_concurrent_requests)} inputMode="numeric" onChange={(event) => onChange({ ...limit, max_concurrent_requests: numberValue(event.target.value) })} />
        <TextInput id={`${idPrefix}-body`} labelText={t("access.maxBodyBytes")} value={String(limit.max_body_bytes)} inputMode="numeric" onChange={(event) => onChange({ ...limit, max_body_bytes: numberValue(event.target.value) })} />
        <Toggle id={`${idPrefix}-enabled`} labelText={t("routing.enabled")} toggled={limit.enabled} onToggle={(enabled) => onChange({ ...limit, enabled })} />
      </div>
      <Button type="submit" renderIcon={Save} disabled={isPending || limit.route_key.trim().length === 0}>
        {isPending ? savingLabel : saveLabel}
      </Button>
    </form>
  );
}
