import { useEffect, useState, useRef } from "react"
import { getTaskNotes, createTaskNote, type MCTaskNote } from "@/api/mc"

interface TaskChatTabProps {
  taskId: string
}

export function TaskChatTab({ taskId }: TaskChatTabProps) {
  const [notes, setNotes] = useState<MCTaskNote[]>([])
  const [message, setMessage] = useState("")
  const [mode, setMode] = useState<"note" | "direct">("note")
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    getTaskNotes(taskId).then(setNotes).catch(() => {})
  }, [taskId])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" })
  }, [notes])

  async function handleSend() {
    if (!message.trim()) return
    try {
      const note = await createTaskNote(taskId, { content: message.trim(), mode })
      setNotes((prev) => [...prev, note])
      setMessage("")
    } catch {
      // Error handled silently
    }
  }

  return (
    <div className="flex flex-col h-full min-h-[300px]">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto space-y-2 mb-3">
        {notes.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">No messages yet</p>
        ) : (
          notes.map((note) => (
            <div
              key={note.id}
              className={`flex ${note.role === "user" ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[80%] rounded-lg px-3 py-2 text-sm ${
                  note.role === "user"
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted"
                }`}
              >
                <p className="whitespace-pre-wrap">{note.content}</p>
                <div className="flex items-center gap-2 mt-1">
                  <span className="text-xs opacity-60">
                    {note.role === "user" ? "You" : note.role}
                  </span>
                  {note.mode === "direct" && (
                    <span className="text-xs opacity-60">direct</span>
                  )}
                  {note.status === "pending" && (
                    <span className="text-xs opacity-60">pending</span>
                  )}
                </div>
              </div>
            </div>
          ))
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="border-t pt-3 space-y-2">
        <div className="flex gap-2">
          <select
            className="text-xs border rounded-md px-2 py-1"
            value={mode}
            onChange={(e) => setMode(e.target.value as "note" | "direct")}
          >
            <option value="note">Note</option>
            <option value="direct">Direct</option>
          </select>
          <input
            className="flex-1 text-sm border rounded-md px-3 py-1.5 outline-none"
            placeholder="Type a message..."
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault()
                handleSend()
              }
            }}
          />
          <button
            onClick={handleSend}
            disabled={!message.trim()}
            className="px-3 py-1.5 text-xs bg-primary text-primary-foreground rounded-md disabled:opacity-50"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  )
}