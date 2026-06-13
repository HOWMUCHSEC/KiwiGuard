import { CodeSnippet, DefinitionTooltip, Tile, Tag } from "@carbon/react";

import type { TrafficEvent } from "../model/trafficApi";
import { useI18n } from "platform/i18n";
import { formatPayload, shortHash } from "shared/ui/format";
import { actionTagType, spoolLabel, spoolTagType } from "shared/ui/status";

export function TrafficEventDetail({ event }: { event: TrafficEvent }) {
  const { t } = useI18n();

  return (
    <Tile className="kg-detail">
      <div className="kg-detail__header">
        <div>
          <p className="kg-kicker">{t("traffic.detail")}</p>
          <h3>{event.request_id}</h3>
        </div>
        <div className="kg-tag-row">
          <Tag type={actionTagType(event.action)}>{event.action || t("common.unknown")}</Tag>
          <Tag type={spoolTagType(event.spool_status)}>{t(spoolLabel(event.spool_status))}</Tag>
        </div>
      </div>
      <dl className="kg-detail-grid">
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.route")}</dt>
          <dd>{event.route_id || t("common.unknown")}</dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.provider")}</dt>
          <dd>{event.provider_id || t("common.unknown")}</dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.status")}</dt>
          <dd>
            {event.gateway_status}/{event.upstream_status}
          </dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.model")}</dt>
          <dd>
            {event.requested_model || t("common.unknown")} {t("traffic.to")} {event.mapped_model || t("common.unknown")}
          </dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.requestHash")}</dt>
          <dd>
            <DefinitionTooltip definition={event.request_hash || t("common.none")}>{shortHash(event.request_hash, t("common.none"))}</DefinitionTooltip>
          </dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.responseHash")}</dt>
          <dd>
            <DefinitionTooltip definition={event.response_hash || t("common.none")}>{shortHash(event.response_hash, t("common.none"))}</DefinitionTooltip>
          </dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.detectorLatency")}</dt>
          <dd>{event.detector_latency_ms} ms</dd>
        </div>
        <div className="kg-detail-grid__item">
          <dt>{t("traffic.verdictLatency")}</dt>
          <dd>{event.verdict_latency_ms} ms</dd>
        </div>
      </dl>
      {event.block_reason ? <p className="kg-risk-text">{event.block_reason}</p> : null}
      <div className="kg-payload-grid">
        <section className="kg-detail-payload">
          <h4>{t("traffic.request")}</h4>
          <CodeSnippet type="multi" feedback={t("traffic.copiedRequest")}>
            {formatPayload(event.request_payload, t("traffic.rawCaptureUnavailable"))}
          </CodeSnippet>
        </section>
        <section className="kg-detail-payload">
          <h4>{t("traffic.response")}</h4>
          <CodeSnippet type="multi" feedback={t("traffic.copiedResponse")}>
            {formatPayload(event.response_payload, t("traffic.rawCaptureUnavailable"))}
          </CodeSnippet>
        </section>
      </div>
    </Tile>
  );
}
