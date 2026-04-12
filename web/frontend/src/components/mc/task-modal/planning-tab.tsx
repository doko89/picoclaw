import { useEffect, useState } from "react"
import {
  getTaskPlanning,
  startTaskPlanning,
  answerPlanningQuestion,
  approvePlanning,
  forceCompletePlanning,
  cancelTaskPlanning,
  pollTaskPlanning,
  type MCPlanningState,
} from "@/api/mc"
import type { MCTask } from "@/api/mc"

interface PlanningTabProps {
  taskId: string
  task: MCTask
  onUpdated?: () => void
}

export function PlanningTab({ taskId, task, onUpdated }: PlanningTabProps) {
  const [planning, setPlanning] = useState<MCPlanningState | null>(null)
  const [loading, setLoading] = useState(false)
  const [answer, setAnswer] = useState("")
  const [otherText, setOtherText] = useState("")

  useEffect(() => {
    getTaskPlanning(taskId).then(setPlanning).catch(() => {})
  }, [taskId])

  async function handleStart() {
    setLoading(true)
    try {
      await startTaskPlanning(taskId)
      const state = await getTaskPlanning(taskId)
      setPlanning(state)
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  async function handleAnswer() {
    if (!answer) return
    setLoading(true)
    try {
      await answerPlanningQuestion(taskId, {
        answer,
        other_text: otherText,
      })
      setAnswer("")
      setOtherText("")
      // Poll for updated state
      const state = await pollTaskPlanning(taskId)
      setPlanning(state)
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  async function handleApprove() {
    setLoading(true)
    try {
      await approvePlanning(taskId)
      const state = await getTaskPlanning(taskId)
      setPlanning(state)
      onUpdated?.()
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  async function handleForceComplete() {
    setLoading(true)
    try {
      await forceCompletePlanning(taskId)
      const state = await getTaskPlanning(taskId)
      setPlanning(state)
      onUpdated?.()
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  async function handleCancel() {
    setLoading(true)
    try {
      await cancelTaskPlanning(taskId)
      const state = await getTaskPlanning(taskId)
      setPlanning(state)
      onUpdated?.()
    } catch {
      // Error handled silently
    } finally {
      setLoading(false)
    }
  }

  // Not started yet
  if (!planning?.is_started && !task.planning_complete) {
    return (
      <div className="text-center py-8">
        <p className="text-sm text-muted-foreground mb-4">
          Start an AI-assisted planning session to define the task scope, deliverables, and success criteria.
        </p>
        <button
          onClick={handleStart}
          disabled={loading}
          className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm disabled:opacity-50"
        >
          {loading ? "Starting..." : "Start Planning"}
        </button>
      </div>
    )
  }

  // Planning complete
  if (planning?.is_complete || task.planning_complete) {
    return (
      <div className="space-y-4">
        <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
          <span className="text-sm font-medium">Planning Complete</span>
        </div>
        {planning?.spec && (
          <div className="prose prose-sm dark:prose-invert max-w-none">
            <pre className="whitespace-pre-wrap text-xs">{planning.spec.spec_markdown}</pre>
          </div>
        )}
        <div className="flex gap-2">
          <button
            onClick={handleCancel}
            disabled={loading}
            className="px-3 py-1.5 text-xs border rounded-md"
          >
            Reset Planning
          </button>
        </div>
      </div>
    )
  }

  // Planning in progress
  const currentQ = planning?.current_question

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-blue-600 dark:text-blue-400">Planning In Progress</span>
        <div className="flex gap-2">
          <button
            onClick={handleApprove}
            disabled={loading}
            className="px-3 py-1.5 text-xs bg-green-600 text-white rounded-md disabled:opacity-50"
          >
            Approve & Lock
          </button>
          <button
            onClick={handleForceComplete}
            disabled={loading}
            className="px-3 py-1.5 text-xs border rounded-md disabled:opacity-50"
          >
            Force Complete
          </button>
          <button
            onClick={handleCancel}
            disabled={loading}
            className="px-3 py-1.5 text-xs text-red-600 border border-red-200 rounded-md disabled:opacity-50"
          >
            Cancel
          </button>
        </div>
      </div>

      {/* Messages history */}
      {planning?.messages && planning.messages.length > 0 && (
        <div className="space-y-2 max-h-48 overflow-y-auto border rounded-md p-3">
          {planning.messages.map((msg, i) => (
            <div key={i} className={`text-sm ${msg.role === "user" ? "text-right" : ""}`}>
              <span className={`inline-block rounded-lg px-3 py-1.5 max-w-[80%] ${
                msg.role === "user"
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted"
              }`}>
                {msg.content}
              </span>
            </div>
          ))}
        </div>
      )}

      {/* Current question */}
      {currentQ && (
        <div className="border rounded-md p-4 space-y-3">
          <p className="font-medium text-sm">{currentQ.question}</p>
          <div className="grid grid-cols-2 gap-2">
            {currentQ.options?.map((opt) => (
              <button
                key={opt.id}
                onClick={() => setAnswer(opt.id)}
                className={`text-left text-sm px-3 py-2 rounded-md border transition-colors ${
                  answer === opt.id
                    ? "border-primary bg-primary/10"
                    : "hover:bg-accent"
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {answer === "other" && (
            <input
              className="w-full text-sm border rounded-md px-3 py-2 outline-none"
              placeholder="Type your answer..."
              value={otherText}
              onChange={(e) => setOtherText(e.target.value)}
            />
          )}

          <button
            onClick={handleAnswer}
            disabled={loading || !answer}
            className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm disabled:opacity-50"
          >
            {loading ? "Submitting..." : "Submit Answer"}
          </button>
        </div>
      )}
    </div>
  )
}