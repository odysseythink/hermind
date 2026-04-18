let schema = [];
let values = {};
let currentSection = null;

async function boot() {
  schema = await (await fetch('/api/schema')).json();
  values = await (await fetch('/api/config')).json();
  const sections = [...new Set(schema.map(f => f.section))];
  currentSection = sections[0];
  const nav = document.getElementById('sections');
  sections.forEach(name => {
    const d = document.createElement('div');
    d.textContent = name;
    d.onclick = () => { currentSection = name; renderForm(); renderNav(sections); };
    nav.appendChild(d);
  });
  renderNav(sections); renderForm();
  document.getElementById('save').onclick = () => save(false);
  document.getElementById('save-exit').onclick = () => save(true);
}

function renderNav(sections) {
  const nav = document.getElementById('sections');
  [...nav.children].forEach((d, i) => {
    d.classList.toggle('active', sections[i] === currentSection);
  });
}

// Hide fields under `<section>.<sub>.*` when a sibling `<section>.provider` enum
// exists and does not equal `<sub>`. Keeps the form focused on the currently
// selected memory/browser provider instead of listing every provider's keys.
function isVisible(f) {
  const parts = f.path.split('.');
  if (parts.length < 3) return true;
  const providerPath = parts[0] + '.provider';
  const providerField = schema.find(x => x.path === providerPath && x.kind === 4);
  if (!providerField) return true;
  return (values[providerPath] ?? '') === parts[1];
}

async function renderForm() {
  const main = document.getElementById('form');
  main.innerHTML = '';
  const panel = document.createElement('section');
  panel.className = 'panel';
  const header = document.createElement('header');
  header.className = 'panel-header';
  const title = document.createElement('h2');
  title.className = 'section-title';
  title.textContent = currentSection;
  header.appendChild(title);
  panel.appendChild(header);
  main.appendChild(panel);
  if (currentSection === 'Providers') {
    await renderProviders(panel);
    return;
  }
  schema.filter(f => f.section === currentSection).forEach(f => {
    if (!isVisible(f)) return;
    const wrap = document.createElement('label');
    const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = f.label;
    wrap.appendChild(lbl);
    wrap.appendChild(renderField(f));
    if (f.help) {
      const help = document.createElement('span');
      help.className = 'help';
      help.textContent = f.help;
      wrap.appendChild(help);
    }
    panel.appendChild(wrap);
  });
}

async function renderProviders(panel) {
  const r = await fetch('/api/providers');
  if (!r.ok) { status('error: ' + await r.text(), 'error'); return; }
  const list = await r.json();
  list.forEach(p => panel.appendChild(renderProviderCard(p)));
  const addBtn = document.createElement('button');
  addBtn.type = 'button';
  addBtn.className = 'btn secondary provider-add';
  addBtn.textContent = '+ Add provider';
  addBtn.onclick = async () => {
    const key = (prompt('Provider key (letters, digits, _ or -):') || '').trim();
    if (!key) return;
    const resp = await fetch('/api/providers', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({op: 'add', key}),
    });
    if (!resp.ok) { status('error: ' + await resp.text(), 'error'); return; }
    status('unsaved changes', 'unsaved');
    renderForm();
  };
  panel.appendChild(addBtn);
}

function renderProviderCard(p) {
  const card = document.createElement('div');
  card.className = 'provider-card';

  const keyRow = document.createElement('div');
  keyRow.className = 'provider-key-row';
  const keyLabel = document.createElement('span');
  keyLabel.className = 'provider-key';
  keyLabel.textContent = p.key;
  keyRow.appendChild(keyLabel);
  const del = document.createElement('button');
  del.type = 'button';
  del.className = 'btn secondary provider-delete';
  del.textContent = 'Delete';
  del.onclick = async () => {
    if (!confirm('Remove provider "' + p.key + '"?')) return;
    const resp = await fetch('/api/providers', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({op: 'delete', key: p.key}),
    });
    if (!resp.ok) { status('error: ' + await resp.text(), 'error'); return; }
    status('unsaved changes', 'unsaved');
    renderForm();
  };
  keyRow.appendChild(del);
  card.appendChild(keyRow);

  card.appendChild(providerRow('Provider type', p, 'provider', 'text'));
  card.appendChild(providerRow('Base URL', p, 'base_url', 'text'));
  card.appendChild(providerRow('API key', p, 'api_key', 'secret'));
  card.appendChild(providerRow('Model', p, 'model', 'model'));
  return card;
}

function providerRow(label, p, field, kind) {
  const wrap = document.createElement('label');
  const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = label;
  wrap.appendChild(lbl);
  if (kind === 'secret') {
    const box = document.createElement('span');
    box.className = 'secret-wrap';
    const inp = document.createElement('input');
    inp.type = 'password';
    inp.value = p[field] || '';
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'reveal-btn';
    btn.textContent = 'Show';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {
          method: 'POST',
          body: JSON.stringify({path: 'providers.' + p.key + '.api_key'}),
        });
        if (r.ok) {
          const b = await r.json();
          inp.value = b.value;
          inp.type = 'text';
          btn.textContent = 'Hide';
        }
      } else {
        inp.type = 'password';
        btn.textContent = 'Show';
      }
    };
    inp.oninput = () => updateGetBtn(p.key, inp.value);
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    wrap.appendChild(box);
  } else if (kind === 'model') {
    const row = document.createElement('span');
    row.className = 'model-row';
    const inp = document.createElement('input');
    inp.type = 'text';
    inp.value = p[field] || '';
    inp.setAttribute('list', 'models-' + p.key);
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    const dl = document.createElement('datalist');
    dl.id = 'models-' + p.key;
    row.appendChild(inp);
    row.appendChild(dl);
    if (!UNSUPPORTED_LIST_MODELS.has(p.provider)) {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'get-models-btn';
      btn.id = 'get-btn-' + p.key;
      btn.textContent = 'Get';
      const hasKey = (p.api_key && p.api_key.length > 0);
      if (hasKey) btn.classList.add('active');
      btn.onclick = () => fetchModels(p, dl, btn, row);
      row.appendChild(btn);
    }
    wrap.appendChild(row);
  } else {
    const inp = document.createElement('input');
    inp.type = 'text';
    inp.value = p[field] || '';
    inp.onchange = () => persistProviderField(p.key, field, inp.value);
    wrap.appendChild(inp);
  }
  return wrap;
}

async function persistProviderField(key, field, value) {
  const r = await fetch('/api/providers', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({op: 'set', key, field, value}),
  });
  if (!r.ok) { status('error: ' + await r.text(), 'error'); return; }
  status('unsaved changes', 'unsaved');
}

// Providers whose backend doesn't implement provider.ModelLister.
// Kept in sync with server-side type assertions.
const UNSUPPORTED_LIST_MODELS = new Set(['zhipu', 'wenxin']);

// updateGetBtn is called from the api_key input's oninput handler so
// the Get button reacts live (no server roundtrip).
function updateGetBtn(key, apiKeyValue) {
  const btn = document.getElementById('get-btn-' + key);
  if (!btn) return;
  if (apiKeyValue && apiKeyValue.length > 0) {
    btn.classList.add('active');
  } else {
    btn.classList.remove('active');
  }
}

async function fetchModels(p, datalist, btn, row) {
  if (!btn.classList.contains('active')) return;
  row.querySelectorAll('.inline-error').forEach(el => el.remove());
  const prevText = btn.textContent;
  btn.textContent = 'Loading…';
  btn.disabled = true;
  // Blur any active input so its onchange fires and persistProviderField
  // commits the current value. We do NOT iterate every input on the card:
  // the api_key field's displayed value may be the "••••" mask sentinel
  // that came back from /api/providers, and persisting that would overwrite
  // the real key in the in-memory doc. Rely on the browser's natural
  // blur→onchange→persist chain; if the persist hasn't round-tripped by the
  // time we fetch, the user retries. The race window is ~a few ms on
  // loopback.
  if (document.activeElement && typeof document.activeElement.blur === 'function') {
    document.activeElement.blur();
  }
  try {
    const r = await fetch('/api/providers/models', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify({key: p.key}),
    });
    if (!r.ok) {
      const msg = await r.text();
      const err = document.createElement('span');
      err.className = 'inline-error';
      err.textContent = msg.trim();
      row.appendChild(err);
      btn.textContent = prevText;
      btn.disabled = false;
      return;
    }
    const body = await r.json();
    while (datalist.firstChild) datalist.removeChild(datalist.firstChild);
    for (const id of body.models || []) {
      const opt = document.createElement('option');
      opt.value = id;
      datalist.appendChild(opt);
    }
    btn.textContent = 'Got ' + (body.models || []).length;
    setTimeout(() => {
      btn.textContent = prevText;
      btn.disabled = false;
    }, 1000);
  } catch (e) {
    const err = document.createElement('span');
    err.className = 'inline-error';
    err.textContent = String(e);
    row.appendChild(err);
    btn.textContent = prevText;
    btn.disabled = false;
  }
}

// Kind constants mirror Go's iota order in schema.go:
// 0=String 1=Int 2=Float 3=Bool 4=Enum 5=Secret 6=List
function renderField(f) {
  const cur = values[f.path] ?? '';
  if (f.kind === 4) {
    const sel = document.createElement('select');
    (f.enum || []).forEach(v => { const o = document.createElement('option'); o.value = v; o.textContent = v || '(none)'; sel.appendChild(o); });
    sel.value = cur;
    sel.onchange = async () => {
      await persist(f.path, sel.value);
      if (f.path.endsWith('.provider')) renderForm();
    };
    return sel;
  }
  if (f.kind === 3) {
    const cb = document.createElement('input'); cb.type = 'checkbox';
    cb.checked = cur === 'true' || cur === true;
    cb.onchange = () => persist(f.path, cb.checked);
    return cb;
  }
  if (f.kind === 5) {
    const box = document.createElement('span');
    box.className = 'secret-wrap';
    const inp = document.createElement('input'); inp.type = 'password'; inp.value = cur;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'reveal-btn';
    btn.textContent = 'Show';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {method:'POST', body: JSON.stringify({path: f.path})});
        if (r.ok) {
          const b = await r.json();
          inp.value = b.value;
          inp.type = 'text';
          btn.textContent = 'Hide';
        }
      } else {
        inp.type = 'password';
        btn.textContent = 'Show';
      }
    };
    inp.onchange = () => persist(f.path, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    return box;
  }
  if (f.kind === 6) {
    const note = document.createElement('span');
    note.className = 'list-note';
    note.textContent = '(edit via YAML or TUI)';
    return note;
  }
  const inp = document.createElement('input');
  inp.type = (f.kind === 1 || f.kind === 2) ? 'number' : 'text';
  inp.value = cur;
  inp.onchange = () => {
    let v = inp.value;
    if (f.kind === 1) v = parseInt(v, 10);
    else if (f.kind === 2) v = parseFloat(v);
    persist(f.path, v);
  };
  return inp;
}

async function persist(path, value) {
  const r = await fetch('/api/config', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({path, value})
  });
  if (!r.ok) { status('error: ' + await r.text(), 'error'); return; }
  values[path] = value;
  status('unsaved changes', 'unsaved');
}

async function save(exit) {
  const r = await fetch('/api/save', {method:'POST'});
  if (!r.ok) { status('save failed', 'error'); return; }
  status('saved — applied on next message', 'saved');
  if (exit) await fetch('/api/shutdown', {method:'POST'});
}

let _flashTimer = null;
function status(s, state) {
  const st = state || 'idle';
  const el = document.getElementById('status');
  el.querySelector('.msg').textContent = s;
  el.className = 'status ' + st;
  const topDot = document.getElementById('topbar-dot');
  if (topDot) topDot.className = 'dot ' + st;
  if (st === 'error') {
    const f = document.querySelector('footer');
    if (f) {
      f.classList.add('flash-error');
      clearTimeout(_flashTimer);
      _flashTimer = setTimeout(() => f.classList.remove('flash-error'), 1000);
    }
  }
}
boot();
