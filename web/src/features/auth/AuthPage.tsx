import { type FormEvent, useEffect, useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { api, extractApiErrorMessage } from '../../lib/api'
import { useAuthStore } from './authStore'
import './AuthPage.css'

type AuthTab = 'login' | 'register'

export function AuthPage() {
  const navigate = useNavigate()
  const token = useAuthStore((s) => s.token)
  const setSession = useAuthStore((s) => s.setSession)

  const [tab, setTab] = useState<AuthTab>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [apiStatus, setApiStatus] = useState<string>('AE2 1.21.1 CAD')

  useEffect(() => {
    let active = true
    api
      .health()
      .then((res) => {
        if (active && res?.status === 'ok') {
          setApiStatus(`API v${res.version} ONLINE`)
        }
      })
      .catch(() => {
        // Ignore health probe error when offline or in unit tests
      })
    return () => {
      active = false
    }
  }, [])

  if (token || api.getToken()) {
    return <Navigate to="/projects" replace />
  }

  const switchTab = (next: AuthTab) => {
    setTab(next)
    setError(null)
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)

    const trimmedEmail = email.trim()
    if (!trimmedEmail || !password) {
      setError('Email and password are required.')
      return
    }

    if (tab === 'register') {
      if (password.length < 8) {
        setError('Password must be at least 8 characters long.')
        return
      }
      if (confirmPassword && password !== confirmPassword) {
        setError('Passwords do not match.')
        return
      }
    }

    setSubmitting(true)
    try {
      const response =
        tab === 'login'
          ? await api.login(trimmedEmail, password)
          : await api.register(trimmedEmail, password)
      setSession(response.token, response.user)
      navigate('/projects', { replace: true })
    } catch (err) {
      setError(
        extractApiErrorMessage(
          err,
          tab === 'login' ? 'Unable to sign in with these credentials.' : 'Account creation failed.'
        )
      )
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <header className="auth-brand">
          <div className="auth-brand-top">
            <div className="auth-logo-row">
              <span className="auth-core-glyph" aria-hidden="true">
                ME
              </span>
              <h1>ME Lab</h1>
            </div>
            <span className="badge">{apiStatus}</span>
          </div>
          <p className="auth-subtitle">
            Applied Energistics 2 Network Designer &amp; Deterministic Simulator
          </p>
        </header>

        <div className="auth-tabs" role="tablist" aria-label="Authentication Mode">
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'login'}
            className={`auth-tab ${tab === 'login' ? 'active' : ''}`}
            onClick={() => switchTab('login')}
          >
            Sign In
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={tab === 'register'}
            className={`auth-tab ${tab === 'register' ? 'active' : ''}`}
            onClick={() => switchTab('register')}
          >
            Create Account
          </button>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          {error && (
            <div className="auth-error-banner" role="alert">
              {error}
            </div>
          )}

          <div className="auth-field">
            <label htmlFor="auth-email">Engineer Email</label>
            <input
              id="auth-email"
              type="email"
              autoComplete="email"
              placeholder="engineer@melab.local"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
            />
          </div>

          <div className="auth-field">
            <label htmlFor="auth-password">Password</label>
            <input
              id="auth-password"
              type="password"
              autoComplete={tab === 'login' ? 'current-password' : 'new-password'}
              placeholder={tab === 'register' ? 'Minimum 8 characters' : '••••••••'}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </div>

          {tab === 'register' && (
            <div className="auth-field">
              <label htmlFor="auth-confirm-password">Confirm Password</label>
              <input
                id="auth-confirm-password"
                type="password"
                autoComplete="new-password"
                placeholder="Repeat password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
              />
            </div>
          )}

          <button type="submit" className="btn btn-primary auth-submit" disabled={submitting}>
            {submitting
              ? tab === 'login'
                ? 'Authenticating…'
                : 'Creating Account…'
              : tab === 'login'
                ? 'Sign In to Workspace'
                : 'Create Account & Continue'}
          </button>
        </form>

        <footer className="auth-footer">
          <span>MC 1.21.1 / AE2 19.x</span>
          <span>PURE PROCEDURAL CAD</span>
        </footer>
      </div>
    </div>
  )
}
