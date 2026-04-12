import { createFileRoute, Link } from "@tanstack/react-router";
import { Suspense, useEffect, useState } from "react";
import { useAtom } from "jotai";
import { IconPlus } from "@tabler/icons-react";
import { getMCProducts, createMCProduct } from "@/api/mc-products";
import type { MCProduct } from "@/api/mc-products";
import { mcProductsAtom, mcLoadingProductsAtom } from "@/store/mc-autopilot";
import { toast } from "sonner";
import { useTranslation } from "react-i18next";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

export const Route = createFileRoute("/mc/autopilot/")({
  component: AutopilotIndex,
});

function AutopilotIndex() {
  const { t } = useTranslation();
  return (
    <div className="p-6">
      <h1 className="text-2xl font-bold mb-6">{t("mc.autopilot_dashboard")}</h1>

      <Suspense fallback={<div className="text-muted-foreground">{t("mc.loading_products")}</div>}>
        <AutopilotContent />
      </Suspense>
    </div>
  );
}

function AutopilotContent() {
  const { t } = useTranslation();
  const [products, setProducts] = useAtom(mcProductsAtom);
  const [loading, setLoading] = useAtom(mcLoadingProductsAtom);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    loadProducts();
  }, []);

  async function loadProducts() {
    setLoading(true);
    try {
      const data = await getMCProducts();
      setProducts(data);
    } catch (error) {
      console.error("Failed to load products:", error);
      toast.error(t("mc.error_fetch_products"));
    }
    setLoading(false);
  }

  async function handleCreateProduct() {
    if (!newName.trim()) return;
    setCreating(true);
    try {
      const newProduct = await createMCProduct({
        name: newName.trim(),
        description: newDesc.trim() || undefined,
        automation_tier: "supervised",
        research_enabled: true,
        ideation_enabled: true,
      });
      setProducts([...products, newProduct]);
      setNewName("");
      setNewDesc("");
      setShowCreate(false);
    } catch (error) {
      console.error("Failed to create product:", error);
      toast.error(t("mc.error_create_product"));
    }
    setCreating(false);
  }

  if (loading && products.length === 0) {
    return <div className="text-muted-foreground">{t("mc.loading_products")}</div>;
  }

  if (products.length === 0) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground mb-4">{t("mc.no_products_yet")}</p>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md"
        >
          <IconPlus size={16} />
          {t("mc.create_product")}
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-lg font-semibold">{t("mc.products")} ({products.length})</h2>
        <button
          onClick={() => setShowCreate(true)}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md"
        >
          <IconPlus size={16} />
          {t("mc.new_product")}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {products.map((product) => (
          <ProductCard key={product.id} product={product} />
        ))}
      </div>

      <Dialog open={showCreate} onOpenChange={(v) => !v && setShowCreate(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("mc.create_product")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <input
              className="w-full text-sm border rounded-md px-3 py-2 outline-none"
              placeholder={t("mc.product_name")}
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              autoFocus
            />
            <textarea
              className="w-full text-sm border rounded-md px-3 py-2 outline-none resize-none"
              rows={2}
              placeholder={t("mc.description_optional")}
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowCreate(false)}
                className="px-3 py-1.5 text-sm border rounded-md"
              >
                {t("mc.cancel")}
              </button>
              <button
                onClick={handleCreateProduct}
                disabled={creating || !newName.trim()}
                className="px-3 py-1.5 text-sm bg-primary text-primary-foreground rounded-md disabled:opacity-50"
              >
                {creating ? t("mc.creating") : t("mc.create")}
              </button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function ProductCard({ product }: { product: MCProduct }) {
  const { t } = useTranslation();
  const tierColors: Record<string, string> = {
    supervised: "bg-muted text-muted-foreground",
    semi_auto: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400",
    full_auto: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
  };

  return (
    <Link
      to="/mc/autopilot/$productId"
      params={{ productId: product.id }}
      className="rounded-lg border bg-card p-4 hover:bg-accent/50 transition-colors block"
    >
      <div className="flex justify-between items-start mb-2">
        <h3 className="font-semibold text-lg">{product.name}</h3>
        <span className={`text-xs px-2 py-1 rounded ${tierColors[product.automation_tier] || "bg-muted text-muted-foreground"}`}>
          {product.automation_tier === "full_auto" ? t("mc.tier_full_auto") : product.automation_tier === "semi_auto" ? t("mc.tier_semi_auto") : t("mc.tier_supervised")}
        </span>
      </div>

      {product.description && (
        <p className="text-sm text-muted-foreground mb-3 line-clamp-2">{product.description}</p>
      )}

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        {product.research_enabled && <span className="text-blue-600">Research ✓</span>}
        {product.ideation_enabled && <span className="text-purple-600">Ideation ✓</span>}
        {product.repo_url && <span>📦</span>}
        {product.live_url && <span>🌐</span>}
      </div>
    </Link>
  );
}