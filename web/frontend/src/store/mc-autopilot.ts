// Mission Control Autopilot state management with Jotai
import { atom } from "jotai";

// Products
export const mcProductsAtom = atom<MCProduct[]>([]);
export const mcSelectedProductIdAtom = atom<string | null>(null);

// Derived: Selected product
export const mcSelectedProductAtom = atom((get) => {
	const products = get(mcProductsAtom);
	const selectedId = get(mcSelectedProductIdAtom);
	return products.find((p) => p.id === selectedId) || null;
});

// Research cycles for selected product
export const mcResearchCyclesAtom = atom<MCResearchCycle[]>([]);

// Ideation cycles for selected product
export const mcIdeationCyclesAtom = atom<MCIdeationCycle[]>([]);

// Swipe deck
export const mcSwipeDeckAtom = atom<MCIdea[]>([]);
export const mcCurrentSwipeIndexAtom = atom(0);

// Derived: Current swipe card
export const mCurrentSwipeCardAtom = atom((get) => {
	const deck = get(mcSwipeDeckAtom);
	const index = get(mcCurrentSwipeIndexAtom);
	return deck[index] || null;
});

// Swipe stats
export const mcSwipeStatsAtom = atom<{
	approved: number;
	rejected: number;
	maybe: number;
}>({ approved: 0, rejected: 0, maybe: 0 });

// Loading states
export const mcLoadingProductsAtom = atom(false);
export const mcLoadingResearchAtom = atom(false);
export const mcLoadingIdeationAtom = atom(false);
export const mcRunningResearchAtom = atom(false);
export const mcRunningIdeationAtom = atom(false);

// Import types
import type { MCProduct, MCResearchCycle, MCIdeationCycle, MCIdea } from "@/api/mc-products";
