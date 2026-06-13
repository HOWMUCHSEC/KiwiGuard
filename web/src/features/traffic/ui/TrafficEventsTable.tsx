import {
  Button,
  DataTable,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableHeader,
  TableRow,
  Tag
} from "@carbon/react";

import type { TrafficEvent } from "../model/trafficApi";
import { useI18n } from "platform/i18n";
import { shortHash } from "shared/ui/format";
import { actionTagType, spoolLabel, spoolTagType } from "shared/ui/status";

type TrafficEventsTableProps = {
  events: TrafficEvent[];
  isError: boolean;
  isLoading: boolean;
  onInspect: (key: string) => void;
  trafficKey: (event: TrafficEvent) => string;
};

export function TrafficEventsTable({ events, isError, isLoading, onInspect, trafficKey }: TrafficEventsTableProps) {
  const { t } = useI18n();
  const headers = [
    { key: "time", header: t("traffic.time") },
    { key: "route", header: t("traffic.route") },
    { key: "direction", header: t("traffic.direction") },
    { key: "status", header: t("traffic.status") },
    { key: "spool", header: t("traffic.spool") },
    { key: "action", header: t("traffic.action") },
    { key: "latency", header: t("traffic.latency") },
    { key: "hashes", header: t("traffic.hashes") },
    { key: "inspect", header: t("traffic.inspect") }
  ];
  const tableRows = events.map((event) => ({
    id: trafficKey(event),
    time: new Date(event.event_time).toLocaleTimeString(),
    route: (
      <span className="kg-stack kg-traffic-table__stack">
        <strong>{event.route_id || t("common.unknown")}</strong>
        <small>{event.provider_id || t("common.unknown")}</small>
      </span>
    ),
    direction: event.direction ? t(`traffic.${event.direction}`) : t("common.unknown"),
    status: (
      <span className="kg-stack kg-traffic-table__stack">
        <strong>
          {event.gateway_status}/{event.upstream_status}
        </strong>
        {event.error_type ? <small>{event.error_type}</small> : null}
      </span>
    ),
    spool: <Tag type={spoolTagType(event.spool_status)}>{t(spoolLabel(event.spool_status))}</Tag>,
    action: <Tag type={actionTagType(event.action)}>{event.action || t("common.unknown")}</Tag>,
    latency: (
      <span className="kg-stack kg-traffic-table__stack">
        <strong>{event.latency_ms} ms</strong>
        <small>
          v {event.verdict_latency_ms} / d {event.detector_latency_ms}
        </small>
      </span>
    ),
    hashes: (
      <span className="kg-stack kg-traffic-table__stack">
        <code>{shortHash(event.request_hash, t("common.none"))}</code>
        <code>{shortHash(event.response_hash, t("common.none"))}</code>
      </span>
    ),
    inspect: (
      <Button kind="ghost" size="sm" onClick={() => onInspect(trafficKey(event))}>
        {t("traffic.inspect")}
      </Button>
    )
  }));

  return (
    <DataTable rows={tableRows} headers={headers}>
      {({ rows, headers, getHeaderProps, getRowProps, getTableProps }) => (
        <div className="kg-traffic-table">
          <TableContainer title={t("traffic.tableTitle")} description={t("traffic.tableDescription")}>
            <Table {...getTableProps()} useZebraStyles>
              <TableHead>
                <TableRow>
                  {headers.map((header) => (
                    <TableHeader {...getHeaderProps({ header })} key={header.key}>
                      {header.header}
                    </TableHeader>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {isError ? (
                  <TableRow>
                    <TableCell colSpan={headers.length}>{t("traffic.loadFailed")}</TableCell>
                  </TableRow>
                ) : rows.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={headers.length}>{isLoading ? t("traffic.loading") : t("traffic.empty")}</TableCell>
                  </TableRow>
                ) : (
                  rows.map((row) => (
                    <TableRow {...getRowProps({ row })} key={row.id}>
                      {row.cells.map((cell) => (
                        <TableCell key={cell.id}>{cell.value}</TableCell>
                      ))}
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </div>
      )}
    </DataTable>
  );
}
