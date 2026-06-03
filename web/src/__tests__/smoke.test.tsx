import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";

function SmokeComponent() {
  return <div data-testid="smoke">Hello from Vitest + RTL + happy-dom</div>;
}

describe("smoke test", () => {
  it("renders a component and queries by testid", () => {
    render(<SmokeComponent />);
    expect(screen.getByTestId("smoke")).toHaveTextContent(
      "Hello from Vitest + RTL + happy-dom",
    );
  });

  it("runs basic assertions", () => {
    expect(1 + 1).toBe(2);
    expect("hello".toUpperCase()).toBe("HELLO");
  });
});
