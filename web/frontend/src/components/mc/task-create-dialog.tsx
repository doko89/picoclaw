import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { createTask, type MCTask } from "@/api/mc"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

const PRIORITY_OPTIONS = ["low", "normal", "high", "urgent"]

interface TaskCreateDialogProps {
  open: boolean
  onClose: () => void
  workspaceId?: string
  onCreated?: (task: MCTask) => void
}

export function TaskCreateDialog({ open, onClose, workspaceId, onCreated }: TaskCreateDialogProps) {
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [priority, setPriority] = useState("normal")
  const [loading, setLoading] = useState(false)

  async function handleCreate() {
    if (!title.trim()) return
    setLoading(true)
    try {
      const task = await createTask({
        title: title.trim(),
        description: description.trim(),
        priority,
        workspace_id: workspaceId || "default",
      })
      onCreated?.(task)
      setTitle("")
      setDescription("")
      setPriority("normal")
      onClose()
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create Task</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <input
            className="w-full text-sm border rounded-md px-3 py-2 outline-none"
            placeholder="Task title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            autoFocus
            onKeyDown={(e) => {
              if (e.key === "Enter" && title.trim()) handleCreate()
            }}
          />
          <textarea
            className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
            rows={3}
            placeholder="Description (optional)"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <Select
            value={priority}
            onValueChange={(val) => setPriority(val)}
          >
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PRIORITY_OPTIONS.map((p) => (
                <SelectItem key={p} value={p}>{p.charAt(0).toUpperCase() + p.slice(1)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="flex justify-end gap-2">
            <button onClick={onClose} className="px-3 py-1.5 text-sm border rounded-md">
              Cancel
            </button>
            <button
              onClick={handleCreate}
              disabled={loading || !title.trim()}
              className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md disabled:opacity-50"
            >
              {loading ? "Creating..." : "Create"}
            </button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}