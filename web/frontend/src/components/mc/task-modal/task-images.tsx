import { useEffect, useState, useRef } from "react"
import { getTaskImages, uploadTaskImage, deleteTaskImage, type MCTaskImage } from "@/api/mc"

interface TaskImagesProps {
  taskId: string
}

export function TaskImages({ taskId }: TaskImagesProps) {
  const [images, setImages] = useState<MCTaskImage[]>([])
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    getTaskImages(taskId).then((res) => setImages(res.images)).catch(() => {})
  }, [taskId])

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return

    setUploading(true)
    try {
      await uploadTaskImage(taskId, file)
      const res = await getTaskImages(taskId)
      setImages(res.images)
    } catch {
      // Error handled silently
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ""
    }
  }

  async function handleDelete(filename: string) {
    try {
      await deleteTaskImage(taskId, filename)
      setImages((prev) => prev.filter((img) => img.filename !== filename))
    } catch {
      // Error handled silently
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium">Images</h3>
        <div>
          <input
            ref={fileRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={handleUpload}
          />
          <button
            onClick={() => fileRef.current?.click()}
            disabled={uploading}
            className="text-xs px-2 py-1 border rounded-md hover:bg-accent disabled:opacity-50"
          >
            {uploading ? "Uploading..." : "+ Upload"}
          </button>
        </div>
      </div>

      {images.length === 0 ? (
        <p className="text-sm text-muted-foreground text-center py-4">No images attached</p>
      ) : (
        <div className="grid grid-cols-3 gap-2">
          {images.map((img) => (
            <div key={img.filename} className="relative group rounded-md overflow-hidden border">
              <img
                src={`/api/mc/task-images/${taskId}/${img.filename}`}
                alt={img.original_name}
                className="w-full h-24 object-cover"
              />
              <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                <button
                  onClick={() => handleDelete(img.filename)}
                  className="text-xs text-white bg-red-600 px-2 py-1 rounded-md"
                >
                  Delete
                </button>
              </div>
              <p className="text-xs text-muted-foreground truncate px-1 py-0.5 bg-background">
                {img.original_name}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}