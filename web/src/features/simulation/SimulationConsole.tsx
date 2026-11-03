import { useMemo, useState } from 'react'
import { selectActiveOverlay, useEditorStore } from '../editor/store'
import './SimulationConsole.css'

type ConsoleTab = 'diagnostics' | 'events' | 'jobs' | 'telemetry'

export function SimulationConsole() {
  const diagnostics = useEditorStore((s) => s.diagnostics)
  const selectedDiagnosticId = useEditorStore((s) => s.selectedDiagnosticId)
  const highlightedCoords = useEditorStore((s) => s.highlightedCoords)
  const events = useEditorStore((s) => s.events)
  const simulationRun = useEditorStore((s) => s.simulationRun)
  const actionError = useEditorStore((s) => s.actionError)

  const selectDiagnostic = useEditorStore((s) => s.selectDiagnostic)
  const clearHighlights = useEditorStore((s) => s.clearHighlights)

  const [tab, setTab] = useState<ConsoleTab>('diagnostics')
  const [collapsed, setCollapsed] = useState<boolean>(false)

  const activeOverlay = useMemo(
    () => selectActiveOverlay({ simulationRun, validationResult: null }),
    [simulationRun]
  )

  const activeSummary = simulationRun?.summary ?? null
  const craftingJobs = activeOverlay?.crafting_jobs ?? []
  const networks = activeOverlay?.networks ?? []
  const storageCells = activeOverlay?.storage_cells ?? []
  const storedTotals = activeOverlay?.stored_totals ?? []

  const sortedEvents = useMemo(
    () =>
      [...events].sort((a, b) =>
        a.tick !== b.tick ? a.tick - b.tick : a.sequence - b.sequence
      ),
    [events]
  )

  return (
    <section
      className={`simulation-console ${collapsed ? 'collapsed' : ''}`}
      aria-label="ME Network Runtime"
    >
      <div className="sim-control-bar">
        <div className="sim-request-group">
          <span className="sim-request-label">ME NETWORK RUNTIME</span>
          <span className="badge badge-exact">AUTOMATIC</span>
          <span className="sim-runtime-note">20 TPS · power, channels, storage and crafting update live</span>
        </div>
        <div className="sim-status-group">
          {simulationRun && (
            <>
              <span className="badge">RUNTIME: {simulationRun.status.toUpperCase()}</span>
              <span className="badge">TICK {simulationRun.current_tick}</span>
            </>
          )}
          <button
            type="button"
            className="btn"
            onClick={() => setCollapsed((v) => !v)}
          >
            {collapsed ? '▲ Expand' : '▼ Collapse'}
          </button>
        </div>
      </div>

      {!collapsed && (
        <>
          <div className="sim-tabs-row">
            <div className="sim-tabs" role="tablist" aria-label="ME Runtime Tabs">
              <button
                type="button"
                role="tab"
                aria-selected={tab === 'diagnostics'}
                className={`sim-tab ${tab === 'diagnostics' ? 'active' : ''}`}
                onClick={() => setTab('diagnostics')}
              >
                Diagnostics ({diagnostics.length})
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={tab === 'events'}
                className={`sim-tab ${tab === 'events' ? 'active' : ''}`}
                onClick={() => setTab('events')}
              >
                Event Timeline ({sortedEvents.length})
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={tab === 'jobs'}
                className={`sim-tab ${tab === 'jobs' ? 'active' : ''}`}
                onClick={() => setTab('jobs')}
              >
                Crafting Jobs ({craftingJobs.length})
              </button>
              <button
                type="button"
                role="tab"
                aria-selected={tab === 'telemetry'}
                className={`sim-tab ${tab === 'telemetry' ? 'active' : ''}`}
                onClick={() => setTab('telemetry')}
              >
                Telemetry (Channels / Energy / Storage)
              </button>
            </div>

            {highlightedCoords.length > 0 && (
              <button
                type="button"
                className="btn btn-danger"
                style={{ padding: '2px 8px', fontSize: 11 }}
                onClick={clearHighlights}
              >
                Clear Highlight ({highlightedCoords.length})
              </button>
            )}
          </div>

          {actionError && (
            <div className="sim-error-strip" role="alert">
              {actionError}
            </div>
          )}

          <div className="sim-panel-body">
            {/* Diagnostics Tab */}
            {tab === 'diagnostics' && (
              <>
                {diagnostics.length === 0 ? (
                  <div className="sim-empty-note">
                    {simulationRun
                      ? 'Zero runtime diagnostics reported — the live AE2 network is operating without faults.'
                      : 'Runtime is initializing from the saved layout.'}
                  </div>
                ) : (
                  <div className="diag-list">
                    {diagnostics.map((diag) => {
                      const isSelected = selectedDiagnosticId === diag.id
                      return (
                        <button
                          key={diag.id || `${diag.code}-${diag.message}`}
                          type="button"
                          data-testid={`diagnostic-item-${diag.code}`}
                          className={`diag-row ${isSelected ? 'selected' : ''}`}
                          onClick={() => selectDiagnostic(diag)}
                        >
                          <div className="diag-row-top">
                            <span className={`badge diag-sev-${diag.severity}`}>
                              {diag.severity.toUpperCase()}
                            </span>
                            <span className="diag-code">{diag.code}</span>
                            {diag.blocks_simulation && (
                              <span className="badge badge-unsupported">BLOCKS SIM</span>
                            )}
                            <div className="diag-coords">
                              {(diag.affected_blocks ?? []).map((ab, idx) => (
                                <span
                                  key={`${ab.id}-${idx}`}
                                  className="badge"
                                >
                                  ({ab.x}, {ab.y}, {ab.z})
                                </span>
                              ))}
                            </div>
                          </div>
                          <div className="diag-msg">{diag.message}</div>
                          {diag.remediation && (
                            <div className="diag-remediation">
                              Remediation: {diag.remediation}
                            </div>
                          )}
                        </button>
                      )
                    })}
                  </div>
                )}
              </>
            )}

            {/* Event Timeline Tab */}
            {tab === 'events' && (
              <>
                {sortedEvents.length === 0 ? (
                  <div className="sim-empty-note">
                    No runtime events recorded yet. The live network will append events as ticks advance.
                  </div>
                ) : (
                  <table className="events-table">
                    <thead>
                      <tr>
                        <th>Tick</th>
                        <th>Seq</th>
                        <th>Event Type</th>
                        <th>Payload Details</th>
                      </tr>
                    </thead>
                    <tbody>
                      {sortedEvents.map((ev) => (
                        <tr key={`${ev.tick}-${ev.sequence}-${ev.id}`}>
                          <td className="mono">{ev.tick}</td>
                          <td className="mono">#{ev.sequence}</td>
                          <td>
                            <span className="badge">{ev.event_type}</span>
                          </td>
                          <td className="event-payload">
                            {JSON.stringify(ev.payload)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </>
            )}

            {/* Crafting Jobs Tab */}
            {tab === 'jobs' && (
              <>
                {craftingJobs.length === 0 ? (
                  <div className="sim-empty-note">
                    No active or completed Crafting CPU jobs in the live runtime.
                  </div>
                ) : (
                  <div className="telemetry-grid">
                    {craftingJobs.map((job) => {
                      const batchPct =
                        job.total_batches > 0
                          ? Math.min(100, (job.completed_crafts / job.total_batches) * 100)
                          : 0
                      const bytePct =
                        job.bytes_capacity > 0
                          ? Math.min(100, (job.bytes_used / job.bytes_capacity) * 100)
                          : 0
                      return (
                        <div key={job.id} className="telemetry-card">
                          <div className="telemetry-card-header">
                            <span className="mono" style={{ fontWeight: 700 }}>
                              {job.target_item} × {job.target_quantity}
                            </span>
                            <span
                              className={`badge ${
                                job.status === 'completed'
                                  ? 'badge-exact'
                                  : job.status === 'failed'
                                    ? 'badge-unsupported'
                                    : 'badge-approximated'
                              }`}
                            >
                              {job.status.toUpperCase()}
                            </span>
                          </div>

                          <div className="telemetry-kv">
                            <span>CPU ID</span>
                            <span>{job.cpu_id || '—'}</span>
                          </div>
                          <div className="telemetry-kv">
                            <span>Co-Processors</span>
                            <span>{job.co_processors}</span>
                          </div>
                          <div className="telemetry-kv">
                            <span>CPU Bytes</span>
                            <span>
                              {job.bytes_used} / {job.bytes_capacity} B
                            </span>
                          </div>
                          <div className="meter-bar-track">
                            <div
                              className="meter-bar-fill"
                              style={{
                                width: `${bytePct}%`,
                                background: 'var(--accent-fluix)',
                              }}
                            />
                          </div>

                          <div className="telemetry-kv">
                            <span>Batches Dispatched</span>
                            <span>
                              {job.completed_crafts} / {job.total_batches}
                            </span>
                          </div>
                          <div className="meter-bar-track">
                            <div
                              className="meter-bar-fill"
                              style={{
                                width: `${batchPct}%`,
                                background: 'var(--accent-certus)',
                              }}
                            />
                          </div>

                          {job.failure_reason && (
                            <div style={{ color: '#fca5a5', fontSize: 11 }}>
                              Reason: {job.failure_reason}
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                )}
              </>
            )}

            {/* Telemetry Tab */}
            {tab === 'telemetry' && (
              <>
                {!activeSummary ? (
                  <div className="sim-empty-note">
                    The live runtime is still initializing channel, energy, storage and crafting state.
                  </div>
                ) : (
                  <div className="telemetry-grid">
                    {/* Global Summary Card */}
                    <div className="telemetry-card">
                      <div className="telemetry-card-header">
                        <span style={{ fontWeight: 700 }}>Global Network Summary</span>
                        <span className="badge badge-exact">
                          {activeSummary.online_networks}/{activeSummary.networks} ONLINE
                        </span>
                      </div>
                      <div className="telemetry-kv">
                        <span>Channels Used</span>
                        <span>
                          {activeSummary.channels_used} / {activeSummary.channels_available}
                        </span>
                      </div>
                      <div className="telemetry-kv">
                        <span>Energy Gen vs Drain</span>
                        <span>
                          +{activeSummary.energy_generation.toFixed(1)} / -
                          {activeSummary.energy_consumption.toFixed(1)} AE/t
                        </span>
                      </div>
                      <div className="telemetry-kv">
                        <span>Energy Buffer</span>
                        <span>
                          {Math.round(activeSummary.energy_stored)} /{' '}
                          {Math.round(activeSummary.energy_capacity)} AE
                        </span>
                      </div>
                      <div className="meter-bar-track">
                        <div
                          className="meter-bar-fill"
                          style={{
                            width: `${
                              activeSummary.energy_capacity > 0
                                ? Math.min(
                                    100,
                                    (activeSummary.energy_stored /
                                      activeSummary.energy_capacity) *
                                      100
                                  )
                                : 0
                            }%`,
                            background: '#10b981',
                          }}
                        />
                      </div>
                      <div className="telemetry-kv">
                        <span>Items Stored</span>
                        <span>{activeSummary.items_stored} items</span>
                      </div>
                    </div>

                    {/* Per-Network Cards */}
                    {networks.map((net) => (
                      <div key={net.id} className="telemetry-card">
                        <div className="telemetry-card-header">
                          <span className="mono" style={{ fontWeight: 700 }}>
                            Network {net.id}
                          </span>
                          <span
                            className={`badge ${
                              net.online ? 'badge-exact' : 'badge-unsupported'
                            }`}
                          >
                            {net.online ? 'ONLINE' : 'OFFLINE'}
                          </span>
                        </div>
                        <div className="telemetry-kv">
                          <span>Topology</span>
                          <span>
                            {net.has_controller
                              ? net.controller_valid
                                ? 'Controller'
                                : 'Controller Conflict'
                              : 'Ad-Hoc (Max 8 Ch)'}
                          </span>
                        </div>
                        <div className="telemetry-kv">
                          <span>Blocks / Channels</span>
                          <span>
                            {net.block_count} blk • {net.channels_used}/{net.channels_available}{' '}
                            ch
                          </span>
                        </div>
                        <div className="telemetry-kv">
                          <span>AE/t Balance</span>
                          <span>
                            +{net.energy_generation.toFixed(1)} / -
                            {net.energy_consumption.toFixed(1)} AE/t
                          </span>
                        </div>
                      </div>
                    ))}

                    {/* Stored Item Totals Card */}
                    <div className="telemetry-card">
                      <div className="telemetry-card-header">
                        <span style={{ fontWeight: 700 }}>ME Storage Inventory</span>
                        <span className="badge">{storedTotals.length} Types</span>
                      </div>
                      {storedTotals.length === 0 ? (
                        <div className="telemetry-kv">
                          <span>Empty</span>
                          <span>0</span>
                        </div>
                      ) : (
                        storedTotals.map((st) => (
                          <div key={st.item_id} className="telemetry-kv">
                            <span>{st.item_id}</span>
                            <span>×{st.quantity}</span>
                          </div>
                        ))
                      )}
                    </div>

                    {/* Drive Storage Cell Cards */}
                    {storageCells.map((cell) => {
                      const bytePct =
                        cell.bytes_total > 0
                          ? Math.min(100, (cell.bytes_used / cell.bytes_total) * 100)
                          : 0
                      return (
                        <div
                          key={`${cell.drive_block_id}-slot-${cell.slot}`}
                          className="telemetry-card"
                        >
                          <div className="telemetry-card-header">
                            <span className="mono" style={{ fontWeight: 700 }}>
                              Drive ({cell.drive_coord.x},{cell.drive_coord.y},
                              {cell.drive_coord.z}) #{cell.slot + 1}
                            </span>
                            <span className="badge">
                              {cell.tier.toUpperCase()} • Pri {cell.priority}
                            </span>
                          </div>
                          <div className="telemetry-kv">
                            <span>Bytes Used</span>
                            <span>
                              {cell.bytes_used} / {cell.bytes_total} B
                            </span>
                          </div>
                          <div className="meter-bar-track">
                            <div
                              className="meter-bar-fill"
                              style={{
                                width: `${bytePct}%`,
                                background: 'var(--accent-fluix)',
                              }}
                            />
                          </div>
                          <div className="telemetry-kv">
                            <span>Types Occupied</span>
                            <span>
                              {cell.types_used} / {cell.types_total}
                            </span>
                          </div>
                          <div className="telemetry-cell-items" aria-label="Cell contents">
                            {cell.items.length === 0 ? (
                              <span className="telemetry-muted">Empty cell</span>
                            ) : (
                              cell.items.map((item) => (
                                <div key={item.item_id} className="telemetry-kv">
                                  <span className="mono">{item.item_id}</span>
                                  <span>×{item.quantity}</span>
                                </div>
                              ))
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                )}
              </>
            )}
          </div>
        </>
      )}
    </section>
  )
}
