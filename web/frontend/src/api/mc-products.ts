// MC Products API client
import { request } from "./mc";

export interface MCProduct {
	id: string;
	workspace_id: string;
	name: string;
	description: string;
	repo_url: string;
	live_url: string;
	product_program?: string;
	icon: string;
	status: string;
	build_mode: string;
	automation_tier: string;
	research_enabled: boolean;
	ideation_enabled: boolean;
	default_branch: string;
	cost_cap_per_task?: number;
	cost_cap_monthly?: number;
	health_weight_config?: string;
	batch_review_threshold: number;
	settings?: any;
	created_at: string;
	updated_at: string;
}

export interface MCIdeationCycle {
	id: string;
	product_id: string;
	research_cycle_id?: string;
	phase: string;
	ideas_count: number;
	created_at: string;
	updated_at: string;
	completed_at?: string;
}

export interface MCIdea {
	id: string;
	product_id: string;
	title: string;
	description: string;
	category: string;
	priority: number;
	source: string;
	status: string;
	suppressed: boolean;
	created_at: string;
	updated_at: string;
}

export interface MCResearchCycle {
	id: string;
	product_id: string;
	phase: string;
	query: string;
	report: string;
	sources: string[];
	variant: string;
	parent_cycle_id?: string;
	created_at: string;
	updated_at: string;
	completed_at?: string;
}

export async function getMCProducts(): Promise<MCProduct[]> {
	return request<MCProduct[]>("/products");
}

export async function createMCProduct(product: {
	workspace_id?: string;
	name: string;
	description?: string;
	repo_url?: string;
	live_url?: string;
	build_mode?: string;
	automation_tier?: string;
	research_enabled?: boolean;
	ideation_enabled?: boolean;
	default_branch?: string;
}): Promise<MCProduct> {
	return request<MCProduct>("/products", {
		method: "POST",
		body: JSON.stringify(product),
	});
}

export async function updateMCProduct(
	id: string,
	updates: Partial<Pick<MCProduct, "name" | "description" | "repo_url" | "live_url" | "status" | "build_mode" | "automation_tier" | "research_enabled" | "ideation_enabled" | "default_branch">>
): Promise<MCProduct> {
	return request(`/products/${id}`, {
		method: "PUT",
		body: JSON.stringify(updates),
	});
}

export async function deleteMCProduct(id: string): Promise<void> {
	return request<void>(`/products/${id}`, { method: "DELETE" });
}

export async function generateProductDescription(id: string): Promise<{ description: string }> {
	return request<{ description: string }>(`/products/${id}/generate-description`, { method: "POST" });
}

// Research
export async function runMCResearch(productId: string, options: {
	existing_cycle_id?: string;
	chain_ideation?: boolean;
}): Promise<{ cycle_id: string }> {
	return request<{ cycle_id: string }>(`/products/${productId}/research/run`, {
		method: "POST",
		body: JSON.stringify(options),
	});
}

export async function getMCResearchCycles(productId: string): Promise<MCResearchCycle[]> {
	return request<MCResearchCycle[]>(`/products/${productId}/research/cycles`);
}

// Ideation
export async function runMCIdeation(productId: string, options: {
	research_cycle_id?: string;
}): Promise<{ cycle_id: string }> {
	return request<{ cycle_id: string }>(`/products/${productId}/ideation/run`, {
		method: "POST",
		body: JSON.stringify(options),
	});
}

export async function getMCIdeationCycles(productId: string): Promise<MCIdeationCycle[]> {
	return request<MCIdeationCycle[]>(`/products/${productId}/ideation/cycles`);
}

export async function getMCIdeas(productId: string): Promise<MCIdea[]> {
	return request<MCIdea[]>(`/products/${productId}/ideas`);
}

export async function swipeIdea(ideaId: string, data: {
	product_id: string;
	action: "approve" | "reject" | "maybe";
	notes?: string;
}): Promise<{ success: boolean }> {
	return request<{ success: boolean }>(`/ideas/${ideaId}/swipe`, {
		method: "POST",
		body: JSON.stringify(data),
	});
}

export async function getMCSwipeDeck(productId: string): Promise<MCIdea[]> {
	return request<MCIdea[]>(`/products/${productId}/swipe-deck`);
}
