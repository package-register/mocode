import { type ReactElement, memo, useCallback } from "react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  CheckSquare,
  Square,
  Copy,
  Download,
  X,
} from "lucide-react";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { LiveMessage } from "@/hooks/types";
import type { ExportFormat } from "../message-export";
import { copyMessagesToClipboard, downloadMessages } from "../message-export";

type MessageSelectBarProps = {
  /** Number of currently selected messages */
  selectedCount: number;
  /** Total number of selectable messages */
  totalCount: number;
  /** Whether all messages are selected */
  isAllSelected: boolean;
  /** Selected messages data for export */
  selectedMessages: LiveMessage[];
  /** Select all messages */
  onSelectAll: () => void;
  /** Clear all selections */
  onClearSelection: () => void;
  /** Exit selection mode */
  onExit: () => void;
  /** Optional session name for export filename */
  sessionName?: string;
};

export const MessageSelectBar = memo(function MessageSelectBarComponent({
  selectedCount,
  totalCount,
  isAllSelected,
  selectedMessages,
  onSelectAll,
  onClearSelection,
  onExit,
  sessionName,
}: MessageSelectBarProps): ReactElement {
  const handleCopy = useCallback(
    async (format: ExportFormat) => {
      if (selectedMessages.length === 0) return;
      const ok = await copyMessagesToClipboard(selectedMessages, format);
      if (ok) {
        toast.success("Copied to clipboard", {
          description: `${selectedMessages.length} message${selectedMessages.length !== 1 ? "s" : ""} copied as ${format}`,
        });
      } else {
        toast.error("Copy failed", {
          description: "Unable to access clipboard",
        });
      }
    },
    [selectedMessages],
  );

  const handleDownload = useCallback(
    (format: ExportFormat) => {
      if (selectedMessages.length === 0) return;
      const ext =
        format === "markdown" ? "md" : format === "json" ? "json" : "txt";
      const timestamp = new Date()
        .toISOString()
        .replace(/[:.]/g, "-")
        .slice(0, 19);
      const baseName = sessionName
        ? `messages-${sessionName}`
        : "messages-export";
      const filename = `${baseName}-${timestamp}.${ext}`;

      downloadMessages(selectedMessages, format, filename);
      toast.success("Export started", {
        description: `${selectedMessages.length} message${selectedMessages.length !== 1 ? "s" : ""} exported as ${format}`,
      });
    },
    [selectedMessages, sessionName],
  );

  return (
    <div
      className={cn(
        "flex items-center justify-between gap-2",
        "rounded-md bg-secondary/80 border border-border px-3 py-1.5",
      )}
      role="toolbar"
      aria-label="Message selection actions"
    >
      {/* Left: select toggle + count */}
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="text-muted-foreground hover:text-foreground transition-colors"
          onClick={isAllSelected ? onClearSelection : onSelectAll}
          aria-label={isAllSelected ? "Deselect all" : "Select all"}
        >
          {isAllSelected ? (
            <CheckSquare className="size-4" />
          ) : (
            <Square className="size-4" />
          )}
        </button>
        <span className="text-xs text-muted-foreground">
          {selectedCount > 0
            ? `${selectedCount} of ${totalCount} selected`
            : `${totalCount} messages`}
        </span>
      </div>

      {/* Right: actions */}
      <div className="flex items-center gap-1">
        {/* Copy dropdown */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 text-xs"
              disabled={selectedCount === 0}
            >
              <Copy className="size-3.5" />
              Copy
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-40">
            <DropdownMenuItem onClick={() => handleCopy("markdown")}>
              as Markdown
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleCopy("text")}>
              as Plain Text
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleCopy("json")}>
              as JSON
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Download dropdown */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1.5 text-xs"
              disabled={selectedCount === 0}
            >
              <Download className="size-3.5" />
              Export
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-40">
            <DropdownMenuItem onClick={() => handleDownload("markdown")}>
              as Markdown (.md)
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleDownload("text")}>
              as Plain Text (.txt)
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleDownload("json")}>
              as JSON (.json)
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Divider */}
        <div className="mx-1 h-4 w-px bg-border" />

        {/* Cancel */}
        <button
          type="button"
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-background hover:text-foreground transition-colors"
          onClick={onExit}
          aria-label="Exit selection mode"
        >
          <X className="size-4" />
        </button>
      </div>
    </div>
  );
});
