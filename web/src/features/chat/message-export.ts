import type { LiveMessage } from "@/hooks/types";

export type ExportFormat = "markdown" | "json" | "text";

/**
 * Format a list of LiveMessages into a Markdown document.
 */
export function messagesToMarkdown(messages: LiveMessage[]): string {
  if (messages.length === 0) return "";

  const lines: string[] = [];

  for (const msg of messages) {
    const roleLabel =
      msg.role === "user" ? "**User**" : "**Assistant**";

    if (msg.variant === "tool" && msg.toolCall) {
      lines.push(`### ${roleLabel} — Tool: ${msg.toolCall.title ?? "unknown"}`);
      lines.push("");
      if (msg.toolCall.input) {
        lines.push("**Input:**");
        lines.push("```json");
        lines.push(
          typeof msg.toolCall.input === "string"
            ? msg.toolCall.input
            : JSON.stringify(msg.toolCall.input, null, 2),
        );
        lines.push("```");
      }
      if (msg.toolCall.output) {
        lines.push("**Output:**");
        lines.push("```");
        lines.push(msg.toolCall.output);
        lines.push("```");
      }
      if (msg.toolCall.isError) {
        lines.push(`> ⚠ Error: ${msg.toolCall.errorText ?? "unknown error"}`);
      }
      lines.push("");
    } else if (msg.variant === "thinking" && msg.thinking) {
      lines.push(`### ${roleLabel} — Thinking`);
      lines.push("");
      lines.push(`> ${msg.thinking.replace(/\n/g, "\n> ")}`);
      lines.push("");
    } else if (msg.variant === "status" && msg.content) {
      lines.push(`> *${msg.content}*`);
      lines.push("");
    } else if (msg.content) {
      lines.push(`### ${roleLabel}`);
      lines.push("");
      lines.push(msg.content);
      lines.push("");
    }
  }

  return lines.join("\n");
}

/**
 * Format a list of LiveMessages into a JSON string.
 */
export function messagesToJSON(messages: LiveMessage[]): string {
  if (messages.length === 0) return "[]";

  const items = messages.map((msg) => {
    const item: Record<string, unknown> = {
      id: msg.id,
      role: msg.role,
    };

    if (msg.content) item.content = msg.content;
    if (msg.thinking) item.thinking = msg.thinking;
    if (msg.variant) item.variant = msg.variant;

    if (msg.toolCall) {
      item.toolCall = {
        title: msg.toolCall.title,
        input: msg.toolCall.input,
        output: msg.toolCall.output,
        isError: msg.toolCall.isError,
      };
    }

    if (msg.turnIndex !== undefined) item.turnIndex = msg.turnIndex;

    return item;
  });

  return JSON.stringify(items, null, 2);
}

/**
 * Format a list of LiveMessages into plain text.
 */
export function messagesToPlainText(messages: LiveMessage[]): string {
  if (messages.length === 0) return "";

  const lines: string[] = [];

  for (const msg of messages) {
    const roleLabel = msg.role === "user" ? "User" : "Assistant";

    if (msg.variant === "tool" && msg.toolCall) {
      lines.push(
        `[${roleLabel} — Tool: ${msg.toolCall.title ?? "unknown"}]`,
      );
      if (msg.toolCall.input) {
        const inputStr =
          typeof msg.toolCall.input === "string"
            ? msg.toolCall.input
            : JSON.stringify(msg.toolCall.input);
        lines.push(`  Input: ${inputStr}`);
      }
      if (msg.toolCall.output) {
        lines.push(`  Output: ${msg.toolCall.output}`);
      }
      if (msg.toolCall.isError) {
        lines.push(`  Error: ${msg.toolCall.errorText ?? "unknown"}`);
      }
      lines.push("");
    } else if (msg.variant === "thinking" && msg.thinking) {
      lines.push(`[${roleLabel} — Thinking]`);
      lines.push(`  ${msg.thinking.replace(/\n/g, "\n  ")}`);
      lines.push("");
    } else if (msg.variant === "status" && msg.content) {
      lines.push(`--- ${msg.content} ---`);
      lines.push("");
    } else if (msg.content) {
      lines.push(`${roleLabel}:`);
      lines.push(msg.content);
      lines.push("");
    }
  }

  return lines.join("\n").trim();
}

/**
 * Format messages into the requested export format.
 */
export function formatMessages(
  messages: LiveMessage[],
  format: ExportFormat,
): string {
  switch (format) {
    case "markdown":
      return messagesToMarkdown(messages);
    case "json":
      return messagesToJSON(messages);
    case "text":
      return messagesToPlainText(messages);
  }
}

/**
 * Copy formatted messages to the clipboard.
 * Falls back to execCommand for HTTP environments.
 */
export async function copyMessagesToClipboard(
  messages: LiveMessage[],
  format: ExportFormat = "markdown",
): Promise<boolean> {
  const text = formatMessages(messages, format);

  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Fallback for non-HTTPS environments
    try {
      const textarea = document.createElement("textarea");
      textarea.value = text;
      textarea.style.position = "fixed";
      textarea.style.opacity = "0";
      document.body.appendChild(textarea);
      textarea.select();
      const ok = document.execCommand("copy");
      document.body.removeChild(textarea);
      return ok;
    } catch {
      return false;
    }
  }
}

/**
 * Trigger a file download of the formatted messages.
 */
export function downloadMessages(
  messages: LiveMessage[],
  format: ExportFormat = "markdown",
  filename?: string,
): void {
  const text = formatMessages(messages, format);
  const ext = format === "markdown" ? "md" : format === "json" ? "json" : "txt";
  const mime =
    format === "json"
      ? "application/json"
      : format === "markdown"
        ? "text/markdown"
        : "text/plain";

  const blob = new Blob([text], { type: mime });
  const url = URL.createObjectURL(blob);

  const a = document.createElement("a");
  a.href = url;
  a.download = filename ?? `messages-export.${ext}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
