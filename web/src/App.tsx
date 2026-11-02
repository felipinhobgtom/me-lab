import { Routes, Route, Navigate } from 'react-router-dom'
import { ProjectList } from './features/projects/ProjectList'
import { EditorLayout } from './features/editor/EditorLayout'

function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/projects" replace />} />
      <Route path="/projects" element={<ProjectList />} />
      <Route path="/projects/:projectId/editor" element={<EditorLayout />} />
    </Routes>
  )
}

export default App
