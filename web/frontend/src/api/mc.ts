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

// ─── Autopilot: Products & Health ───────────────────────────────────────────────

export interface MCProductHealthScore {
  id: string
  product_id: string
  overall_score: number
  research_freshness_score: number
  pipeline_depth_score: number
  swipe_velocity_score: number
  build_success_score: number
  cost_efficiency_score: number
  component_data: string | null
  snapshot_date: string | null
  calculated_at: string
}

export interface MCHealthComponentScore {
  name: string
  label: string
  score: number
  weight: number
  effective_weight: number
  raw_value: number
  unit: string
  description: string
}

export interface MCHealthWeightConfig {
  research: number
  pipeline: number
  swipe: number
  build: number
  cost: number
  disabled: string[]
}

export interface MCHealthScoreResponse {
  score: MCProductHealthScore
  components: MCHealthComponentScore[]
  weights: MCHealthWeightConfig
  history: MCProductHealthScore[]
}

export const getProductHealthScore = (productId: string) =>
  request<MCHealthScoreResponse>(`/products/${productId}/health`)

export const updateProductHealthWeights = (productId: string, weights: MCHealthWeightConfig) =>
  request<{ success: boolean }>(`/products/${productId}/health/weights`, {
    method: "PUT",
    body: JSON.stringify(weights),
  })

export const getAllProductHealthScores = () =>
  request<Record<string, number>>("/health-scores")

// ─── Autopilot: Ideas ──────────────────────────────────────────────────────────

export interface MCIdea {
  id: string
  product_id: string
  title: string
  description: string
  category: string
  priority: number
  source: string
  status: string
  suppressed: boolean
  created_at: string
  updated_at: string
}

export interface MCListIdeasOptions {
  status?: string
  category?: string
  source?: string
  limit?: number
}

export const listIdeas = (productId: string, opts?: MCListIdeasOptions) => {
  const params = new URLSearchParams()
  if (opts?.status) params.set("status", opts.status)
  if (opts?.category) params.set("category", opts.category)
  if (opts?.source) params.set("source", opts.source)
  if (opts?.limit) params.set("limit", String(opts.limit))
  const qs = params.toString()
  return request<MCIdea[]>(`/products/${productId}/ideas${qs ? `?${qs}` : ""}`)
}

export const getPendingIdeas = (productId: string) =>
  request<MCIdea[]>(`/products/${productId}/ideas/pending`)

export const getIdea = (ideaId: string) =>
  request<MCIdea>(`/ideas/${ideaId}`)

export const createIdea = (productId: string, data: { title: string; description: string; category: string; impact_score?: number }) =>
  request<{ id: string; title: string; status: string }>(`/products/${productId}/ideas`, {
    method: "POST",
    body: JSON.stringify(data),
  })

export const updateIdea = (ideaId: string, data: { title?: string; description?: string; category?: string; status?: string; user_notes?: string; impact_score?: number; feasibility_score?: number }) =>
  request<MCIdea>(`/ideas/${ideaId}`, { method: "PATCH", body: JSON.stringify(data) })

// ─── Autopilot: Swipe ───────────────────────────────────────────────────────────

export interface MCSwipeHistoryEntry {
  id: string
  idea_id: string
  product_id: string
  action: "approve" | "reject" | "maybe" | "fire"
  category: string
  tags: string | null
  impact_score: number | null
  feasibility_score: number | null
  complexity: string | null
  user_notes: string | null
  created_at: string
}

export interface MCSwipeStats {
  total_swipes: number
  approval_rate: number
  per_category: Record<string, { approved: number; rejected: number; maybe: number; fire: number }>
}

export const getSwipeHistory = (productId: string, limit?: number) => {
  const params = limit ? `?limit=${limit}` : ""
  return request<MCSwipeHistoryEntry[]>(`/products/${productId}/swipe/history${params}`)
}

export const getSwipeStats = (productId: string) =>
  request<MCSwipeStats>(`/products/${productId}/swipe/stats`)

export const undoSwipe = (productId: string, swipeId: string) =>
  request<{ success: boolean; idea: MCIdea }>(`/products/${productId}/swipe/${swipeId}/undo`, {
    method: "DELETE",
  })

export const batchSwipe = (productId: string, actions: Array<{ idea_id: string; action: string }>) =>
  request<{ success: boolean; count: number }>(`/products/${productId}/swipe/batch`, {
    method: "POST",
    body: JSON.stringify({ actions }),
  })

// ─── Autopilot: Maybe Pool ──────────────────────────────────────────────────────

export interface MCMaybePoolEntry {
  id: string
  idea_id: string
  product_id: string
  last_evaluated_at: string | null
  next_evaluate_at: string | null
  evaluation_count: number
  evaluation_notes: string | null
  created_at: string
  idea_title: string
  idea_description: string
  idea_category: string
  idea_priority: number
}

export const getMaybePool = (productId: string) =>
  request<MCMaybePoolEntry[]>(`/products/${productId}/maybe`)

export const resurfaceIdea = (productId: string, maybePoolId: string, reason: string) =>
  request<{ success: boolean; idea_id: string }>(`/products/${productId}/maybe/resurface`, {
    method: "POST",
    body: JSON.stringify({ maybe_pool_id: maybePoolId, reason }),
  })

// ─── Autopilot: Activity Log ───────────────────────────────────────────────────

export interface MCActivityEntry {
  id: string
  product_id: string
  cycle_id: string
  cycle_type: string
  event_type: string
  message: string
  detail: string | null
  cost_usd: number | null
  tokens_used: number | null
  created_at: string
}

export const getActivityLog = (productId: string, limit?: number) =>
  request<{ entries: MCActivityEntry[] }>(`/products/${productId}/activity${limit ? `?limit=${limit}` : ""}`)

// ─── Autopilot: A/B Testing ────────────────────────────────────────────────────

export interface MCVariant {
  id: string
  product_id: string
  name: string
  content: string
  is_control: boolean
  created_at: string
}

export interface MCABTest {
  id: string
  product_id: string
  variant_a_id: string
  variant_b_id: string
  status: "active" | "concluded" | "cancelled"
  split_mode: "concurrent" | "alternating"
  min_swipes: number
  last_variant_used: string | null
  winner_variant_id: string | null
  created_at: string
  concluded_at: string | null
}

export interface MCTestComparison {
  test_id: string
  variant_a: { variant_id: string; total_swipes: number; approved: number; rejected: number; maybe: number; approval_rate: number; built_count: number; cost_per_built: number }
  variant_b: { variant_id: string; total_swipes: number; approved: number; rejected: number; maybe: number; approval_rate: number; built_count: number; cost_per_built: number }
  chi_squared: number
  p_value: number
  significance: string
  winner: string | null
  recommended_winner: string | null
}

export const listVariants = (productId: string) =>
  request<MCVariant[]>(`/products/${productId}/variants`)

export const createVariant = (productId: string, data: { name: string; content: string; is_control?: boolean }) =>
  request<{ id: string; name: string }>(`/products/${productId}/variants`, { method: "POST", body: JSON.stringify(data) })

export const getVariant = (productId: string, variantId: string) =>
  request<MCVariant>(`/products/${productId}/variants/${variantId}`)

export const updateVariant = (productId: string, variantId: string, data: { name: string; content: string }) =>
  request<MCVariant>(`/products/${productId}/variants/${variantId}`, { method: "PATCH", body: JSON.stringify(data) })

export const deleteVariant = (productId: string, variantId: string) =>
  request<{ success: boolean }>(`/products/${productId}/variants/${variantId}`, { method: "DELETE" })

export const listABTests = (productId: string) =>
  request<MCABTest[]>(`/products/${productId}/ab-tests`)

export const startABTest = (productId: string, data: { variant_a_id: string; variant_b_id: string; min_swipes?: number; split_mode?: string }) =>
  request<{ test_id: string; status: string }>(`/products/${productId}/ab-tests`, { method: "POST", body: JSON.stringify(data) })

export const getABTest = (productId: string, testId: string) =>
  request<MCABTest>(`/products/${productId}/ab-tests/${testId}`)

export const concludeABTest = (productId: string, testId: string, winnerVariantId: string) =>
  request<{ success: boolean }>(`/products/${productId}/ab-tests/${testId}/conclude`, {
    method: "PATCH",
    body: JSON.stringify({ winner_variant_id: winnerVariantId }),
  })

export const getABTestComparison = (productId: string, testId: string) =>
  request<MCTestComparison>(`/products/${productId}/ab-tests/${testId}/comparison`)

export const promoteABWinner = (productId: string, testId: string) =>
  request<{ success: boolean; winner_variant_id: string }>(`/products/${productId}/ab-tests/${testId}/promote`, { method: "POST" })

// ─── Autopilot: Scheduling ─────────────────────────────────────────────────────

export interface MCSchedule {
  id: string
  product_id: string
  schedule_type: string
  cron_expression: string
  timezone: string
  enabled: boolean
  last_run_at: string | null
  next_run_at: string | null
  config: string | null
  created_at: string
  updated_at: string
}

export const listSchedules = (productId: string) =>
  request<MCSchedule[]>(`/products/${productId}/schedules`)

export const createSchedule = (productId: string, data: { schedule_type: string; cron_expression: string; timezone?: string; config?: string }) =>
  request<{ id: string; schedule_type: string }>(`/products/${productId}/schedules`, { method: "POST", body: JSON.stringify(data) })

export const updateSchedule = (scheduleId: string, data: { cron_expression?: string; timezone?: string; enabled?: boolean; config?: string }) =>
  request<{ success: boolean }>(`/schedules/${scheduleId}`, { method: "PATCH", body: JSON.stringify(data) })

export const deleteSchedule = (scheduleId: string) =>
  request<{ success: boolean }>(`/schedules/${scheduleId}`, { method: "DELETE" })

// ─── Autopilot: Costs ─────────────────────────────────────────────────────────

export interface MCCostEvent {
  id: string
  product_id: string | null
  workspace_id: string
  task_id: string | null
  cycle_id: string | null
  agent_id: string | null
  event_type: string
  provider: string | null
  model: string | null
  tokens_input: number
  tokens_output: number
  cost_usd: number
  metadata: string | null
  created_at: string
}

export interface MCCostCap {
  id: string
  workspace_id: string
  product_id: string | null
  cap_type: string
  limit_usd: number
  current_spend_usd: number
  period_start: string | null
  period_end: string | null
  status: string
  created_at: string
  updated_at: string
}

export const getProductCosts = (productId: string) =>
  request<{ events: MCCostEvent[] }>(`/products/${productId}/costs`)

export const listCostCaps = (workspaceId: string, productId?: string) => {
  const params = new URLSearchParams({ workspace_id: workspaceId })
  if (productId) params.set("product_id", productId)
  return request<MCCostCap[]>(`/costs/caps?${params}`)
}

export const createCostCap = (data: { workspace_id: string; product_id?: string; cap_type: string; limit_usd: number; period_start?: string; period_end?: string }) =>
  request<{ id: string; cap_type: string }>("/costs/caps", { method: "POST", body: JSON.stringify(data) })

export const updateCostCap = (capId: string, data: { limit_usd?: number; status?: string }) =>
  request<{ success: boolean }>(`/costs/caps/${capId}`, { method: "PATCH", body: JSON.stringify(data) })

export const deleteCostCap = (capId: string) =>
  request<{ success: boolean }>(`/costs/caps/${capId}`, { method: "DELETE" })

export const deleteTaskImage = (taskId: string, filename: string) =>
  request<{ success: boolean; remaining: number }>(`/tasks/${taskId}/images`, { method: "DELETE", body: JSON.stringify({ filename }) })