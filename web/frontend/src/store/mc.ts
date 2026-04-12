import { atom } from "jotai"
import type { MCWorkspace, MCAgent, MCTask, MCEvent } from "@/api/mc"

// Mission Control state atoms, following the pattern from store/gateway.ts

export const mcWorkspacesAtom = atom<MCWorkspace[]>([])
export const mcAgentsAtom = atom<MCAgent[]>([])
export const mcTasksAtom = atom<MCTask[]>([])
export const mcEventsAtom = atom<MCEvent[]>([])
export const mcIsOnlineAtom = atom(false)
export const mcSelectedTaskIdAtom = atom<string | null>(null)

// Task detail atoms
export const mcUnreadCountsAtom = atom<Array<{ task_id: string; unread_count: number }>>([])

export function updateMCStoreFromSSE(_event: { type: string; payload: Record<string, unknown> }) {
  // This function is called from useMCSSE when events arrive.
  // Individual event handlers update atoms directly in the hook.
  // This exported function is available for external callers that need
  // to push events into the store outside the SSE stream.
}