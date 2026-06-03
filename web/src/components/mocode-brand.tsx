import { cn } from "@/lib/utils";

type MocodeBrandProps = {
  className?: string;
  size?: "sm" | "md";
};

export function MocodeBrand({
  className,
  size = "md",
}: MocodeBrandProps) {
  const textSizeClass = size === "sm" ? "text-base" : "text-lg";
  const logoSize = size === "sm" ? "size-6" : "size-7";
  const logoPx = size === "sm" ? 24 : 28;

  return (
    <div className={cn("flex items-center gap-2", className)}>
      <img
        src="/logo.png"
        alt="Mocode"
        width={logoPx}
        height={logoPx}
        className={logoSize}
      />
      <span className={cn(textSizeClass, "font-semibold text-foreground")}>
        Mocode
      </span>
    </div>
  );
}
