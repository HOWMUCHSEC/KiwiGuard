import { z } from "zod";

export async function request<T>(path: string, schema: z.ZodType<T>, init?: RequestInit, opts?: { acceptErrorBody?: boolean }): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers
    }
  });
  if (!response.ok && !opts?.acceptErrorBody) {
    throw new Error(await errorMessage(path, response));
  }
  const data: unknown = await response.json();
  return schema.parse(data);
}

export async function requestNoContent(path: string, init?: RequestInit): Promise<void> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers
    }
  });
  if (!response.ok) {
    throw new Error(await errorMessage(path, response));
  }
}

async function errorMessage(path: string, response: Response) {
  try {
    const data = (await response.json()) as { message?: string };
    if (data.message) return data.message;
  } catch {
    // Fall through to the status-based message.
  }
  return `request to ${path} failed with status ${response.status}`;
}
