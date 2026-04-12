import { useEffect, useState } from "react"
import { getTaskRoles, createTaskRole, deleteTaskRole, getAgents, type MCTaskRole, type MCAgent } from "@/api/mc"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

interface TeamTabProps {
  taskId: string
}

export function TeamTab({ taskId }: TeamTabProps) {
  const [roles, setRoles] = useState<MCTaskRole[]>([])
  const [agents, setAgents] = useState<MCAgent[]>([])
  const [showForm, setShowForm] = useState(false)
  const [selectedAgent, setSelectedAgent] = useState("")
  const [roleName, setRoleName] = useState("")

  useEffect(() => {
    getTaskRoles(taskId).then(setRoles).catch(() => {})
    getAgents().then(setAgents).catch(() => {})
  }, [taskId])

  async function handleAddRole() {
    if (!selectedAgent || !roleName) return
    try {
      const role = await createTaskRole(taskId, { agent_id: selectedAgent, role: roleName })
      setRoles((prev) => [...prev, role])
      setShowForm(false)
      setSelectedAgent("")
      setRoleName("")
    } catch {
      // Error handled silently
    }
  }

  async function handleDeleteRole(roleId: string) {
    try {
      await deleteTaskRole(taskId, roleId)
      setRoles((prev) => prev.filter((r) => r.id !== roleId))
    } catch {
      // Error handled silently
    }
  }

  const agentMap = Object.fromEntries(agents.map((a) => [a.id, a]))

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Team Roles</h3>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-xs px-2 py-1 border rounded-md hover:bg-accent"
        >
          {showForm ? "Cancel" : "+ Assign Role"}
        </button>
      </div>

      {showForm && (
        <div className="border rounded-md p-3 space-y-2">
          <Select
            value={selectedAgent || undefined}
            onValueChange={(val) => setSelectedAgent(val)}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Select agent..." />
            </SelectTrigger>
            <SelectContent>
              {agents.map((a) => (
                <SelectItem key={a.id} value={a.id}>
                  {a.avatar_emoji} {a.name} - {a.role}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <input
            className="w-full text-sm border rounded-md px-3 py-1.5 outline-none"
            placeholder="Role (e.g., developer, reviewer, tester)"
            value={roleName}
            onChange={(e) => setRoleName(e.target.value)}
          />
          <button
            onClick={handleAddRole}
            disabled={!selectedAgent || !roleName}
            className="px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded-md disabled:opacity-50"
          >
            Assign
          </button>
        </div>
      )}

      {roles.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-4">No roles assigned</p>
      ) : (
        <div className="space-y-2">
          {roles.map((r) => {
            const agent = agentMap[r.agent_id]
            return (
              <div key={r.id} className="flex items-center justify-between border rounded-md p-3">
                <div className="flex items-center gap-2">
                  <span className="text-lg">{agent?.avatar_emoji || "🤖"}</span>
                  <div>
                    <p className="text-sm font-medium">{agent?.name || r.agent_id}</p>
                    <p className="text-xs text-muted-foreground">{r.role}</p>
                  </div>
                </div>
                <button
                  onClick={() => handleDeleteRole(r.id)}
                  className="text-xs text-red-500 hover:text-red-700"
                >
                  Remove
                </button>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}