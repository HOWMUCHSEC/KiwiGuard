import { Select, SelectItem, TextInput } from "@carbon/react";

type TrafficFiltersToolbarProps = {
  direction: "" | "input" | "output";
  provider: string;
  route: string;
  setDirection: (direction: "" | "input" | "output") => void;
  setProvider: (value: string) => void;
  setRoute: (value: string) => void;
  setStatus: (value: string) => void;
  status: string;
  t: (key: string) => string;
};

export function TrafficFiltersToolbar({
  direction,
  provider,
  route,
  setDirection,
  setProvider,
  setRoute,
  setStatus,
  status,
  t
}: TrafficFiltersToolbarProps) {
  return (
    <div className="traffic-toolbar">
      <TextInput id="traffic-route" labelText={t("traffic.route")} value={route} onChange={(event) => setRoute(event.target.value)} placeholder={t("traffic.routePlaceholder")} />
      <TextInput id="traffic-provider" labelText={t("traffic.provider")} value={provider} onChange={(event) => setProvider(event.target.value)} placeholder={t("traffic.providerPlaceholder")} />
      <Select id="traffic-direction" labelText={t("traffic.direction")} value={direction} onChange={(event) => setDirection(event.target.value as "input" | "output" | "")}>
        <SelectItem value="" text={t("traffic.any")} />
        <SelectItem value="input" text={t("traffic.input")} />
        <SelectItem value="output" text={t("traffic.output")} />
      </Select>
      <TextInput id="traffic-status" labelText={t("traffic.status")} value={status} onChange={(event) => setStatus(event.target.value)} inputMode="numeric" placeholder={t("traffic.statusPlaceholder")} />
    </div>
  );
}
