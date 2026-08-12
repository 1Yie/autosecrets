// Shared client state that crosses component/route boundaries: the CSRF
// token issued by Core at login and restored from /me. Server data (the
// admin profile) stays in TanStack Query, never duplicated here.
import { create } from "zustand";

interface SessionState {
  csrfToken: string;
  setCsrfToken: (token: string) => void;
  clearSession: () => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  csrfToken: "",
  setCsrfToken: (csrfToken) => set({ csrfToken }),
  clearSession: () => set({ csrfToken: "" }),
}));
