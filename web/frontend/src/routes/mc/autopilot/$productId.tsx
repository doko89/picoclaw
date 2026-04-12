import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Suspense, useCallback, useEffect, useState } from "react";
import { useAtom, useSetAtom } from "jotai";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import { IconPencil, IconTrash } from "@tabler/icons-react";
import {
	mcProductsAtom,
	mcResearchCyclesAtom,
	mcIdeationCyclesAtom,
	mcSwipeDeckAtom,
	mcCurrentSwipeIndexAtom,
	mcSwipeStatsAtom,
	mcLoadingResearchAtom,
	mcLoadingIdeationAtom,
	mcRunningResearchAtom,
	mcRunningIdeationAtom,
	mcSelectedProductIdAtom,
} from "@/store/mc-autopilot";
import {
	getMCResearchCycles,
	getMCIdeationCycles,
	getMCSwipeDeck,
	runMCResearch,
	runMCIdeation,
	swipeIdea,
	updateMCProduct,
	deleteMCProduct,
} from "@/api/mc-products";
import type { MCResearchCycle, MCIdeationCycle, MCIdea } from "@/api/mc-products";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";

export const Route = createFileRoute("/mc/autopilot/$productId")({
	component: ProductDetail,
});

function ProductDetail() {
	const { t } = useTranslation();
	const { productId } = Route.useParams();
	const setSelectedProductId = useSetAtom(mcSelectedProductIdAtom);

	useEffect(() => {
		setSelectedProductId(productId);
	}, [productId, setSelectedProductId]);

	return (
		<div className="p-6">
			<Suspense fallback={<div className="text-muted-foreground">{t("mc.loading")}</div>}>
				<ProductDetailContent productId={productId} />
			</Suspense>
		</div>
	);
}

function ProductDetailContent({ productId }: { productId: string }) {
	const { t } = useTranslation();
	const [products] = useAtom(mcProductsAtom);
	const setProducts = useSetAtom(mcProductsAtom);
	const product = products.find((p) => p.id === productId);
	const [researchCycles, setResearchCycles] = useAtom(mcResearchCyclesAtom);
	const [ideationCycles, setIdeationCycles] = useAtom(mcIdeationCyclesAtom);
	const [swipeDeck, setSwipeDeck] = useAtom(mcSwipeDeckAtom);
	const [currentIndex, setCurrentIndex] = useAtom(mcCurrentSwipeIndexAtom);
	const [swipeStats, setSwipeStats] = useAtom(mcSwipeStatsAtom);
	const [loadingResearch, setLoadingResearch] = useAtom(mcLoadingResearchAtom);
	const [loadingIdeation, setLoadingIdeation] = useAtom(mcLoadingIdeationAtom);
	const [runningResearch, setRunningResearch] = useAtom(mcRunningResearchAtom);
	const [runningIdeation, setRunningIdeation] = useAtom(mcRunningIdeationAtom);
	const [activeTab, setActiveTab] = useState<"overview" | "swipe" | "settings">("overview");
	const [showEdit, setShowEdit] = useState(false);
	const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
	const [editName, setEditName] = useState("");
	const [editDesc, setEditDesc] = useState("");
	const [editRepo, setEditRepo] = useState("");
	const [editWebsite, setEditWebsite] = useState("");
	const [editTier, setEditTier] = useState("supervised");
	const [editResearch, setEditResearch] = useState(true);
	const [editIdeation, setEditIdeation] = useState(true);
	const [saving, setSaving] = useState(false);
	const navigate = useNavigate();

	useEffect(() => {
		if (product) {
			loadData();
		}
	}, [productId, product]);

	async function loadData() {
		setLoadingResearch(true);
		setLoadingIdeation(true);
		try {
			const [research, ideation, deck] = await Promise.all([
				getMCResearchCycles(productId),
				getMCIdeationCycles(productId),
				getMCSwipeDeck(productId),
			]);
			setResearchCycles(research);
			setIdeationCycles(ideation);
			setSwipeDeck(deck);
			setCurrentIndex(0);
		} catch (error) {
			console.error("Failed to load product data:", error);
			toast.error(t("mc.error_load_product_data"));
		}
		setLoadingResearch(false);
		setLoadingIdeation(false);
	}

	const handleSwipe = useCallback(async (action: "approve" | "reject" | "maybe") => {
		const card = swipeDeck[currentIndex];
		if (!card) return;

		try {
			await swipeIdea(card.id, {
				product_id: productId,
				action,
			});

			if (action === "approve") {
				setSwipeStats({ ...swipeStats, approved: swipeStats.approved + 1 });
			} else if (action === "reject") {
				setSwipeStats({ ...swipeStats, rejected: swipeStats.rejected + 1 });
			} else {
				setSwipeStats({ ...swipeStats, maybe: swipeStats.maybe + 1 });
			}

			if (currentIndex < swipeDeck.length - 1) {
				setCurrentIndex(currentIndex + 1);
			} else {
				await loadData();
			}
		} catch (error) {
			console.error("Failed to swipe:", error);
			toast.error(t("mc.error_swipe"));
		}
	}, [swipeDeck, currentIndex, swipeStats, productId, t]);

	function openEditDialog() {
		if (!product) return;
		setEditName(product.name);
		setEditDesc(product.description);
		setEditRepo(product.repo_url);
		setEditWebsite(product.live_url);
		setEditTier(product.automation_tier);
		setEditResearch(product.research_enabled);
		setEditIdeation(product.ideation_enabled);
		setShowEdit(true);
	}

	async function handleUpdate() {
		setSaving(true);
		try {
			await updateMCProduct(productId, {
				name: editName.trim(),
				description: editDesc.trim(),
				repo_url: editRepo.trim(),
				live_url: editWebsite.trim(),
				automation_tier: editTier,
				research_enabled: editResearch,
				ideation_enabled: editIdeation,
			});
			setProducts(products.map((p) => p.id === productId ? {
				...p,
				name: editName.trim(),
				description: editDesc.trim(),
				repo_url: editRepo.trim(),
				live_url: editWebsite.trim(),
				automation_tier: editTier,
				research_enabled: editResearch,
				ideation_enabled: editIdeation,
			} : p));
			setShowEdit(false);
		} catch (error) {
			console.error("Failed to update product:", error);
			toast.error(t("mc.error_update_product"));
		}
		setSaving(false);
	}

	async function handleDelete() {
		try {
			await deleteMCProduct(productId);
			setProducts(products.filter((p) => p.id !== productId));
			navigate({ to: "/mc/autopilot" });
			toast.success(t("mc.product_deleted"));
		} catch (error) {
			console.error("Failed to delete product:", error);
			toast.error(t("mc.error_delete_product"));
		}
	}

	// Keyboard shortcuts for swipe deck
	useEffect(() => {
		if (activeTab !== "swipe") return;

		function onKeyDown(e: KeyboardEvent) {
			if (e.key === "ArrowLeft") {
				e.preventDefault();
				handleSwipe("reject");
			} else if (e.key === "ArrowRight") {
				e.preventDefault();
				handleSwipe("approve");
			} else if (e.key === "`" || e.key === "~") {
				e.preventDefault();
				handleSwipe("maybe");
			}
		}

		window.addEventListener("keydown", onKeyDown);
		return () => window.removeEventListener("keydown", onKeyDown);
	}, [activeTab, handleSwipe]);

	if (!product) {
		return <div className="text-muted-foreground">{t("mc.product_not_found")}</div>;
	}

	const latestResearch = researchCycles[0];

	return (
		<div className="space-y-6">
			{/* Header */}
			<div className="flex justify-between items-start">
				<div>
					<Link
						to="/mc/autopilot"
						className="text-sm text-muted-foreground hover:text-foreground mb-2 inline-block"
					>
						&larr; {t("mc.back_to_products")}
					</Link>
					<h1 className="text-2xl font-bold">{product.name}</h1>
					{product.description && (
						<p className="text-muted-foreground mt-1">{product.description}</p>
					)}
				</div>
				<div className="flex gap-2 items-center">
					<button
						onClick={() => setActiveTab("overview")}
						className={`px-4 py-2 rounded-lg ${activeTab === "overview" ? "bg-primary text-primary-foreground" : "bg-muted"}`}
					>
						{t("mc.overview")}
					</button>
					<button
						onClick={() => setActiveTab("swipe")}
						className={`px-4 py-2 rounded-lg ${activeTab === "swipe" ? "bg-primary text-primary-foreground" : "bg-muted"}`}
					>
						{t("mc.swipe_deck")} {swipeDeck.length > 0 && `(${swipeDeck.length})`}
					</button>
					<button
						onClick={() => setActiveTab("settings")}
						className={`px-4 py-2 rounded-lg ${activeTab === "settings" ? "bg-primary text-primary-foreground" : "bg-muted"}`}
					>
						{t("mc.settings")}
					</button>
					<div className="border-l pl-2 flex gap-1">
						<button onClick={openEditDialog} className="p-1.5 text-muted-foreground hover:text-foreground rounded">
							<IconPencil size={16} />
						</button>
						<button onClick={() => setShowDeleteConfirm(true)} className="p-1.5 text-muted-foreground hover:text-red-500 rounded">
							<IconTrash size={16} />
						</button>
					</div>
				</div>
			</div>

			{activeTab === "overview" ? (
				<>
					{/* Actions */}
					<div className="flex gap-3">
						<button
							onClick={async () => {
								setRunningResearch(true);
								try {
									await runMCResearch(productId, { chain_ideation: true });
									await loadData();
								} catch (error) {
									console.error("Failed to run research:", error);
									toast.error(t("mc.error_run_research"));
								}
								setRunningResearch(false);
							}}
							disabled={runningResearch}
							className="px-4 py-2 bg-primary text-primary-foreground rounded-lg disabled:opacity-50"
						>
							{runningResearch ? t("mc.running_research") : t("mc.run_research")}
						</button>
						<button
							onClick={async () => {
								setRunningIdeation(true);
								try {
									await runMCIdeation(productId, {
										research_cycle_id: latestResearch?.id,
									});
									await loadData();
								} catch (error) {
									console.error("Failed to run ideation:", error);
									toast.error(t("mc.error_run_ideation"));
								}
								setRunningIdeation(false);
							}}
							disabled={runningIdeation || !latestResearch}
							className="px-4 py-2 bg-primary text-primary-foreground rounded-lg disabled:opacity-50"
						>
							{runningIdeation ? t("mc.generating_ideas") : t("mc.generate_ideas")}
						</button>
					</div>

					{/* Research Cycles */}
					<div className="rounded-lg border bg-card p-4">
						<h2 className="text-lg font-semibold mb-3">{t("mc.research_cycles")}</h2>
						{loadingResearch ? (
							<div className="text-muted-foreground">{t("mc.loading")}</div>
						) : researchCycles.length === 0 ? (
							<div className="text-muted-foreground">{t("mc.no_research_cycles_yet")}</div>
						) : (
							<div className="space-y-3">
								{researchCycles.map((cycle) => (
									<ResearchCard key={cycle.id} cycle={cycle} />
								))}
							</div>
						)}
					</div>

					{/* Ideation Cycles */}
					<div className="rounded-lg border bg-card p-4">
						<h2 className="text-lg font-semibold mb-3">{t("mc.ideation_cycles")}</h2>
						{loadingIdeation ? (
							<div className="text-muted-foreground">{t("mc.loading")}</div>
						) : ideationCycles.length === 0 ? (
							<div className="text-muted-foreground">{t("mc.no_ideation_cycles_yet")}</div>
						) : (
							<div className="space-y-3">
								{ideationCycles.map((cycle) => (
									<IdeationCard key={cycle.id} cycle={cycle} />
								))}
							</div>
						)}
					</div>
				</>
			) : activeTab === "swipe" ? (
				<SwipeDeckView
					deck={swipeDeck}
					currentIndex={currentIndex}
					onSwipe={handleSwipe}
					stats={swipeStats}
				/>
			) : (
				/* Settings tab */
				<div className="rounded-lg border bg-card p-4 space-y-4">
					<h2 className="text-lg font-semibold">{t("mc.product_settings")}</h2>
					<div className="space-y-3">
						<div>
							<label className="text-sm font-medium">{t("mc.automation_tier")}</label>
							<p className="text-xs text-muted-foreground mb-1">{t("mc.automation_tier_desc")}</p>
							<select
								className="w-full text-sm border rounded-md px-3 py-2"
								value={product.automation_tier}
								onChange={async (e) => {
									try {
										await updateMCProduct(productId, { automation_tier: e.target.value });
										setProducts(products.map((p) => p.id === productId ? { ...p, automation_tier: e.target.value } : p));
									} catch (error) {
										console.error("Failed to update tier:", error);
										toast.error(t("mc.error_update_product"));
									}
								}}
							>
								<option value="supervised">{t("mc.tier_supervised")}</option>
								<option value="semi_auto">{t("mc.tier_semi_auto")}</option>
								<option value="full_auto">{t("mc.tier_full_auto")}</option>
							</select>
						</div>
						<div>
							<label className="text-sm font-medium">{t("mc.repo_url")}</label>
							<input
								className="w-full text-sm border rounded-md px-3 py-2 outline-none"
								placeholder="https://github.com/..."
								defaultValue={product.repo_url}
								onBlur={async (e) => {
									if (e.target.value !== product.repo_url) {
										try {
											await updateMCProduct(productId, { repo_url: e.target.value });
											setProducts(products.map((p) => p.id === productId ? { ...p, repo_url: e.target.value } : p));
										} catch (error) {
											console.error("Failed to update repo URL:", error);
											toast.error(t("mc.error_update_product"));
										}
									}
								}}
							/>
						</div>
						<div>
							<label className="text-sm font-medium">{t("mc.live_url")}</label>
							<input
								className="w-full text-sm border rounded-md px-3 py-2 outline-none"
								placeholder="https://..."
								defaultValue={product.live_url}
								onBlur={async (e) => {
									if (e.target.value !== product.live_url) {
										try {
											await updateMCProduct(productId, { live_url: e.target.value });
											setProducts(products.map((p) => p.id === productId ? { ...p, live_url: e.target.value } : p));
										} catch (error) {
											console.error("Failed to update website URL:", error);
											toast.error(t("mc.error_update_product"));
										}
									}
								}}
							/>
						</div>
						<div className="flex gap-6">
							<label className="flex items-center gap-2 text-sm">
								<input
									type="checkbox"
									checked={product.research_enabled}
									onChange={async (e) => {
										try {
											await updateMCProduct(productId, { research_enabled: e.target.checked });
											setProducts(products.map((p) => p.id === productId ? { ...p, research_enabled: e.target.checked } : p));
										} catch (error) {
											console.error("Failed to toggle research:", error);
											toast.error(t("mc.error_update_product"));
										}
									}}
								/>
								{t("mc.research_enabled")}
							</label>
							<label className="flex items-center gap-2 text-sm">
								<input
									type="checkbox"
									checked={product.ideation_enabled}
									onChange={async (e) => {
										try {
											await updateMCProduct(productId, { ideation_enabled: e.target.checked });
											setProducts(products.map((p) => p.id === productId ? { ...p, ideation_enabled: e.target.checked } : p));
										} catch (error) {
											console.error("Failed to toggle ideation:", error);
											toast.error(t("mc.error_update_product"));
										}
									}}
								/>
								{t("mc.ideation_enabled")}
							</label>
						</div>
					</div>
					<div className="pt-4 border-t">
						<button
							onClick={() => setShowDeleteConfirm(true)}
							className="px-3 py-1.5 text-sm bg-red-600 text-white rounded-md"
						>
							{t("mc.delete_product")}
						</button>
					</div>
				</div>
			)}

			{/* Edit Dialog */}
			<Dialog open={showEdit} onOpenChange={(v) => !v && setShowEdit(false)}>
				<DialogContent className="max-w-md">
					<DialogHeader>
						<DialogTitle>{t("mc.edit_product")}</DialogTitle>
					</DialogHeader>
					<div className="space-y-3">
						<input
							className="w-full text-sm border rounded-md px-3 py-2 outline-none"
							placeholder={t("mc.product_name")}
							value={editName}
							onChange={(e) => setEditName(e.target.value)}
							autoFocus
						/>
						<textarea
							className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
							rows={2}
							placeholder={t("mc.description_optional")}
							value={editDesc}
							onChange={(e) => setEditDesc(e.target.value)}
						/>
						<input
							className="w-full text-sm border rounded-md px-3 py-2 outline-none"
							placeholder={t("mc.repo_url")}
							value={editRepo}
							onChange={(e) => setEditRepo(e.target.value)}
						/>
						<input
							className="w-full text-sm border rounded-md px-3 py-2 outline-none"
							placeholder={t("mc.live_url")}
							value={editWebsite}
							onChange={(e) => setEditWebsite(e.target.value)}
						/>
						<select
							className="w-full text-sm border rounded-md px-3 py-2"
							value={editTier}
							onChange={(e) => setEditTier(e.target.value)}
						>
							<option value="supervised">{t("mc.tier_supervised")}</option>
							<option value="semi_auto">{t("mc.tier_semi_auto")}</option>
							<option value="full_auto">{t("mc.tier_full_auto")}</option>
						</select>
						<div className="flex gap-6">
							<label className="flex items-center gap-2 text-sm">
								<input type="checkbox" checked={editResearch} onChange={(e) => setEditResearch(e.target.checked)} />
								{t("mc.research_enabled")}
							</label>
							<label className="flex items-center gap-2 text-sm">
								<input type="checkbox" checked={editIdeation} onChange={(e) => setEditIdeation(e.target.checked)} />
								{t("mc.ideation_enabled")}
							</label>
						</div>
						<div className="flex justify-end gap-2">
							<button onClick={() => setShowEdit(false)} className="px-3 py-1.5 text-sm border rounded-md">
								{t("mc.cancel")}
							</button>
							<button
								onClick={handleUpdate}
								disabled={saving || !editName.trim()}
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
						<DialogTitle>{t("mc.delete_product")}</DialogTitle>
					</DialogHeader>
					<p className="text-sm text-muted-foreground">
						{t("mc.confirm_delete_product", { name: product.name })}
					</p>
					<div className="flex justify-end gap-2 mt-4">
						<button onClick={() => setShowDeleteConfirm(false)} className="px-3 py-1.5 text-sm border rounded-md">
							{t("mc.cancel")}
						</button>
						<button onClick={handleDelete} className="px-3 py-1.5 text-sm bg-red-600 text-white rounded-md">
							{t("mc.delete")}
						</button>
					</div>
				</DialogContent>
			</Dialog>
		</div>
	);
}

function ResearchCard({ cycle }: { cycle: MCResearchCycle }) {
	const phaseColors: Record<string, string> = {
		init: "bg-muted",
		llm_submitted: "bg-blue-100 dark:bg-blue-900/30",
		llm_polling: "bg-yellow-100 dark:bg-yellow-900/30",
		report_received: "bg-green-100 dark:bg-green-900/30",
		completed: "bg-green-200 dark:bg-green-900/50",
		failed: "bg-red-100 dark:bg-red-900/30",
	};

	return (
		<div className="border rounded-lg p-3">
			<div className="flex justify-between items-center mb-2">
				<span className={`text-xs px-2 py-1 rounded ${phaseColors[cycle.phase] || "bg-muted"}`}>
					{cycle.phase}
				</span>
				<span className="text-xs text-muted-foreground">
					{new Date(cycle.created_at).toLocaleString()}
				</span>
			</div>
			{cycle.report && (
				<div className="text-sm text-muted-foreground mt-2 p-2 bg-muted/50 rounded">
					{cycle.report.substring(0, 200)}...
				</div>
			)}
		</div>
	);
}

function IdeationCard({ cycle }: { cycle: MCIdeationCycle }) {
	const { t } = useTranslation();
	const phaseColors: Record<string, string> = {
		init: "bg-muted",
		generating: "bg-blue-100 dark:bg-blue-900/30",
		filtering: "bg-yellow-100 dark:bg-yellow-900/30",
		completed: "bg-green-200 dark:bg-green-900/50",
		failed: "bg-red-100 dark:bg-red-900/30",
	};

	return (
		<div className="border rounded-lg p-3">
			<div className="flex justify-between items-center">
				<span className={`text-xs px-2 py-1 rounded ${phaseColors[cycle.phase] || "bg-muted"}`}>
					{cycle.phase}
				</span>
				<span className="text-sm font-medium">{cycle.ideas_count} {t("mc.ideas")}</span>
				<span className="text-xs text-muted-foreground">
					{new Date(cycle.created_at).toLocaleString()}
				</span>
			</div>
		</div>
	);
}

function SwipeDeckView({
	deck,
	currentIndex,
	onSwipe,
	stats,
}: {
	deck: MCIdea[];
	currentIndex: number;
	onSwipe: (action: "approve" | "reject" | "maybe") => void;
	stats: { approved: number; rejected: number; maybe: number };
}) {
	const { t } = useTranslation();
	const card = deck[currentIndex];

	if (!card) {
		return (
			<div className="text-center py-12">
				<p className="text-muted-foreground mb-4">{t("mc.no_more_ideas")}</p>
				<div className="flex gap-4 justify-center text-sm">
					<span className="text-green-600">✓ {stats.approved} {t("mc.approved")}</span>
					<span className="text-red-600">✗ {stats.rejected} {t("mc.rejected")}</span>
					<span className="text-yellow-600">~ {stats.maybe} {t("mc.maybe")}</span>
				</div>
			</div>
		);
	}

	return (
		<div className="space-y-4">
			{/* Stats */}
			<div className="flex gap-4 justify-center text-sm">
				<span className="text-green-600">✓ {stats.approved}</span>
				<span className="text-red-600">✗ {stats.rejected}</span>
				<span className="text-yellow-600">~ {stats.maybe}</span>
				<span className="text-muted-foreground">{currentIndex + 1} / {deck.length}</span>
			</div>

			{/* Card */}
			<div className="max-w-md mx-auto rounded-xl shadow-lg border-2 bg-card p-6">
				<div className="mb-2">
					<span className="text-xs px-2 py-1 bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400 rounded">
						{card.category}
					</span>
					<span className="text-xs text-muted-foreground ml-2">
						{t("mc.priority")}: {(card.priority * 100).toFixed(0)}%
					</span>
				</div>

				<h3 className="text-xl font-bold mb-3">{card.title}</h3>
				<p className="text-muted-foreground mb-4">{card.description}</p>

				{/* Actions */}
				<div className="flex gap-3 justify-center">
					<button
						onClick={() => onSwipe("reject")}
						className="w-16 h-16 rounded-full bg-red-100 hover:bg-red-200 text-red-600 dark:bg-red-900/30 dark:hover:bg-red-900/50 dark:text-red-400 text-2xl flex items-center justify-center transition-colors"
					>
						✗
					</button>
					<button
						onClick={() => onSwipe("maybe")}
						className="w-16 h-16 rounded-full bg-yellow-100 hover:bg-yellow-200 text-yellow-600 dark:bg-yellow-900/30 dark:hover:bg-yellow-900/50 dark:text-yellow-400 text-2xl flex items-center justify-center transition-colors"
					>
						~
					</button>
					<button
						onClick={() => onSwipe("approve")}
						className="w-16 h-16 rounded-full bg-green-100 hover:bg-green-200 text-green-600 dark:bg-green-900/30 dark:hover:bg-green-900/50 dark:text-green-400 text-2xl flex items-center justify-center transition-colors"
					>
						✓
					</button>
				</div>
			</div>

			{/* Keyboard hints */}
			<div className="text-center text-xs text-muted-foreground">
				← {t("mc.reject")} | ~ {t("mc.maybe")} | → {t("mc.approve")}
			</div>
		</div>
	);
}