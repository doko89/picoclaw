import { useEffect, useRef, useCallback } from "react"
import { getDefaultStore } from "jotai"
import {
  mcTasksAtom,
  mcAgentsAtom,
  mcWorkspacesAtom,
  mcIsOnlineAtom,
  mcUnreadCountsAtom,
} from "@/store/mc"

/**
 * Hook that connects to the Mission Control SSE stream and dispatches
 * events to Jotai atoms. Automatically reconnects with exponential
 * backoff (1s, 2s, 4s, max 5s).
 */
export function useMCSSE() {
  const wsRef = useRef<EventSource | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const reconnectAttemptRef = useRef(0)

  const connect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
    }

    const es = new EventSource("/api/mc/events/stream")
    wsRef.current = es

    es.onopen = () => {
      reconnectAttemptRef.current = 0
      getDefaultStore().set(mcIsOnlineAtom, true)
    }

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleMCEvent(data)
      } catch {
        // Skip malformed events (including keep-alive comments)
      }
    }

    es.onerror = () => {
      es.close()
      wsRef.current = null
      getDefaultStore().set(mcIsOnlineAtom, false)
      scheduleReconnect()
    }
  }, [])

  const scheduleReconnect = useCallback(() => {
    if (reconnectTimerRef.current) return
    const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current), 5000)
    reconnectAttemptRef.current += 1
    reconnectTimerRef.current = setTimeout(() => {
      reconnectTimerRef.current = null
      connect()
    }, delay)
  }, [connect])

  useEffect(() => {
    connect()
    return () => {
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
    }
  }, [connect])
}

interface MCSSEEvent {
  type: string
  payload: Record<string, unknown>
}

function handleMCEvent(event: MCSSEEvent) {
  const store = getDefaultStore()
  const { type } = event

  switch (type) {
    // Task events
    case "task_created":
    case "task_updated":
    case "task_status_changed":
    case "task_deleted":
    case "planning_started":
    case "planning_answer_submitted":
    case "planning_approved":
    case "planning_force_completed":
    case "planning_cancelled":
    case "planning_retry_dispatch":
    case "activity_logged":
    case "deliverable_added":
    case "note_queued": {
      import("@/api/mc").then(({ getTasks }) => {
        getTasks().then((tasks) => {
          store.set(mcTasksAtom, tasks)
        }).catch(() => {})
      })
      break
    }

    // Agent events
    case "agent_created":
    case "agent_updated":
    case "agent_deleted":
    case "agent_imported": {
      import("@/api/mc").then(({ getAgents }) => {
        getAgents().then((agents) => {
          store.set(mcAgentsAtom, agents)
        }).catch(() => {})
      })
      break
    }

    // Workspace events
    case "workspace_created":
    case "workspace_updated":
    case "workspace_deleted": {
      import("@/api/mc").then(({ getWorkspaces }) => {
        getWorkspaces().then((workspaces) => {
          store.set(mcWorkspacesAtom, workspaces)
        }).catch(() => {})
      })
      break
    }

    // Unread updates
    case "note_queued":
    case "note_delivered": {
      import("@/api/mc").then(({ getUnreadTasks }) => {
        getUnreadTasks().then((counts) => {
          store.set(mcUnreadCountsAtom, counts)
        }).catch(() => {})
      })
      break
    }

    default:
      // Unknown event type - ignore
      break
  }
}