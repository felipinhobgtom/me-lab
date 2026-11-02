import { Routes, Route, Navigate } from 'react-router-dom'
import { EditorLayout } from './features/editor/EditorLayout'

function App() {
  return (
    <Routes>
      <Route path="/" element={<EditorLayout />} />
      <Route path="/projects/:projectId/editor" element={<EditorLayout />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default App
