const API = 'http://localhost:8080'

let selectedWorkflowID = null
let historyPage = 1
const HISTORY_LIMIT = 5

// ── Fetch all workflows and render the list ──────────────────────────────────
async function loadWorkflows() {
  // We fetch each workflow by iterating — for a real app you'd have GET /workflows
  // For now we store IDs in localStorage to track created ones
  const ids = getStoredIDs()
  const list = document.getElementById('workflow-list')
  list.innerHTML = ''

  if (ids.length === 0) {
    list.innerHTML = '<li class="empty">No workflows yet.</li>'
    return
  }

  for (const id of ids) {
    try {
      const res = await fetch(`${API}/workflow/${id}`)
      if (!res.ok) continue
      const wf = await res.json()
      renderWorkflowItem(wf)
    } catch {}
  }
}

function renderWorkflowItem(wf) {
  const list = document.getElementById('workflow-list')
  const li = document.createElement('li')
  li.dataset.id = wf.ID
  if (wf.ID === selectedWorkflowID) li.classList.add('active')

  const statusClass = wf.Status === 'approved' ? 'approved' : wf.Status === 'rejected' ? 'rejected' : 'empty'
  const statusText  = wf.Status || 'not executed'

  li.innerHTML = `
    <div class="wf-item-name">${wf.Name}</div>
    <div class="wf-item-status status-${statusClass}">${statusText}</div>
  `
  li.addEventListener('click', () => selectWorkflow(wf.ID))
  list.appendChild(li)
}

// ── Create workflow ──────────────────────────────────────────────────────────
document.getElementById('create-form').addEventListener('submit', async (e) => {
  e.preventDefault()
  const name = document.getElementById('wf-name').value.trim()
  const description = document.getElementById('wf-desc').value.trim()

  const res = await fetch(`${API}/workflow`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, description }),
  })

  if (!res.ok) {
    const err = await res.json()
    alert('Error: ' + err.error)
    return
  }

  const wf = await res.json()
  storeID(wf.ID)

  document.getElementById('wf-name').value = ''
  document.getElementById('wf-desc').value = ''

  await loadWorkflows()
  selectWorkflow(wf.ID)
})

// ── Select and render workflow detail ────────────────────────────────────────
async function selectWorkflow(id) {
  selectedWorkflowID = id
  historyPage = 1

  document.querySelectorAll('#workflow-list li').forEach(li => {
    li.classList.toggle('active', parseInt(li.dataset.id) === id)
  })

  const res = await fetch(`${API}/workflow/${id}`)
  const wf  = await res.json()
  renderDetail(wf)
}

function renderDetail(wf) {
  const panel = document.getElementById('detail-panel')
  const dslText = wf.DSL ? JSON.stringify(wf.DSL, null, 2) : 'No DSL generated yet.'

  panel.innerHTML = `
    <div class="detail-header">
      <div>
        <h2>${wf.Name}</h2>
        <p>${wf.Description}</p>
      </div>
      <div class="detail-actions">
        <button class="secondary" onclick="generateDSL(${wf.ID})">Generate DSL</button>
        <button class="danger" onclick="deleteWorkflow(${wf.ID})">Delete</button>
      </div>
    </div>

    <div class="section">
      <h3>DSL</h3>
      <pre id="dsl-view">${dslText}</pre>
    </div>

    <div class="section">
      <h3>Execute Workflow</h3>
      <div class="execute-form">
        <textarea id="execute-input" placeholder='{"age": 25, "email": "user@example.com"}'></textarea>
        <button onclick="executeWorkflow(${wf.ID})">Run</button>
        <div class="error-msg" id="execute-error"></div>
        <div class="result-box" id="result-box"></div>
      </div>
    </div>

    <div class="section">
      <h3>Execution History</h3>
      <div class="history-controls">
        <span id="history-info" style="font-size:12px;color:var(--muted)"></span>
        <div class="pagination">
          <button class="secondary" id="prev-btn" onclick="changePage(-1)">← Prev</button>
          <span id="page-info"></span>
          <button class="secondary" id="next-btn" onclick="changePage(1)">Next →</button>
        </div>
      </div>
      <div id="history-list"></div>
    </div>
  `

  loadHistory(wf.ID)
}

// ── Generate DSL ─────────────────────────────────────────────────────────────
async function generateDSL(id) {
  const res = await fetch(`${API}/workflow/${id}/generate-dsl`, { method: 'POST' })
  if (!res.ok) {
    const err = await res.json()
    alert('Error: ' + err.error)
    return
  }
  const wf = await res.json()
  document.getElementById('dsl-view').textContent = JSON.stringify(wf.DSL, null, 2)
  await loadWorkflows()
}

// ── Execute workflow ─────────────────────────────────────────────────────────
async function executeWorkflow(id) {
  const raw = document.getElementById('execute-input').value.trim()
  const errorEl = document.getElementById('execute-error')
  errorEl.textContent = ''

  let input
  try {
    input = JSON.parse(raw)
  } catch {
    errorEl.textContent = 'Invalid JSON input'
    return
  }

  const res = await fetch(`${API}/workflow/${id}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })

  const result = await res.json()

  if (!res.ok) {
    errorEl.textContent = result.error
    return
  }

  const box = document.getElementById('result-box')
  box.style.display = 'block'

  const stepsHtml = result.steps.map(s => {
    const cls = s.type === 'action' ? 'action' : s.passed ? 'passed' : 'failed'
    return `
      <div class="step-item ${cls}">
        <div class="step-type">${s.type}${s.action ? ' — ' + s.action : ''}</div>
        <div class="step-msg">${s.message}</div>
      </div>`
  }).join('')

  box.innerHTML = `
    <span class="result-status ${result.status}">${result.status}</span>
    ${stepsHtml}
  `

  await loadWorkflows()
  await loadHistory(id)
}

// ── Execution history ────────────────────────────────────────────────────────
async function loadHistory(id) {
  const res = await fetch(`${API}/workflow/${id}/executions?page=${historyPage}&limit=${HISTORY_LIMIT}`)
  const data = await res.json()

  document.getElementById('history-info').textContent = `${data.total} total runs`
  document.getElementById('page-info').textContent = `${data.page} / ${data.total_pages || 1}`
  document.getElementById('prev-btn').disabled = historyPage <= 1
  document.getElementById('next-btn').disabled = historyPage >= (data.total_pages || 1)

  const list = document.getElementById('history-list')
  if (!data.data || data.data.length === 0) {
    list.innerHTML = '<p style="color:var(--muted);font-size:12px">No executions yet.</p>'
    return
  }

  list.innerHTML = data.data.map(log => {
    const stepsHtml = (log.steps || []).map(s => {
      const cls = s.type === 'action' ? 'action' : s.passed ? 'passed' : 'failed'
      return `
        <div class="step-item ${cls}">
          <div class="step-type">${s.type}${s.action ? ' — ' + s.action : ''}</div>
          <div class="step-msg">${s.message}</div>
        </div>`
    }).join('')

    return `
      <div class="log-item">
        <div class="log-item-header">
          <span class="tag ${log.status}">${log.status}</span>
          <span class="log-date">${new Date(log.created_at).toLocaleString()}</span>
        </div>
        <details>
          <summary style="cursor:pointer;font-size:12px;color:var(--muted);margin-bottom:6px">Input data</summary>
          <pre>${JSON.stringify(log.input, null, 2)}</pre>
        </details>
        <div style="margin-top:8px">${stepsHtml}</div>
      </div>`
  }).join('')
}

function changePage(dir) {
  historyPage += dir
  loadHistory(selectedWorkflowID)
}

// ── Delete workflow ──────────────────────────────────────────────────────────
async function deleteWorkflow(id) {
  if (!confirm('Delete this workflow?')) return

  await fetch(`${API}/workflow/${id}`, { method: 'DELETE' })
  removeID(id)
  selectedWorkflowID = null
  document.getElementById('detail-panel').innerHTML = '<div class="placeholder"><p>Select a workflow to view details</p></div>'
  await loadWorkflows()
}

// ── LocalStorage helpers ─────────────────────────────────────────────────────
function getStoredIDs() {
  return JSON.parse(localStorage.getItem('workflow_ids') || '[]')
}
function storeID(id) {
  const ids = getStoredIDs()
  if (!ids.includes(id)) ids.unshift(id)
  localStorage.setItem('workflow_ids', JSON.stringify(ids))
}
function removeID(id) {
  const ids = getStoredIDs().filter(i => i !== id)
  localStorage.setItem('workflow_ids', JSON.stringify(ids))
}

// ── Init ─────────────────────────────────────────────────────────────────────
loadWorkflows()