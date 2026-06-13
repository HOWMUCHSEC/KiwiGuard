import type { ReactNode } from "react";

type PanelHeaderProps = {
  icon?: ReactNode;
  kicker?: string;
  title: string;
};

export function PanelHeader({ icon, kicker, title }: PanelHeaderProps) {
  return (
    <div className="kg-panel-header">
      <div className="kg-panel-header__body">
        {kicker ? <p className="kg-panel-header__kicker">{kicker}</p> : null}
        <h2 className="kg-panel-header__title">{title}</h2>
      </div>
      {icon ? <div className="kg-panel-header__icon">{icon}</div> : null}
    </div>
  );
}
