import { Tile } from "@carbon/react";
import type { LucideIcon } from "lucide-react";
import { useId } from "react";
import type { MouseEventHandler } from "react";

type MetricTileBaseProps = {
  label: string;
  value: string;
  icon: LucideIcon;
};

type MetricTileButtonProps = MetricTileBaseProps & {
  ariaLabel?: string;
  onClick: MouseEventHandler<HTMLButtonElement>;
};

type MetricTileStaticProps = MetricTileBaseProps & {
  ariaLabel?: never;
  onClick?: never;
};

type MetricTileProps = MetricTileButtonProps | MetricTileStaticProps;

export function MetricTile({ label, value, icon: Icon, ariaLabel, onClick }: MetricTileProps) {
  const labelId = useId();
  const valueId = useId();
  const content = (
    <>
      <div className="kg-metric-tile__body">
        <span id={labelId} className="kg-metric-tile__label">
          {label}
        </span>
        <strong id={valueId} className="kg-metric-tile__value">
          {value}
        </strong>
      </div>
      <span className="kg-metric-tile__icon" aria-hidden="true">
        <Icon />
      </span>
    </>
  );

  if (onClick !== undefined) {
    return (
      <button
        className="kg-metric-tile kg-metric-tile--interactive"
        type="button"
        aria-label={ariaLabel}
        aria-labelledby={ariaLabel ? undefined : `${labelId} ${valueId}`}
        onClick={onClick}
      >
        {content}
      </button>
    );
  }

  return (
    <Tile className="kg-metric-tile">
      {content}
    </Tile>
  );
}
