import { useParams, Link } from 'react-router-dom'

export function EditorLayout() {
  const { projectId } = useParams<{ projectId: string }>()
  return (
    <div style={{ padding: '24px', color: '#e2e8f0', background: '#0b1120', minHeight: '100vh' }}>
      <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h1>ME Lab Workspace — Project {projectId}</h1>
        <Link to="/projects" style={{ color: '#38bdf8' }}>Back to Projects</Link>
      </header>
    </div>
  )
}
