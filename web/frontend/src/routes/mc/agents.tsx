import { createFileRoute } from "@tanstack/react-router"
import { useAtomValue, useSetAtom } from "jotai"
import { mcAgentsAtom } from "@/store/mc"
import {
	getAgents,
	createAgent,
	updateAgent,
	deleteAgent,
	discoverAgents,
	importAgent,
	type MCAgent,
} from "@/api/mc"
import { useEffect, useState } from "react"
import { IconPlus, IconTrash, IconPencil, IconDownload } from "@tabler/icons-react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"

export const Route = createFileRoute("/mc/agents")({
	component: AgentsPage,
})

const STATUS_COLORS: Record<string, string> = {
	standby: "bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300",
	working: "bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300",
	offline: "bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300",
}

interface DiscoveredAgent {
	name: string
	role: string
	gateway_agent_id: string
}

function AgentsPage() {
	const { t } = useTranslation()
	const agents = useAtomValue(mcAgentsAtom)
	const setAgents = useSetAtom(mcAgentsAtom)
	const [showCreate, setShowCreate] = useState(false)
	const [showDiscover, setShowDiscover] = useState(false)
	const [discovered, setDiscovered] = useState<DiscoveredAgent[]>([])
	const [discovering, setDiscovering] = useState(false)
	const [name, setName] = useState("")
	const [role, setRole] = useState("developer")
	const [description, setDescription] = useState("")
	const [avatar, setAvatar] = useState("🤖")
	const [loading, setLoading] = useState(false)

	useEffect(() => {
		getAgents().then(setAgents).catch((err) => {
			console.error("Failed to fetch agents:", err)
			toast.error(t("mc.error_fetch_agents"))
		})
	}, [setAgents, t])

	async function handleCreate() {
		if (!name.trim()) return
		setLoading(true)
		try {
			const agent = await createAgent({
				name: name.trim(),
				role,
				description: description.trim(),
				avatar_emoji: avatar,
				status: "standby",
				source: "local",
				workspace_id: "default",
			})
			setAgents((prev) => [...prev, agent])
			resetForm()
			setShowCreate(false)
		} catch (err) {
			console.error("Failed to create agent:", err)
			toast.error(t("mc.error_create_agent"))
		} finally {
			setLoading(false)
		}
	}

	async function handleDiscover() {
		setDiscovering(true)
		try {
			const result = await discoverAgents()
			setDiscovered(result)
		} catch (err) {
			console.error("Failed to discover agents:", err)
			toast.error(t("mc.error_discover_agents"))
		}
		setDiscovering(false)
	}

	async function handleImport(da: DiscoveredAgent) {
		try {
			const agent = await importAgent({
				name: da.name,
				gateway_agent_id: da.gateway_agent_id,
				workspace_id: "default",
			})
			setAgents((prev) => [...prev, agent])
			setDiscovered((prev) => prev.filter((a) => a.gateway_agent_id !== da.gateway_agent_id))
			toast.success(t("mc.agent_imported"))
		} catch (err) {
			console.error("Failed to import agent:", err)
			toast.error(t("mc.error_import_agent"))
		}
	}

	async function handleDelete(agentId: string) {
		try {
			await deleteAgent(agentId)
			setAgents((prev) => prev.filter((a) => a.id !== agentId))
			toast.success(t("mc.agent_deleted"))
		} catch (err) {
			console.error("Failed to delete agent:", err)
			toast.error(t("mc.error_delete_agent"))
		}
	}

	function resetForm() {
		setName("")
		setRole("developer")
		setDescription("")
		setAvatar("🤖")
	}

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				<h1 className="text-2xl font-bold">{t("mc.agents")}</h1>
				<div className="flex gap-2">
					<button
						onClick={() => { setShowDiscover(true); handleDiscover() }}
						className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm border rounded-md"
					>
						<IconDownload size={16} />
						{t("mc.discover_agents")}
					</button>
					<button
						onClick={() => setShowCreate(true)}
						className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md"
					>
						<IconPlus size={16} />
						{t("mc.new_agent")}
					</button>
				</div>
			</div>

			{agents.length === 0 ? (
				<div className="text-muted-foreground text-center py-12">
					<p className="mb-2">{t("mc.no_agents_yet")}</p>
					<div className="flex justify-center gap-2">
						<button
							onClick={() => setShowCreate(true)}
							className="text-sm px-4 py-2 bg-primary text-primary-foreground rounded-md"
						>
							{t("mc.create_agent")}
						</button>
					</div>
				</div>
			) : (
				<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
					{agents.map((agent) => (
						<AgentCard key={agent.id} agent={agent} onDelete={handleDelete} />
					))}
				</div>
			)}

			{/* Create Dialog */}
			<Dialog open={showCreate} onOpenChange={(v) => !v && setShowCreate(false)}>
				<DialogContent className="max-w-md">
					<DialogHeader>
						<DialogTitle>{t("mc.create_agent")}</DialogTitle>
					</DialogHeader>
					<div className="space-y-3">
						<input
							className="w-full text-sm border rounded-md px-3 py-2 outline-none"
							placeholder={t("mc.agent_name")}
							value={name}
							onChange={(e) => setName(e.target.value)}
							autoFocus
						/>
						<Select
							value={role}
							onValueChange={(val) => setRole(val)}
						>
							<SelectTrigger className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="developer">{t("mc.role_developer")}</SelectItem>
								<SelectItem value="reviewer">{t("mc.role_reviewer")}</SelectItem>
								<SelectItem value="tester">{t("mc.role_tester")}</SelectItem>
								<SelectItem value="planner">{t("mc.role_planner")}</SelectItem>
								<SelectItem value="orchestrator">{t("mc.role_orchestrator")}</SelectItem>
							</SelectContent>
						</Select>
						<textarea
							className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
							rows={2}
							placeholder={t("mc.description_optional")}
							value={description}
							onChange={(e) => setDescription(e.target.value)}
						/>
						<div className="flex items-center gap-2">
							<span className="text-sm">{t("mc.avatar_label")}:</span>
							<input
								className="text-sm border rounded-md px-2 py-1 w-16 text-center text-lg"
								value={avatar}
								onChange={(e) => setAvatar(e.target.value)}
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

			{/* Discover Dialog */}
			<Dialog open={showDiscover} onOpenChange={(v) => !v && setShowDiscover(false)}>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>{t("mc.discover_agents")}</DialogTitle>
					</DialogHeader>
					{discovering ? (
						<div className="text-center py-8 text-muted-foreground">{t("mc.discovering")}</div>
					) : discovered.length === 0 ? (
						<div className="text-center py-8 text-muted-foreground">{t("mc.no_discovered_agents")}</div>
					) : (
						<div className="space-y-2 max-h-80 overflow-y-auto">
							{discovered.map((da) => (
								<div key={da.gateway_agent_id} className="flex items-center justify-between border rounded-md p-3">
									<div>
										<p className="font-medium text-sm">{da.name}</p>
										<p className="text-xs text-muted-foreground">{da.role}</p>
									</div>
									<button
										onClick={() => handleImport(da)}
										className="text-xs px-2 py-1 bg-primary text-primary-foreground rounded-md"
									>
										{t("mc.import")}
									</button>
								</div>
							))}
						</div>
					)}
				</DialogContent>
			</Dialog>
		</div>
	)
}

function AgentCard({ agent, onDelete }: { agent: MCAgent; onDelete: (id: string) => void }) {
	const { t } = useTranslation()
	const setAgents = useSetAtom(mcAgentsAtom)
	const [showEdit, setShowEdit] = useState(false)
	const [name, setName] = useState(agent.name)
	const [role, setRole] = useState(agent.role)
	const [description, setDescription] = useState(agent.description)
	const [avatar, setAvatar] = useState(agent.avatar_emoji)
	const [saving, setSaving] = useState(false)
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

	async function handleUpdate() {
		setSaving(true)
		try {
			const updated = await updateAgent(agent.id, {
				name: name.trim(),
				role,
				description: description.trim(),
				avatar_emoji: avatar,
			})
			setAgents((prev) => prev.map((a) => a.id === agent.id ? updated : a))
			setShowEdit(false)
		} catch (err) {
			console.error("Failed to update agent:", err)
			toast.error(t("mc.error_update_agent"))
		}
		setSaving(false)
	}

	return (
		<>
			<div className="rounded-lg border bg-card p-4 hover:bg-accent/50 transition-colors">
				<div className="flex items-start gap-3">
					<span className="text-2xl">{agent.avatar_emoji || "🤖"}</span>
					<div className="flex-1 min-w-0">
						<div className="flex items-center gap-2">
							<h3 className="font-medium text-sm">{agent.name}</h3>
							<span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_COLORS[agent.status] || STATUS_COLORS.standby}`}>
								{agent.status}
							</span>
						</div>
						<p className="text-xs text-muted-foreground">{agent.role}</p>
						{agent.description && (
							<p className="text-xs text-muted-foreground mt-1 line-clamp-2">{agent.description}</p>
						)}
					</div>
					<div className="flex gap-1">
						<button
							onClick={() => setShowEdit(true)}
							className="p-1 text-muted-foreground hover:text-foreground rounded"
						>
							<IconPencil size={14} />
						</button>
						<button
							onClick={() => setShowDeleteConfirm(true)}
							className="p-1 text-muted-foreground hover:text-red-500 rounded"
						>
							<IconTrash size={14} />
						</button>
					</div>
				</div>
			</div>

			{/* Edit Dialog */}
			<Dialog open={showEdit} onOpenChange={(v) => !v && setShowEdit(false)}>
				<DialogContent className="max-w-md">
					<DialogHeader>
						<DialogTitle>{t("mc.edit_agent")}</DialogTitle>
					</DialogHeader>
					<div className="space-y-3">
						<input
							className="w-full text-sm border rounded-md px-3 py-2 outline-none"
							value={name}
							onChange={(e) => setName(e.target.value)}
							autoFocus
						/>
						<Select
							value={role}
							onValueChange={(val) => setRole(val)}
						>
							<SelectTrigger className="w-full">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="developer">{t("mc.role_developer")}</SelectItem>
								<SelectItem value="reviewer">{t("mc.role_reviewer")}</SelectItem>
								<SelectItem value="tester">{t("mc.role_tester")}</SelectItem>
								<SelectItem value="planner">{t("mc.role_planner")}</SelectItem>
								<SelectItem value="orchestrator">{t("mc.role_orchestrator")}</SelectItem>
							</SelectContent>
						</Select>
						<textarea
							className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
							rows={2}
							value={description}
							onChange={(e) => setDescription(e.target.value)}
						/>
						<div className="flex items-center gap-2">
							<span className="text-sm">{t("mc.avatar_label")}:</span>
							<input
								className="text-sm border rounded-md px-2 py-1 w-16 text-center text-lg"
								value={avatar}
								onChange={(e) => setAvatar(e.target.value)}
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
						<DialogTitle>{t("mc.delete_agent")}</DialogTitle>
					</DialogHeader>
					<p className="text-sm text-muted-foreground">
						{t("mc.confirm_delete_agent", { name: agent.name })}
					</p>
					<div className="flex justify-end gap-2 mt-4">
						<button onClick={() => setShowDeleteConfirm(false)} className="px-3 py-1.5 text-sm border rounded-md">
							{t("mc.cancel")}
						</button>
						<button
							onClick={() => { onDelete(agent.id); setShowDeleteConfirm(false) }}
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