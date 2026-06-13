import type { ReactNode } from "react";

type FactListItem = {
  label: string;
  value: ReactNode;
};

type FactListProps = {
  items: FactListItem[];
  divided?: boolean;
};

export function FactList({ items, divided = false }: FactListProps) {
  return (
    <dl className={divided ? "stacked-list stacked-list--divided" : "stacked-list"}>
      {items.map((item) => (
        <div key={item.label}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
