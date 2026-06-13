import { InlineNotification } from "@carbon/react";

export function AccessError({ title, fallback, error }: { title: string; fallback: string; error?: unknown }) {
  return <InlineNotification kind="error" lowContrast hideCloseButton title={title} subtitle={error instanceof Error ? error.message : fallback} />;
}
