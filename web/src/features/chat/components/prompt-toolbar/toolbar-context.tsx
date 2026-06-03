import { type ReactElement, memo } from "react";
import {
  Context,
  ContextTrigger,
  ContextContent,
  ContextContentHeader,
  ContextContentBody,
  ContextProgressIcon,
} from "@ai-elements";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import type { TokenUsage } from "@/hooks/wireTypes";

type ToolbarContextIndicatorProps = {
  usagePercent: number;
  usedTokens: number;
  maxTokens: number;
  tokenUsage: TokenUsage | null;
  className?: string;
};

export const ToolbarContextIndicator = memo(
  function ToolbarContextIndicatorComponent({
    usagePercent,
    usedTokens,
    maxTokens,
    tokenUsage,
    className,
  }: ToolbarContextIndicatorProps): ReactElement {
    const usedPercent = maxTokens > 0 ? usedTokens / maxTokens : 0;

    return (
      <Context
        usedTokens={usedTokens}
        maxTokens={maxTokens}
        tokenUsage={tokenUsage}
      >
        <ContextTrigger
          className={cn(
            "flex items-center gap-1.5 h-7 px-2.5 rounded-full text-xs font-medium",
            "transition-colors cursor-default border",
            "bg-transparent text-muted-foreground border-border/60",
            "hover:text-foreground hover:border-border",
            className,
          )}
        >
          <ContextProgressIcon usedPercent={usedPercent} size={14} />
          <span>{usagePercent.toFixed(1)}% context</span>
        </ContextTrigger>
        <ContextContent align="end" side="top" sideOffset={8} className="w-64">
          <ContextContentHeader />
          {tokenUsage && (
            <ScrollArea className="max-h-52">
              <ContextContentBody>
                <TokenBreakdown tokenUsage={tokenUsage} />
              </ContextContentBody>
            </ScrollArea>
          )}
        </ContextContent>
      </Context>
    );
  },
);

const TokenBreakdown = ({
  tokenUsage,
}: {
  tokenUsage: TokenUsage;
}): ReactElement => {
  const totalInput =
    tokenUsage.input_other +
    tokenUsage.input_cache_read +
    tokenUsage.input_cache_creation;

  return (
    <div className="space-y-3 text-xs">
      <div className="space-y-1">
        <div className="text-[11px] font-medium text-muted-foreground">
          Input Tokens
        </div>
        <RawUsageRow
          label="Regular"
          value={tokenUsage.input_other}
          description="Tokens processed without cache"
        />
        <RawUsageRow
          label="Cache Read"
          value={tokenUsage.input_cache_read}
          description="Tokens loaded from cache"
        />
        <RawUsageRow
          label="Cache Write"
          value={tokenUsage.input_cache_creation}
          description="Tokens written to cache"
        />
        <div className="flex items-center justify-between text-xs font-medium border-t pt-1.5 mt-1">
          <span>Total Input</span>
          <span>
            {new Intl.NumberFormat("en-US", { notation: "compact" }).format(
              totalInput,
            )}
          </span>
        </div>
      </div>

      <div className="space-y-1 border-t pt-2.5">
        <div className="text-[11px] font-medium text-muted-foreground">
          Output Tokens
        </div>
        <RawUsageRow
          label="Generated"
          value={tokenUsage.output}
          description="Tokens generated in response"
        />
      </div>
    </div>
  );
};

const RawUsageRow = ({
  label,
  value,
  description,
}: {
  label: string;
  value: number;
  description?: string;
}): ReactElement => {
  const content = (
    <div className="flex items-center justify-between text-xs">
      <span className="text-muted-foreground">{label}</span>
      <span>
        {new Intl.NumberFormat("en-US", { notation: "compact" }).format(value)}
      </span>
    </div>
  );

  if (description) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="cursor-help">{content}</div>
        </TooltipTrigger>
        <TooltipContent side="left">
          <p className="text-xs">{description}</p>
        </TooltipContent>
      </Tooltip>
    );
  }

  return content;
};
