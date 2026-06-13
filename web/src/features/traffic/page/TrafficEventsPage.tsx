import { Tile } from "@carbon/react";
import { MonitorUp } from "lucide-react";
import { useMemo, useState } from "react";

import { PanelHeader } from "shared/ui/PanelHeader";
import type { PageHeading } from "shared/ui/PageHeading";
import { TrafficSummary } from "shared/ui/TrafficSummary";
import { TrafficEventDetail } from "../ui/TrafficEventDetail";
import { TrafficFiltersToolbar } from "../ui/TrafficFiltersToolbar";
import { TrafficEventsTable } from "../ui/TrafficEventsTable";
import { useTrafficEventsPageData } from "../model/useTrafficEventsPageData";
import "./traffic.css";

type TrafficEventsPageProps = {
  heading: PageHeading;
  t: (key: string) => string;
};

export function TrafficEventsPage({ heading, t }: TrafficEventsPageProps) {
  const [route, setRoute] = useState("");
  const [provider, setProvider] = useState("");
  const [direction, setDirection] = useState<"input" | "output" | "">("");
  const [status, setStatus] = useState("");
  const [selectedTrafficKey, setSelectedTrafficKey] = useState("");
  const { events, summary, traffic } = useTrafficEventsPageData({
    route_id: route.trim(),
    provider_id: provider.trim(),
    direction,
    status: status.trim(),
    limit: 25
  });
  const selectedTraffic = useMemo(() => events.find((event) => trafficKey(event) === selectedTrafficKey), [events, selectedTrafficKey]);

  return (
    <Tile className="kg-panel span-3 kg-traffic-panel">
      <PanelHeader kicker={heading.kicker} title={heading.title} icon={<MonitorUp aria-hidden="true" />} />
      <div className="kg-panel-body kg-traffic-panel__body">
        <TrafficFiltersToolbar
          direction={direction}
          provider={provider}
          route={route}
          setDirection={setDirection}
          setProvider={setProvider}
          setRoute={setRoute}
          setStatus={setStatus}
          status={status}
          t={t}
        />
        <div className="kg-traffic-summary-wrap">
          <TrafficSummary
            labels={{
              aria: t("traffic.summaryAria"),
              blocked: t("traffic.blocked"),
              fallbacks: t("traffic.fallbacks"),
              total: t("traffic.total"),
              upstreamErrors: t("traffic.upstreamErrors")
            }}
            summary={summary}
          />
        </div>
        <div className="kg-traffic-table-wrap">
          <TrafficEventsTable
            events={events}
            isError={traffic.isError}
            isLoading={traffic.isLoading}
            onInspect={setSelectedTrafficKey}
            trafficKey={trafficKey}
          />
        </div>
        {selectedTraffic ? <TrafficEventDetail event={selectedTraffic} /> : null}
      </div>
    </Tile>
  );
}

function trafficKey(event: { request_id: string; direction: string; event_time: string }) {
  return `${event.request_id}-${event.direction}-${event.event_time}`;
}
