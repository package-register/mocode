import { create } from "zustand";

type MessageSelectionState = {
  /** Whether selection mode is active */
  isSelectMode: boolean;
  /** Set of selected message IDs */
  selectedIds: Set<string>;
};

type MessageSelectionActions = {
  /** Enter selection mode, optionally pre-selecting one message */
  enterSelectMode: (initialId?: string) => void;
  /** Exit selection mode and clear all selections */
  exitSelectMode: () => void;
  /** Toggle selection of a single message */
  toggleSelect: (id: string) => void;
  /** Select all messages from the provided list of IDs */
  selectAll: (ids: string[]) => void;
  /** Deselect all (but stay in select mode) */
  clearSelection: () => void;
  /** Check if a specific message is selected */
  isSelected: (id: string) => boolean;
};

export type MessageSelectionStore = MessageSelectionState &
  MessageSelectionActions;

export const useMessageSelectionStore = create<MessageSelectionStore>(
  (set, get) => ({
    isSelectMode: false,
    selectedIds: new Set<string>(),

    enterSelectMode: (initialId?: string) => {
      const initial = new Set<string>();
      if (initialId) {
        initial.add(initialId);
      }
      set({ isSelectMode: true, selectedIds: initial });
    },

    exitSelectMode: () => {
      set({ isSelectMode: false, selectedIds: new Set() });
    },

    toggleSelect: (id: string) => {
      set((state) => {
        const next = new Set(state.selectedIds);
        if (next.has(id)) {
          next.delete(id);
        } else {
          next.add(id);
        }
        return { selectedIds: next };
      });
    },

    selectAll: (ids: string[]) => {
      set({ selectedIds: new Set(ids) });
    },

    clearSelection: () => {
      set({ selectedIds: new Set() });
    },

    isSelected: (id: string) => {
      return get().selectedIds.has(id);
    },
  }),
);
