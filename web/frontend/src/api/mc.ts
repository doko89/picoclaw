import { launcherFetch } from "@/api/http"

/**
 * Mission Control API client.
 * All MC endpoints use the /api/mc/ prefix.
 */

const MC_BASE = "/api/mc"

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await launcherFetch(`${MC_BASE}${path}`, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
    },
    ...options,
  })

  if (!res.ok) {
    const body = await res.text().catch(() => "")
    throw new Error(`MC API ${res.status}: ${body || res.statusText}`)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

// Workspaces
export interface MCWorkspace {
  id: string
  name: string
  slug: string
  description: string
  icon: string
  created_at: string
  updated_at: string
}

export const getWorkspaces = () =>
  request<MCWorkspace[]>("/workspaces")

export const createWorkspace = (data: Partial<MCWorkspace>) =>
  request<MCWorkspace>("/workspaces", { method: "POST", body: JSON.stringify(data) })

export const getWorkspace = (id: string) =>
  request<MCWorkspace>(`/workspaces/${id}`)

export const updateWorkspace = (id: string, data: Partial<MCWorkspace>) =>
  request<MCWorkspace>(`/workspaces/${id}`, { method: "PUT", body: JSON.stringify(data) })

export const deleteWorkspace = (id: string) =>
  request<void>(`/workspaces/${id}`, { method: "DELETE" })

// Agents
export interface MCAgent {
  id: string
  name: string
  role: string
  description: string
  avatar_emoji: string
  status: string
  is_master: boolean
  workspace_id: string
  model: string
  source: string
  gateway_agent_id: string
  session_key_prefix: string
  total_cost_usd: number
  total_tokens_used: number
  created_at: string
  updated_at: string
}

export const getAgents = (workspaceId?: string) => {
  const params = workspaceId ? `?workspace_id=${workspaceId}` : ""
  return request<MCAgent[]>(`/agents${params}`)
}

export const createAgent = (data: Partial<MCAgent>) =>
  request<MCAgent>("/agents", { method: "POST", body: JSON.stringify(data) })

export const getAgent = (id: string) =>
  request<MCAgent>(`/agents/${id}`)

export const updateAgent = (id: string, data: Partial<MCAgent>) =>
  request<MCAgent>(`/agents/${id}`, { method: "PUT", body: JSON.stringify(data) })

export const deleteAgent = (id: string) =>
  request<void>(`/agents/${id}`, { method: "DELETE" })

export const discoverAgents = () =>
  request<Array<{ name: string; role: string; gateway_agent_id: string }>>("/agents/discover")

export const importAgent = (data: { name: string; gateway_agent_id: string; workspace_id?: string }) =>
  request<MCAgent>("/agents/import", { method: "POST", body: JSON.stringify(data) })

// Tasks
export interface MCTask {
  id: string
  title: string
  description: string
  status: string
  priority: string
  assigned_agent_id: string | null
  workspace_id: string
  due_date: string | null
  planning_complete: boolean
  product_id: string | null
  idea_id: string | null
  estimated_cost_usd: number | null
  actual_cost_usd: number
  is_subtask: boolean
  created_at: string
  updated_at: string
}

export const getTasks = (params?: { workspace_id?: string; status?: string; assigned_agent_id?: string }) => {
  const query = new URLSearchParams()
  if (params?.workspace_id) query.set("workspace_id", params.workspace_id)
  if (params?.status) query.set("status", params.status)
  if (params?.assigned_agent_id) query.set("assigned_agent_id", params.assigned_agent_id)
  const qs = query.toString()
  return request<MCTask[]>(`/tasks${qs ? `?${qs}` : ""}`)
}

export const createTask = (data: Partial<MCTask>) =>
  request<MCTask>("/tasks", { method: "POST", body: JSON.stringify(data) })

export const getTask = (id: string) =>
  request<MCTask>(`/tasks/${id}`)

export const updateTask = (id: string, data: Partial<MCTask>) =>
  request<MCTask>(`/tasks/${id}`, { method: "PUT", body: JSON.stringify(data) })

export const deleteTask = (id: string) =>
  request<void>(`/tasks/${id}`, { method: "DELETE" })

export const updateTaskStatus = (id: string, status: string, status_reason?: string) =>
  request<MCTask>(`/tasks/${id}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status, status_reason }),
  })

// Events
export interface MCEvent {
  id: string
  type: string
  agent_id: string | null
  task_id: string | null
  message: string
  metadata: string | null
  created_at: string
}

export const getEvents = () =>
  request<MCEvent[]>("/events")

// Task Activities
export interface MCActivity {
  id: string
  task_id: string
  agent_id: string | null
  activity_type: string
  message: string
  metadata: string | null
  created_at: string
}

export const getTaskActivities = (taskId: string) =>
  request<MCActivity[]>(`/tasks/${taskId}/activities`)

export const createTaskActivity = (taskId: string, data: { agent_id?: string; activity_type?: string; message: string; metadata?: string }) =>
  request<MCActivity>(`/tasks/${taskId}/activities`, { method: "POST", body: JSON.stringify(data) })

// Task Deliverables
export interface MCDeliverable {
  id: string
  task_id: string
  deliverable_type: string
  title: string
  path: string
  description: string
  created_at: string
}

export const getTaskDeliverables = (taskId: string) =>
  request<MCDeliverable[]>(`/tasks/${taskId}/deliverables`)

export const createTaskDeliverable = (taskId: string, data: { deliverable_type?: string; title: string; path?: string; description?: string }) =>
  request<MCDeliverable>(`/tasks/${taskId}/deliverables`, { method: "POST", body: JSON.stringify(data) })

// Task Roles
export interface MCTaskRole {
  id: string
  task_id: string
  role: string
  agent_id: string
  created_at: string
}

export const getTaskRoles = (taskId: string) =>
  request<MCTaskRole[]>(`/tasks/${taskId}/roles`)

export const createTaskRole = (taskId: string, data: { role: string; agent_id: string }) =>
  request<MCTaskRole>(`/tasks/${taskId}/roles`, { method: "POST", body: JSON.stringify(data) })

export const deleteTaskRole = (taskId: string, roleId: string) =>
  request<void>(`/tasks/${taskId}/roles/${roleId}`, { method: "DELETE" })

// Task Notes (Chat)
export interface MCTaskNote {
  id: string
  task_id: string
  content: string
  mode: string
  role: string
  status: string
  delivered_at: string | null
  created_at: string
}

export const getTaskNotes = (taskId: string) =>
  request<MCTaskNote[]>(`/tasks/${taskId}/chat`)

export const createTaskNote = (taskId: string, data: { content: string; mode?: string }) =>
  request<MCTaskNote>(`/tasks/${taskId}/chat`, { method: "POST", body: JSON.stringify(data) })

export const getTaskChatAgents = (taskId: string) =>
  request<Array<{ id: string; name: string; avatar_emoji: string; role: string; status: string; is_assigned: boolean; is_convoy_member: boolean }>>(`/tasks/${taskId}/chat/agents`)

// Unread
export const getUnreadTasks = () =>
  request<Array<{ task_id: string; unread_count: number }>>("/tasks/unread")

export const markTaskRead = (taskId: string) =>
  request<void>(`/tasks/${taskId}/read`, { method: "POST" })

// Planning
export interface MCPlanningState {
  task_id: string
  session_key: string | null
  messages: Array<{ role: string; content: string; timestamp: number }>
  is_started: boolean
  is_complete: boolean
  current_question: {
    question: string
    options: Array<{ id: string; label: string }>
    question_type: string
    category: string
  } | null
  spec: { id: string; task_id: string; spec_markdown: string; locked_at: string; locked_by: string | null; created_at: string } | null
  agents: unknown
}

export const getTaskPlanning = (taskId: string) =>
  request<MCPlanningState>(`/tasks/${taskId}/planning`)

export const startTaskPlanning = (taskId: string, sessionKeyPrefix?: string) =>
  request<{ success: boolean; session_key: string; messages: unknown[]; note: string }>(`/tasks/${taskId}/planning`, { method: "POST", body: JSON.stringify({ session_key_prefix: sessionKeyPrefix }) })

export const answerPlanningQuestion = (taskId: string, data: { question_id?: string; answer: string; other_text?: string }) =>
  request<{ success: boolean; messages: unknown[]; note: string }>(`/tasks/${taskId}/planning/answer`, { method: "POST", body: JSON.stringify(data) })

export const approvePlanning = (taskId: string) =>
  request<{ success: boolean; spec_id: string; spec_markdown: string }>(`/tasks/${taskId}/planning/approve`, { method: "POST" })

export const forceCompletePlanning = (taskId: string, specMarkdown?: string) =>
  request<{ success: boolean; spec_id: string }>(`/tasks/${taskId}/planning/force-complete`, { method: "POST", body: JSON.stringify({ spec_markdown: specMarkdown }) })

export const retryPlanningDispatch = (taskId: string) =>
  request<{ success: boolean }>(`/tasks/${taskId}/planning/retry-dispatch`, { method: "POST" })

export const pollTaskPlanning = (taskId: string) =>
  request<MCPlanningState>(`/tasks/${taskId}/planning/poll`)

export const cancelTaskPlanning = (taskId: string) =>
  request<{ success: boolean }>(`/tasks/${taskId}/planning`, { method: "DELETE" })

// Task Images
export interface MCTaskImage {
  filename: string
  original_name: string
  uploaded_at: string
}

export const getTaskImages = (taskId: string) =>
  request<{ images: MCTaskImage[] }>(`/tasks/${taskId}/images`)

export const uploadTaskImage = async (taskId: string, file: File) => {
  const formData = new FormData()
  formData.append("file", file)
  const res = await launcherFetch(`${MC_BASE}/tasks/${taskId}/images`, {
    method: "POST",
    credentials: "same-origin",
    body: formData,
  })
  if (!res.ok) {
    const body = await res.text().catch(() => "")
    throw new Error(`MC API ${res.status}: ${body || res.statusText}`)
  }
  return res.json() as Promise<{ image: MCTaskImage; total: number }>
}

export const deleteTaskImage = (taskId: string, filename: string) =>
  request<{ success: boolean; remaining: number }>(`/tasks/${taskId}/images`, { method: "DELETE", body: JSON.stringify({ filename }) })