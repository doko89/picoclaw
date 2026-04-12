import { useEffect, useState } from "react"
import { getTaskActivities, type MCActivity } from "@/api/mc"
import dayjs from "dayjs"

interface ActivityLogProps {
  taskId: string
}

export function ActivityLog({ taskId }: ActivityLogProps) {
  const [activities, setActivities] = useState<MCActivity[]>([])

  useEffect(() => {
    getTaskActivities(taskId).then(setActivities).catch(() => {})
  }, [taskId])

  if (activities.length === 0) {
    return <p className="text-sm text-muted-foreground text-center py-4">No activity yet</p>
  }

  return (
    <div className="space-y-3">
      {activities.map((a) => (
        <div key={a.id} className="flex gap-3 text-sm">
          <div className="flex-shrink-0 w-16 text-xs text-muted-foreground text-right pt-0.5">
            {dayjs(a.created_at).format("MMM D, H:mm")}
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                {a.activity_type}
              </span>
              {a.agent_id && (
                <span className="text-xs text-muted-foreground">{a.agent_id}</span>
              )}
            </div>
            <p className="mt-1">{a.message}</p>
          </div>
        </div>
      ))}
    </div>
  )
}