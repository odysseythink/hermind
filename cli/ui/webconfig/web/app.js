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

function renderForm() {
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
  main.appendChild(panel);
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
  status('saved — restart hermind to apply', 'saved');
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
