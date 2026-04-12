import { Draggable } from "@hello-pangea/dnd"
import type { MCTask } from "@/api/mc"
import dayjs from "dayjs"

const PRIORITY_BADGE: Record<string, string> = {
  low: "text-gray-400",
  normal: "",
  high: "text-orange-500",
  urgent: "text-red-500",
}

interface KanbanCardProps {
  task: MCTask
  index: number
  onClick?: () => void
}

export function KanbanCard({ task, index, onClick }: KanbanCardProps) {
  return (
    <Draggable draggableId={`task-${task.id}`} index={index}>
      {(provided, snapshot) => (
        <div
          ref={provided.innerRef}
          {...provided.draggableProps}
          {...provided.dragHandleProps}
          onClick={onClick}
          className={`rounded-lg border bg-card p-3 cursor-pointer transition-shadow hover:shadow-md ${
            snapshot.isDragging ? "shadow-lg ring-2 ring-primary/20" : ""
          }`}
        >
          <div className="flex items-start justify-between gap-2">
            <h4 className="font-medium text-sm line-clamp-2 leading-tight">{task.title}</h4>
            {task.priority && task.priority !== "normal" && (
              <span className={`text-xs font-medium ${PRIORITY_BADGE[task.priority] || ""}`}>
                {task.priority}
              </span>
            )}
          </div>

          {task.description && (
            <p className="text-xs text-muted-foreground mt-1.5 line-clamp-2">
              {task.description.length > 120
                ? task.description.slice(0, 120) + "..."
                : task.description}
            </p>
          )}

          <div className="flex items-center gap-2 mt-2 text-xs text-muted-foreground">
            <span>{dayjs(task.created_at).format("MMM D")}</span>
            {task.assigned_agent_id && (
              <span className="inline-flex items-center gap-0.5">
                <span className="w-1.5 h-1.5 rounded-full bg-blue-400" />
                assigned
              </span>
            )}
            {task.planning_complete && (
              <span className="inline-flex items-center gap-0.5">
                <span className="w-1.5 h-1.5 rounded-full bg-green-400" />
                planned
              </span>
            )}
          </div>
        </div>
      )}
    </Draggable>
  )
}