import { type FormEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  api,
  extractApiErrorMessage,
  type CompatibilityProfile,
  type Project,
} from '../../lib/api'
import { useAuthStore } from '../auth/authStore'
import {
  buildChannelBottleneckFixtureOps,
  buildValidAutocraftingFixtureOps,
} from './fixtures'
import './ProjectList.css'

const DEFAULT_PROFILE_ID = 'ae2-1.21.1-default'

export function ProjectList() {
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const clearSession = useAuthStore((s) => s.clearSession)

  const [projects, setProjects] = useState<Project[]>([])
  const [profiles, setProfiles] = useState<CompatibilityProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [busyAction, setBusyAction] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const [showNewForm, setShowNewForm] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [selectedProfileId, setSelectedProfileId] = useState(DEFAULT_PROFILE_ID)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [projRes, profRes] = await Promise.all([
        api.listProjects(1, 50),
        api.listProfiles().catch(() => [] as CompatibilityProfile[]),
      ])
      setProjects(projRes.data ?? [])
      setProfiles(profRes)
      if (profRes.length > 0) {
        const preferred =
          profRes.find((p) => p.id === DEFAULT_PROFILE_ID) ?? profRes[0]
        setSelectedProfileId(preferred.id)
      }
    } catch (err) {
      setError(extractApiErrorMessage(err, 'Failed to load projects'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchAll()
    if (!user && api.getToken()) {
      api
        .me()
        .then((u) => setUser(u))
        .catch(() => {
          // Ignore if /auth/me fails during offline/mock runs
        })
    }
  }, [fetchAll, setUser, user])

  const resolveDefaultProfileId = (): string => {
    const match = profiles.find((p) => p.id === DEFAULT_PROFILE_ID)
    if (match) {
      return match.id
    }
    return profiles[0]?.id ?? DEFAULT_PROFILE_ID
  }

  const formatProfileBadge = (profileId: string): string => {
    const found = profiles.find((p) => p.id === profileId)
    if (found) {
      return `${found.name} (${found.minecraft_version})`
    }
    return profileId
  }

  const handleCreateBlankProject = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = newName.trim()
    if (!trimmed) {
      setError('Project name is required.')
      return
    }
    setBusyAction('create')
    setError(null)
    try {
      const created = await api.createProject({
        name: trimmed,
        description: newDescription.trim(),
        profile_id: selectedProfileId || resolveDefaultProfileId(),
      })
      setNewName('')
      setNewDescription('')
      setShowNewForm(false)
      navigate(`/projects/${created.id}/editor`)
    } catch (err) {
      setError(extractApiErrorMessage(err, 'Failed to create project'))
    } finally {
      setBusyAction(null)
    }
  }

  const handleLoadAutocraftingFixture = async () => {
    setBusyAction('fixture-valid')
    setError(null)
    try {
      const project = await api.createProject({
        name: 'AE2 1.21.1 Autocrafting Reference Network',
        description:
          'Controller + Dense/Smart trunk + 4k Drive + 1x2x1 Crafting CPU + Pattern Provider & dual Molecular Assemblers.',
        profile_id: resolveDefaultProfileId(),
      })
      await api.batchBlocks(project.id, buildValidAutocraftingFixtureOps(), true)
      navigate(`/projects/${project.id}/editor`)
    } catch (err) {
      setError(extractApiErrorMessage(err, 'Failed to load Valid Autocrafting Fixture'))
    } finally {
      setBusyAction(null)
    }
  }

  const handleLoadBottleneckFixture = async () => {
    setBusyAction('fixture-fault')
    setError(null)
    try {
      const project = await api.createProject({
        name: 'Channel Bottleneck Fault Study (Ad-Hoc)',
        description:
          'Controllerless ad-hoc network with 10 channel-consuming devices exceeding the 8-channel limit.',
        profile_id: resolveDefaultProfileId(),
      })
      await api.batchBlocks(project.id, buildChannelBottleneckFixtureOps(), true)
      navigate(`/projects/${project.id}/editor`)
    } catch (err) {
      setError(extractApiErrorMessage(err, 'Failed to load Channel Bottleneck Fault Fixture'))
    } finally {
      setBusyAction(null)
    }
  }

  const handleDeleteProject = async (project: Project) => {
    setBusyAction(`delete-${project.id}`)
    setError(null)
    try {
      await api.deleteProject(project.id)
      setProjects((prev) => prev.filter((p) => p.id !== project.id))
    } catch (err) {
      setError(extractApiErrorMessage(err, 'Failed to delete project'))
    } finally {
      setBusyAction(null)
    }
  }

  const handleLogout = async () => {
    await api.logout()
    clearSession()
    navigate('/login', { replace: true })
  }

  return (
    <div className="project-list-page">
      <header className="project-list-header">
        <div className="project-brand-group">
          <span className="project-brand-mark" aria-hidden="true">
            ME
          </span>
          <div>
            <h1>ME Lab</h1>
            <p className="subtitle">AE2 Network Designer &amp; Deterministic Simulator</p>
          </div>
        </div>

        <div className="project-header-actions">
          {user?.email && <span className="project-user-pill">{user.email}</span>}
          <button type="button" className="btn" onClick={() => void handleLogout()}>
            Sign Out
          </button>
        </div>
      </header>

      <main className="project-list-content">
        <div className="project-list-toolbar">
          <h2>Workspace Projects</h2>
          <div className="project-toolbar-buttons">
            <button
              type="button"
              className="btn btn-certus"
              disabled={busyAction !== null}
              onClick={() => void handleLoadAutocraftingFixture()}
            >
              {busyAction === 'fixture-valid'
                ? 'Provisioning Fixture…'
                : 'Load Valid Autocrafting Fixture'}
            </button>
            <button
              type="button"
              className="btn btn-danger"
              disabled={busyAction !== null}
              onClick={() => void handleLoadBottleneckFixture()}
            >
              {busyAction === 'fixture-fault'
                ? 'Provisioning Fault Fixture…'
                : 'Load Channel Bottleneck Fault Fixture'}
            </button>
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => setShowNewForm((v) => !v)}
            >
              {showNewForm ? 'Cancel' : '+ New Project'}
            </button>
          </div>
        </div>

        {error && (
          <div className="project-error-banner" role="alert">
            {error}
          </div>
        )}

        {showNewForm && (
          <form className="new-project-panel" onSubmit={handleCreateBlankProject}>
            <h3>Create Blank AE2 Network Project</h3>
            <div className="new-project-grid">
              <div className="form-col">
                <label htmlFor="new-project-name">Project Name</label>
                <input
                  id="new-project-name"
                  type="text"
                  placeholder="e.g. Base Mainframe Subnetwork"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  required
                />
              </div>

              <div className="form-col">
                <label htmlFor="new-project-desc">Description</label>
                <input
                  id="new-project-desc"
                  type="text"
                  placeholder="Optional architecture notes"
                  value={newDescription}
                  onChange={(e) => setNewDescription(e.target.value)}
                />
              </div>

              <div className="form-col">
                <label htmlFor="new-project-profile">Compatibility Profile</label>
                <select
                  id="new-project-profile"
                  value={selectedProfileId}
                  onChange={(e) => setSelectedProfileId(e.target.value)}
                >
                  {profiles.length > 0 ? (
                    profiles.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} — MC {p.minecraft_version} ({p.channel_mode})
                      </option>
                    ))
                  ) : (
                    <option value={DEFAULT_PROFILE_ID}>
                      AE2 1.21.1 Default (8/32 Channels)
                    </option>
                  )}
                </select>
              </div>

              <div className="form-actions">
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={busyAction !== null}
                >
                  {busyAction === 'create' ? 'Creating…' : 'Create & Open'}
                </button>
              </div>
            </div>
          </form>
        )}

        {loading ? (
          <div className="empty-state">
            <div className="empty-glyph">…</div>
            <h3>Loading persisted projects</h3>
            <p>Querying local SQLite workspace store…</p>
          </div>
        ) : projects.length === 0 ? (
          <div className="empty-state">
            <div className="empty-glyph">XYZ</div>
            <h3>No projects yet</h3>
            <p>
              Create a blank network or load one of the two pre-wired AE2 1.21.1 acceptance
              fixtures above to start designing and simulating immediately.
            </p>
          </div>
        ) : (
          <div className="project-grid">
            {projects.map((project) => (
              <article key={project.id} className="project-card">
                <div>
                  <div className="project-card-top">
                    <h3 className="project-card-title">{project.name}</h3>
                    <span className="badge">{formatProfileBadge(project.profile_id)}</span>
                  </div>
                  <p className="project-card-desc">
                    {project.description || 'No description provided.'}
                  </p>
                </div>

                <div className="project-card-meta">
                  <span>
                    Updated{' '}
                    {project.updated_at
                      ? new Date(project.updated_at).toLocaleString()
                      : 'recently'}
                  </span>
                  <div className="project-card-actions">
                    <button
                      type="button"
                      className="btn btn-danger"
                      disabled={busyAction === `delete-${project.id}`}
                      onClick={() => void handleDeleteProject(project)}
                    >
                      Delete
                    </button>
                    <button
                      type="button"
                      className="btn btn-primary"
                      onClick={() => navigate(`/projects/${project.id}/editor`)}
                    >
                      Open Editor
                    </button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </main>
    </div>
  )
}
