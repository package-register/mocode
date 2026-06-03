import { describe, it, expect, beforeEach } from "vitest";
import { useMessageSelectionStore } from "@/features/chat/message-selection-store";

// Reset store between tests
beforeEach(() => {
  useMessageSelectionStore.getState().exitSelectMode();
});

describe("useMessageSelectionStore", () => {
  it("starts with empty state", () => {
    const { isSelectMode, selectedIds } = useMessageSelectionStore.getState();
    expect(isSelectMode).toBe(false);
    expect(selectedIds.size).toBe(0);
  });

  describe("enterSelectMode", () => {
    it("sets isSelectMode to true", () => {
      useMessageSelectionStore.getState().enterSelectMode();
      expect(useMessageSelectionStore.getState().isSelectMode).toBe(true);
    });

    it("pre-selects an initial ID when provided", () => {
      useMessageSelectionStore.getState().enterSelectMode("msg-1");
      const { isSelectMode, selectedIds } =
        useMessageSelectionStore.getState();
      expect(isSelectMode).toBe(true);
      expect(selectedIds.has("msg-1")).toBe(true);
      expect(selectedIds.size).toBe(1);
    });

    it("starts with empty selection when no initial ID", () => {
      useMessageSelectionStore.getState().enterSelectMode();
      expect(useMessageSelectionStore.getState().selectedIds.size).toBe(0);
    });
  });

  describe("exitSelectMode", () => {
    it("clears isSelectMode and selectedIds", () => {
      const store = useMessageSelectionStore.getState();
      store.enterSelectMode("msg-1");
      store.toggleSelect("msg-2");

      store.exitSelectMode();
      const { isSelectMode, selectedIds } =
        useMessageSelectionStore.getState();
      expect(isSelectMode).toBe(false);
      expect(selectedIds.size).toBe(0);
    });
  });

  describe("toggleSelect", () => {
    it("adds an ID when not present", () => {
      useMessageSelectionStore.getState().toggleSelect("msg-1");
      expect(
        useMessageSelectionStore.getState().selectedIds.has("msg-1"),
      ).toBe(true);
    });

    it("removes an ID when already present", () => {
      const store = useMessageSelectionStore.getState();
      store.toggleSelect("msg-1");
      store.toggleSelect("msg-1");
      expect(
        useMessageSelectionStore.getState().selectedIds.has("msg-1"),
      ).toBe(false);
    });

    it("handles multiple toggles independently", () => {
      const store = useMessageSelectionStore.getState();
      store.toggleSelect("msg-1");
      store.toggleSelect("msg-2");
      store.toggleSelect("msg-3");
      store.toggleSelect("msg-2"); // remove msg-2

      const { selectedIds } = useMessageSelectionStore.getState();
      expect(selectedIds.has("msg-1")).toBe(true);
      expect(selectedIds.has("msg-2")).toBe(false);
      expect(selectedIds.has("msg-3")).toBe(true);
      expect(selectedIds.size).toBe(2);
    });
  });

  describe("selectAll", () => {
    it("selects all provided IDs", () => {
      useMessageSelectionStore
        .getState()
        .selectAll(["msg-1", "msg-2", "msg-3"]);
      const { selectedIds } = useMessageSelectionStore.getState();
      expect(selectedIds.size).toBe(3);
      expect(selectedIds.has("msg-1")).toBe(true);
      expect(selectedIds.has("msg-2")).toBe(true);
      expect(selectedIds.has("msg-3")).toBe(true);
    });

    it("replaces previous selection", () => {
      const store = useMessageSelectionStore.getState();
      store.toggleSelect("old-msg");
      store.selectAll(["new-1", "new-2"]);
      const { selectedIds } = useMessageSelectionStore.getState();
      expect(selectedIds.has("old-msg")).toBe(false);
      expect(selectedIds.size).toBe(2);
    });
  });

  describe("clearSelection", () => {
    it("empties selectedIds but stays in select mode", () => {
      const store = useMessageSelectionStore.getState();
      store.enterSelectMode("msg-1");
      store.toggleSelect("msg-2");
      store.clearSelection();

      const { isSelectMode, selectedIds } =
        useMessageSelectionStore.getState();
      expect(isSelectMode).toBe(true);
      expect(selectedIds.size).toBe(0);
    });
  });

  describe("isSelected", () => {
    it("returns true for selected IDs", () => {
      useMessageSelectionStore.getState().toggleSelect("msg-1");
      expect(useMessageSelectionStore.getState().isSelected("msg-1")).toBe(
        true,
      );
    });

    it("returns false for unselected IDs", () => {
      expect(useMessageSelectionStore.getState().isSelected("msg-1")).toBe(
        false,
      );
    });
  });
});
