import { Droppable } from "@hello-pangea/dnd"
import { KanbanCard } from "./kanban-card"
import type { MCTask } from "@/api/mc"

interface KanbanColumnProps {
  id: string
  label: string
  color: string
  tasks: MCTask[]
  onTaskClick?: (task: MCTask) => void
}

export function KanbanColumn({ id, label, color, tasks, onTaskClick }: KanbanColumnProps) {
  return (
    <div className={`flex-shrink-0 w-72 rounded-lg ${color} flex flex-col`}>
      <div className="p-3 border-b border-border/50">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-sm">{label}</h3>
          <span className="text-xs text-muted-foreground bg-background/80 px-2 py-0.5 rounded-full">
            {tasks.length}
          </span>
        </div>
      </div>

      <Droppable droppableId={id}>
        {(provided, snapshot) => (
          <div
            ref={provided.innerRef}
            {...provided.droppableProps}
            className={`flex-1 p-2 space-y-2 min-h-[100px] transition-colors ${
              snapshot.isDraggingOver ? "bg-accent/30" : ""
            }`}
          >
            {tasks.map((task, index) => (
              <KanbanCard
                key={task.id}
                task={task}
                index={index}
                onClick={() => onTaskClick?.(task)}
              />
            ))}
            {provided.placeholder}
          </div>
        )}
      </Droppable>
    </div>
  )
}