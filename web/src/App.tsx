import type { ReactNode } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { AuthPage } from './features/auth/AuthPage'
import { useAuthStore } from './features/auth/authStore'
import { ProjectList } from './features/projects/ProjectList'
import { EditorLayout } from './features/editor/EditorLayout'
import { api } from './lib/api'

function RequireAuth({ children }: { children: ReactNode }) {
  const token = useAuthStore((s) => s.token)
  const hasToken = Boolean(token || api.getToken())
  if (!hasToken) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<AuthPage />} />
      <Route path="/" element={<Navigate to="/projects" replace />} />
      <Route
        path="/projects"
        element={
          <RequireAuth>
            <ProjectList />
          </RequireAuth>
        }
      />
      <Route
        path="/projects/:projectId/editor"
        element={
          <RequireAuth>
            <EditorLayout />
          </RequireAuth>
        }
      />
      <Route path="*" element={<Navigate to="/projects" replace />} />
    </Routes>
  )
}

export default App
