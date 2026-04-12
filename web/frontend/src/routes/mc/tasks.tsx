import { createFileRoute } from "@tanstack/react-router"
import { useAtomValue, useSetAtom } from "jotai"
import { mcTasksAtom } from "@/store/mc"
import { getTasks } from "@/api/mc"
import { useEffect } from "react"
import type { MCTask } from "@/api/mc"
import { KanbanBoard } from "@/components/mc/kanban/kanban-board"
import { TaskModal } from "@/components/mc/task-modal/task-modal"
import { TaskCreateDialog } from "@/components/mc/task-create-dialog"
import { IconPlus } from "@tabler/icons-react"
import { useState } from "react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

export const Route = createFileRoute("/mc/tasks")({
  component: TasksPage,
})

function TasksPage() {
  const { t } = useTranslation()
  const tasks = useAtomValue(mcTasksAtom)
  const setTasks = useSetAtom(mcTasksAtom)
  const [selectedTask, setSelectedTask] = useState<MCTask | null>(null)
  const [showCreate, setShowCreate] = useState(false)

  useEffect(() => {
    getTasks().then(setTasks).catch((err) => {
      console.error("Failed to fetch tasks:", err)
      toast.error(t("mc.error_fetch_tasks"))
    })
  }, [setTasks, t])

  function refreshTasks() {
    getTasks().then(setTasks).catch((err) => {
      console.error("Failed to refresh tasks:", err)
      toast.error(t("mc.error_fetch_tasks"))
    })
  }

  function handleTaskClick(task: MCTask) {
    setSelectedTask(task)
  }

  function handleTaskCreated(task: MCTask) {
    setTasks((prev) => [...prev, task])
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("mc.tasks")}</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md"
        >
          <IconPlus size={16} />
          {t("mc.new_task")}
        </button>
      </div>

      <KanbanBoard
        tasks={tasks}
        onTaskClick={handleTaskClick}
        onTasksChange={refreshTasks}
      />

      <TaskModal
        task={selectedTask}
        open={!!selectedTask}
        onClose={() => setSelectedTask(null)}
        onUpdated={refreshTasks}
      />

      <TaskCreateDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={handleTaskCreated}
      />
    </div>
  )
}
