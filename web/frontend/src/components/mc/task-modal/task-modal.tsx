import { useState, useEffect } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import type { MCTask } from "@/api/mc"
import { ActivityLog } from "./activity-log"
import { PlanningTab } from "./planning-tab"
import { DeliverablesList } from "./deliverables-list"
import { TaskChatTab } from "./task-chat-tab"
import { TeamTab } from "./team-tab"
import { TaskImages } from "./task-images"
import { updateTask, updateTaskStatus, deleteTask } from "@/api/mc"
import { cn } from "@/lib/utils"

const TABS = [
  { id: "activity", label: "Activity" },
  { id: "planning", label: "Planning" },
  { id: "deliverables", label: "Deliverables" },
  { id: "chat", label: "Chat" },
  { id: "images", label: "Images" },
  { id: "team", label: "Team" },
] as const

type TabId = (typeof TABS)[number]["id"]

const STATUS_OPTIONS = [
  "inbox", "planning", "assigned", "in_progress", "testing", "review", "done",
]

interface TaskModalProps {
  task: MCTask | null
  open: boolean
  onClose: () => void
  onUpdated?: () => void
}

export function TaskModal({ task, open, onClose, onUpdated }: TaskModalProps) {
  const [activeTab, setActiveTab] = useState<TabId>("activity")
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")

  useEffect(() => {
    if (task) {
      setTitle(task.title)
      setDescription(task.description || "")
      setEditing(false)
    }
  }, [task])

  if (!task) return null

  async function handleSave() {
    if (!task) return
    try {
      await updateTask(task.id, { title, description })
      setEditing(false)
      onUpdated?.()
    } catch {
      // Error handled silently
    }
  }

  async function handleStatusChange(status: string) {
    if (!task) return
    try {
      await updateTaskStatus(task.id, status)
      onUpdated?.()
    } catch {
      // Error handled silently
    }
  }

  async function handleDelete() {
    if (!task) return
    try {
      await deleteTask(task.id)
      onClose()
      onUpdated?.()
    } catch {
      // Error handled silently
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-3xl max-h-[85vh] overflow-hidden flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-3">
            {editing ? (
              <input
                className="text-lg font-semibold bg-transparent border-b border-primary flex-1 outline-none"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                autoFocus
              />
            ) : (
              <span className="cursor-pointer hover:underline" onClick={() => setEditing(true)}>
                {task.title}
              </span>
            )}
            <select
              className={cn(
                "text-xs font-medium rounded-full px-2.5 py-0.5 border",
                task.status === "done" ? "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300" :
                task.status === "in_progress" ? "bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300" :
                "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300"
              )}
              value={task.status}
              onChange={(e) => handleStatusChange(e.target.value)}
            >
              {STATUS_OPTIONS.map((s) => (
                <option key={s} value={s}>{s.replace(/_/g, " ")}</option>
              ))}
            </select>
          </DialogTitle>
        </DialogHeader>

        {/* Description */}
        <div className="px-1 pb-2">
          {editing ? (
            <textarea
              className="w-full text-sm bg-transparent border rounded-md p-2 outline-none resize-none"
              rows={3}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          ) : (
            <p className="text-sm text-muted-foreground whitespace-pre-wrap">
              {task.description || "No description. Click to edit."}
            </p>
          )}
          {editing && (
            <div className="flex gap-2 mt-2">
              <button onClick={handleSave} className="text-xs px-3 py-1 bg-primary text-primary-foreground rounded-md">Save</button>
              <button onClick={() => setEditing(false)} className="text-xs px-3 py-1 border rounded-md">Cancel</button>
            </div>
          )}
        </div>

        {/* Tabs */}
        <div className="flex border-b gap-1 px-1">
          {TABS.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                "px-3 py-1.5 text-xs font-medium border-b-2 transition-colors",
                activeTab === tab.id
                  ? "border-primary text-primary"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab Content */}
        <div className="flex-1 overflow-y-auto py-3">
          {activeTab === "activity" && <ActivityLog taskId={task.id} />}
          {activeTab === "planning" && <PlanningTab taskId={task.id} task={task} onUpdated={onUpdated} />}
          {activeTab === "deliverables" && <DeliverablesList taskId={task.id} />}
          {activeTab === "chat" && <TaskChatTab taskId={task.id} />}
          {activeTab === "images" && <TaskImages taskId={task.id} />}
          {activeTab === "team" && <TeamTab taskId={task.id} />}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between border-t pt-3">
          <span className="text-xs text-muted-foreground">
            Created {new Date(task.created_at).toLocaleDateString()}
          </span>
          <div className="flex gap-2">
            <button
              onClick={handleDelete}
              className="text-xs px-3 py-1 text-red-600 border border-red-200 rounded-md hover:bg-red-50 dark:hover:bg-red-950"
            >
              Delete
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}