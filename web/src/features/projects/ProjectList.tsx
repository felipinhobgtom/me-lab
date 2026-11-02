import { useState } from 'react'
import './ProjectList.css'

export function ProjectList() {
  const [_projects] = useState<Array<{ id: string; name: string }>>([])

  return (
    <div className="project-list-page">
      <header className="project-list-header">
        <h1>ME Lab</h1>
        <p className="subtitle">AE2 Network Designer &amp; Simulator</p>
      </header>

      <main className="project-list-content">
        <div className="project-list-toolbar">
          <h2>Your Projects</h2>
          <button className="btn btn-primary">+ New Project</button>
        </div>

        {_projects.length === 0 ? (
          <div className="empty-state">
            <div className="empty-icon">📐</div>
            <h3>No projects yet</h3>
            <p>Create your first AE2 network design to get started.</p>
          </div>
        ) : (
          <div className="project-grid">
            {/* Project cards will go here */}
          </div>
        )}
      </main>
    </div>
  )
}
