import { DragDropContext, type DropResult } from "@hello-pangea/dnd"
import { KanbanColumn } from "./kanban-column"
import type { MCTask } from "@/api/mc"
import { updateTaskStatus } from "@/api/mc"

const KANBAN_COLUMNS: { id: string; label: string; color: string }[] = [
  { id: "inbox", label: "Inbox", color: "bg-gray-100 dark:bg-gray-800" },
  { id: "planning", label: "Planning", color: "bg-blue-50 dark:bg-blue-950" },
  { id: "assigned", label: "Assigned", color: "bg-yellow-50 dark:bg-yellow-950" },
  { id: "in_progress", label: "In Progress", color: "bg-orange-50 dark:bg-orange-950" },
  { id: "testing", label: "Testing", color: "bg-cyan-50 dark:bg-cyan-950" },
  { id: "review", label: "Review", color: "bg-indigo-50 dark:bg-indigo-950" },
  { id: "done", label: "Done", color: "bg-green-50 dark:bg-green-950" },
]

interface KanbanBoardProps {
  tasks: MCTask[]
  onTaskClick?: (task: MCTask) => void
  onTasksChange?: () => void
}

export function KanbanBoard({ tasks, onTaskClick, onTasksChange }: KanbanBoardProps) {
  const columnTasks = KANBAN_COLUMNS.map((col) => ({
    ...col,
    tasks: tasks.filter((t) => t.status === col.id),
  }))

  async function handleDragEnd(result: DropResult) {
    if (!result.destination) return

    const taskId = result.draggableId.replace("task-", "")
    const newStatus = result.destination.droppableId as string

    try {
      await updateTaskStatus(taskId, newStatus)
      onTasksChange?.()
    } catch {
      // Error handled silently - board will refresh on next SSE event
    }
  }

  return (
    <DragDropContext onDragEnd={handleDragEnd}>
      <div className="flex gap-4 overflow-x-auto pb-4 min-h-[calc(100vh-12rem)]">
        {columnTasks.map((col) => (
          <KanbanColumn
            key={col.id}
            id={col.id}
            label={col.label}
            color={col.color}
            tasks={col.tasks}
            onTaskClick={onTaskClick}
          />
        ))}
      </div>
    </DragDropContext>
  )
}