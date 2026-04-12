import { useEffect, useState } from "react"
import { getTaskDeliverables, createTaskDeliverable, type MCDeliverable } from "@/api/mc"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

interface DeliverablesListProps {
  taskId: string
}

export function DeliverablesList({ taskId }: DeliverablesListProps) {
  const [deliverables, setDeliverables] = useState<MCDeliverable[]>([])
  const [showForm, setShowForm] = useState(false)
  const [title, setTitle] = useState("")
  const [path, setPath] = useState("")
  const [desc, setDesc] = useState("")
  const [type, setType] = useState("file")

  useEffect(() => {
    getTaskDeliverables(taskId).then(setDeliverables).catch(() => {})
  }, [taskId])

  async function handleCreate() {
    if (!title) return
    try {
      const d = await createTaskDeliverable(taskId, { title, path, description: desc, deliverable_type: type })
      setDeliverables((prev) => [...prev, d])
      setTitle("")
      setPath("")
      setDesc("")
      setShowForm(false)
    } catch {
      // Error handled silently
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Deliverables</h3>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-xs px-2 py-1 border rounded-md hover:bg-accent"
        >
          {showForm ? "Cancel" : "+ Add"}
        </button>
      </div>

      {showForm && (
        <div className="border rounded-md p-3 space-y-2">
          <input
            className="w-full text-sm border rounded-md px-3 py-1.5 outline-none"
            placeholder="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <div className="flex gap-2">
            <Select
              value={type}
              onValueChange={(val) => setType(val)}
            >
              <SelectTrigger className="w-[120px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="file">File</SelectItem>
                <SelectItem value="document">Document</SelectItem>
                <SelectItem value="code">Code</SelectItem>
                <SelectItem value="artifact">Artifact</SelectItem>
              </SelectContent>
            </Select>
            <input
              className="flex-1 text-sm border rounded-md px-3 py-1.5 outline-none"
              placeholder="Path (optional)"
              value={path}
              onChange={(e) => setPath(e.target.value)}
            />
          </div>
          <input
            className="w-full text-sm border rounded-md px-3 py-1.5 outline-none"
            placeholder="Description (optional)"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
          <button
            onClick={handleCreate}
            disabled={!title}
            className="px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded-md disabled:opacity-50"
          >
            Add Deliverable
          </button>
        </div>
      )}

      {deliverables.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-4">No deliverables yet</p>
      ) : (
        <div className="space-y-2">
          {deliverables.map((d) => (
            <div key={d.id} className="flex items-start gap-3 border rounded-md p-3">
              <span className="inline-flex items-center rounded-full bg-muted px-2 py-0.5 text-xs font-medium">
                {d.deliverable_type}
              </span>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium">{d.title}</p>
                {d.path && (
                  <p className="text-xs text-muted-foreground font-mono truncate">{d.path}</p>
                )}
                {d.description && (
                  <p className="text-xs text-muted-foreground mt-1">{d.description}</p>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}