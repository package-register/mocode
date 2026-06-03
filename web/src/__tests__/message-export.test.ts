import { describe, it, expect } from "vitest";
import type { LiveMessage } from "@/hooks/types";
import {
  messagesToMarkdown,
  messagesToJSON,
  messagesToPlainText,
  formatMessages,
} from "@/features/chat/message-export";

const makeMessages = (): LiveMessage[] => [
  {
    id: "u1",
    role: "user",
    content: "Hello, how are you?",
  },
  {
    id: "a1",
    role: "assistant",
    variant: "text",
    content: "I'm doing well, thanks!",
  },
  {
    id: "a2",
    role: "assistant",
    variant: "thinking",
    thinking: "Let me think about this...",
    thinkingDuration: 3,
  },
  {
    id: "t1",
    role: "assistant",
    variant: "tool",
    toolCall: {
      title: "read_file",
      type: "tool-call",
      state: "output-available",
      input: { path: "/tmp/test.txt" },
      output: "file contents here",
      isError: false,
    },
  },
  {
    id: "e1",
    role: "assistant",
    variant: "tool",
    toolCall: {
      title: "run_command",
      type: "tool-call",
      state: "output-error",
      input: "rm -rf /nonexistent",
      errorText: "No such file or directory",
      isError: true,
    },
  },
  {
    id: "s1",
    role: "assistant",
    variant: "status",
    content: "Compaction completed",
  },
];

describe("messagesToMarkdown", () => {
  it("returns empty string for empty array", () => {
    expect(messagesToMarkdown([])).toBe("");
  });

  it("formats user messages with role header", () => {
    const result = messagesToMarkdown([{ id: "u1", role: "user", content: "Hi" }]);
    expect(result).toContain("**User**");
    expect(result).toContain("Hi");
  });

  it("formats assistant text messages", () => {
    const result = messagesToMarkdown([
      { id: "a1", role: "assistant", content: "Reply" },
    ]);
    expect(result).toContain("**Assistant**");
    expect(result).toContain("Reply");
  });

  it("formats tool messages with input and output", () => {
    const msgs = makeMessages().filter((m) => m.variant === "tool");
    const result = messagesToMarkdown(msgs);
    expect(result).toContain("Tool: read_file");
    expect(result).toContain("```json");
    expect(result).toContain("file contents here");
  });

  it("formats error tool messages with warning", () => {
    const msgs = makeMessages().filter(
      (m) => m.toolCall?.isError === true,
    );
    const result = messagesToMarkdown(msgs);
    expect(result).toContain("⚠ Error");
    expect(result).toContain("No such file or directory");
  });

  it("formats thinking messages as blockquotes", () => {
    const msgs = makeMessages().filter((m) => m.variant === "thinking");
    const result = messagesToMarkdown(msgs);
    expect(result).toContain("Thinking");
    expect(result).toContain("> Let me think");
  });

  it("formats status messages as italic", () => {
    const msgs = makeMessages().filter((m) => m.variant === "status");
    const result = messagesToMarkdown(msgs);
    expect(result).toContain("*Compaction completed*");
  });

  it("formats the full message list without errors", () => {
    const result = messagesToMarkdown(makeMessages());
    expect(result.length).toBeGreaterThan(0);
    // Should contain all major sections
    expect(result).toContain("**User**");
    expect(result).toContain("**Assistant**");
    expect(result).toContain("read_file");
  });
});

describe("messagesToJSON", () => {
  it("returns empty JSON array for empty input", () => {
    expect(messagesToJSON([])).toBe("[]");
  });

  it("produces valid JSON", () => {
    const result = messagesToJSON(makeMessages());
    const parsed = JSON.parse(result);
    expect(Array.isArray(parsed)).toBe(true);
    expect(parsed).toHaveLength(6);
  });

  it("includes role and content fields", () => {
    const result = messagesToJSON([{ id: "u1", role: "user", content: "Hi" }]);
    const parsed = JSON.parse(result);
    expect(parsed[0]).toMatchObject({ id: "u1", role: "user", content: "Hi" });
  });

  it("includes tool call data", () => {
    const msgs = makeMessages().filter((m) => m.variant === "tool");
    const result = messagesToJSON(msgs);
    const parsed = JSON.parse(result);
    expect(parsed[0].toolCall).toBeDefined();
    expect(parsed[0].toolCall.title).toBe("read_file");
  });
});

describe("messagesToPlainText", () => {
  it("returns empty string for empty array", () => {
    expect(messagesToPlainText([])).toBe("");
  });

  it("formats with role labels", () => {
    const result = messagesToPlainText([
      { id: "u1", role: "user", content: "Question" },
    ]);
    expect(result).toContain("User:");
    expect(result).toContain("Question");
  });

  it("formats tool calls with indentation", () => {
    const msgs = makeMessages().filter((m) => m.variant === "tool");
    const result = messagesToPlainText(msgs);
    expect(result).toContain("[Assistant — Tool: read_file]");
    expect(result).toContain("Input:");
    expect(result).toContain("Output:");
  });

  it("formats status messages with dashes", () => {
    const msgs = makeMessages().filter((m) => m.variant === "status");
    const result = messagesToPlainText(msgs);
    expect(result).toContain("--- Compaction completed ---");
  });
});

describe("formatMessages", () => {
  it("delegates to correct formatter based on format", () => {
    const msgs: LiveMessage[] = [
      { id: "u1", role: "user", content: "Test" },
    ];

    expect(formatMessages(msgs, "markdown")).toContain("**User**");
    expect(JSON.parse(formatMessages(msgs, "json"))).toHaveLength(1);
    expect(formatMessages(msgs, "text")).toContain("User:");
  });
});
