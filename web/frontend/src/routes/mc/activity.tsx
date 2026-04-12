import { createFileRoute } from "@tanstack/react-router"
import { getEvents } from "@/api/mc"
import type { MCEvent } from "@/api/mc"
import { useEffect, useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

export const Route = createFileRoute("/mc/activity")({
  component: ActivityPage,
})

const EVENT_ICONS: Record<string, string> = {
  task_created: "📋",
  task_updated: "✏️",
  task_status_changed: "🔄",
  task_deleted: "🗑️",
  agent_created: "🤖",
  agent_updated: "🔧",
  agent_deleted: "❌",
  workspace_created: "📁",
  workspace_updated: "📝",
  workspace_deleted: "🗑️",
}

function ActivityPage() {
  const { t } = useTranslation()
  const [events, setEvents] = useState<MCEvent[]>([])

  useEffect(() => {
    getEvents().then(setEvents).catch((err) => {
      console.error("Failed to fetch events:", err)
      toast.error(t("mc.error_fetch_events"))
    })
  }, [t])

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{t("mc.activity")}</h1>

      {events.length === 0 ? (
        <div className="text-muted-foreground text-center py-12">
          {t("mc.no_activity_yet")}
        </div>
      ) : (
        <div className="space-y-2">
          {events.map((event) => (
            <div
              key={event.id}
              className="flex items-start gap-3 rounded-lg border bg-card p-3"
            >
              <span className="text-lg">
                {EVENT_ICONS[event.type] || "📌"}
              </span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-muted">
                    {event.type.replace(/_/g, " ")}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {new Date(event.created_at).toLocaleString()}
                  </span>
                </div>
                <p className="text-sm mt-1">{event.message}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}