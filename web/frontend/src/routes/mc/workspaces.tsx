import { createFileRoute } from "@tanstack/react-router"
import { useAtomValue, useSetAtom } from "jotai"
import { mcWorkspacesAtom } from "@/store/mc"
import { getWorkspaces, createWorkspace, updateWorkspace, deleteWorkspace, type MCWorkspace } from "@/api/mc"
import { useEffect, useState } from "react"
import { IconPlus, IconTrash, IconPencil } from "@tabler/icons-react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

export const Route = createFileRoute("/mc/workspaces")({
  component: WorkspacesPage,
})

function WorkspacesPage() {
  const { t } = useTranslation()
  const workspaces = useAtomValue(mcWorkspacesAtom)
  const setWorkspaces = useSetAtom(mcWorkspacesAtom)
  const [showCreate, setShowCreate] = useState(false)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [icon, setIcon] = useState("📁")
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    getWorkspaces().then(setWorkspaces).catch((err) => {
      console.error("Failed to fetch workspaces:", err)
      toast.error(t("mc.error_fetch_workspaces"))
    })
  }, [setWorkspaces, t])

  async function handleCreate() {
    if (!name.trim()) return
    setLoading(true)
    try {
      const ws = await createWorkspace({
        name: name.trim(),
        description: description.trim(),
        icon,
        slug: name.trim().toLowerCase().replace(/\s+/g, "-"),
      })
      setWorkspaces((prev) => [...prev, ws])
      resetForm()
      setShowCreate(false)
    } catch (err) {
      console.error("Failed to create workspace:", err)
      toast.error(t("mc.error_create_workspace"))
    } finally {
      setLoading(false)
    }
  }

  async function handleDelete(id: string) {
    try {
      await deleteWorkspace(id)
      setWorkspaces((prev) => prev.filter((ws) => ws.id !== id))
      toast.success(t("mc.workspace_deleted"))
    } catch (err) {
      console.error("Failed to delete workspace:", err)
      toast.error(t("mc.error_delete_workspace"))
    }
  }

  function resetForm() {
    setName("")
    setDescription("")
    setIcon("📁")
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("mc.workspaces")}</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md"
        >
          <IconPlus size={16} />
          {t("mc.new_workspace")}
        </button>
      </div>

      {workspaces.length === 0 ? (
        <div className="text-muted-foreground text-center py-12">
          <p className="mb-4">{t("mc.no_workspaces_yet")}</p>
          <button
            onClick={() => setShowCreate(true)}
            className="text-sm px-4 py-2 bg-primary text-primary-foreground rounded-md"
          >
            {t("mc.create_workspace")}
          </button>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {workspaces.map((ws) => (
            <WorkspaceCard key={ws.id} workspace={ws} onDelete={handleDelete} />
          ))}
        </div>
      )}

      <Dialog open={showCreate} onOpenChange={(v) => !v && setShowCreate(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("mc.create_workspace")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <input
              className="w-full text-sm border rounded-md px-3 py-2 outline-none"
              placeholder={t("mc.workspace_name")}
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
            <textarea
              className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
              rows={2}
              placeholder={t("mc.description_optional")}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
            <div className="flex items-center gap-2">
              <span className="text-sm">{t("mc.icon_label")}:</span>
              <input
                className="text-sm border rounded-md px-2 py-1 w-16 text-center"
                value={icon}
                onChange={(e) => setIcon(e.target.value)}
                maxLength={2}
              />
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowCreate(false)}
                className="px-3 py-1.5 text-sm border rounded-md"
              >
                {t("mc.cancel")}
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !name.trim()}
                className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md disabled:opacity-50"
              >
                {loading ? t("mc.creating") : t("mc.create")}
              </button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function WorkspaceCard({ workspace, onDelete }: { workspace: MCWorkspace; onDelete: (id: string) => void }) {
  const { t } = useTranslation()
  const setWorkspaces = useSetAtom(mcWorkspacesAtom)
  const [showEdit, setShowEdit] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [name, setName] = useState(workspace.name)
  const [description, setDescription] = useState(workspace.description)
  const [icon, setIcon] = useState(workspace.icon)
  const [saving, setSaving] = useState(false)

  async function handleUpdate() {
    setSaving(true)
    try {
      const updated = await updateWorkspace(workspace.id, {
        name: name.trim(),
        description: description.trim(),
        icon,
        slug: name.trim().toLowerCase().replace(/\s+/g, "-"),
      })
      setWorkspaces((prev) => prev.map((ws) => ws.id === workspace.id ? updated : ws))
      setShowEdit(false)
    } catch (err) {
      console.error("Failed to update workspace:", err)
      toast.error(t("mc.error_update_workspace"))
    }
    setSaving(false)
  }

  return (
    <>
      <div className="rounded-lg border bg-card p-4 hover:bg-accent/50 transition-colors">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xl">{workspace.icon}</span>
            <h3 className="font-semibold">{workspace.name}</h3>
          </div>
          <div className="flex gap-1">
            <button onClick={() => setShowEdit(true)} className="p-1 text-muted-foreground hover:text-foreground rounded">
              <IconPencil size={14} />
            </button>
            <button onClick={() => setShowDeleteConfirm(true)} className="p-1 text-muted-foreground hover:text-red-500 rounded">
              <IconTrash size={14} />
            </button>
          </div>
        </div>
        {workspace.description && (
          <p className="text-sm text-muted-foreground line-clamp-2">{workspace.description}</p>
        )}
        <div className="text-xs text-muted-foreground mt-2">/{workspace.slug}</div>
      </div>

      {/* Edit Dialog */}
      <Dialog open={showEdit} onOpenChange={(v) => !v && setShowEdit(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("mc.edit_workspace")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <input
              className="w-full text-sm border rounded-md px-3 py-2 outline-none"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
            <textarea
              className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
              rows={2}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
            <div className="flex items-center gap-2">
              <span className="text-sm">{t("mc.icon_label")}:</span>
              <input
                className="text-sm border rounded-md px-2 py-1 w-16 text-center"
                value={icon}
                onChange={(e) => setIcon(e.target.value)}
                maxLength={2}
              />
            </div>
            <div className="flex justify-end gap-2">
              <button onClick={() => setShowEdit(false)} className="px-3 py-1.5 text-sm border rounded-md">
                {t("mc.cancel")}
              </button>
              <button
                onClick={handleUpdate}
                disabled={saving || !name.trim()}
                className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md disabled:opacity-50"
              >
                {saving ? t("mc.saving") : t("mc.save")}
              </button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <Dialog open={showDeleteConfirm} onOpenChange={(v) => !v && setShowDeleteConfirm(false)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("mc.delete_workspace")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("mc.confirm_delete_workspace", { name: workspace.name })}
          </p>
          <div className="flex justify-end gap-2 mt-4">
            <button onClick={() => setShowDeleteConfirm(false)} className="px-3 py-1.5 text-sm border rounded-md">
              {t("mc.cancel")}
            </button>
            <button
              onClick={() => { onDelete(workspace.id); setShowDeleteConfirm(false) }}
              className="px-3 py-1.5 text-sm bg-red-600 text-white rounded-md"
            >
              {t("mc.delete")}
            </button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}