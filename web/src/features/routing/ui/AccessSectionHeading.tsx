import type { LucideIcon } from "lucide-react";

export function AccessSectionHeading({ title, icon: Icon }: { title: string; icon: LucideIcon }) {
  return (
    <div className="access-section-heading">
      <h3>{title}</h3>
      <Icon aria-hidden="true" />
    </div>
  );
}
