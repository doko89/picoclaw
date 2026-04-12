import { createFileRoute, Outlet } from "@tanstack/react-router"

export const Route = createFileRoute("/mc")({
  component: MCLayout,
})

function MCLayout() {
  return <Outlet />
}