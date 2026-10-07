if (window.location.protocol !== 'file:') document.getElementById('buttons').classList.add('visible');

const store = {
  get(key, fallback) { try { return localStorage.getItem(key) ?? fallback; } catch { return fallback; } },
  set(key, value)    { try {        localStorage.setItem(key, value);      } catch {                  } },
};

const codeFrame = document.getElementById('code');

const applyTheme = (theme) => {
  document.documentElement.setAttribute('theme', theme);
  store.set('user-theme', theme);
  codeFrame.contentWindow.postMessage({ type: 'TOGGLE_THEME', theme }, location.origin);
};

const savedTheme = store.get('user-theme');
if (savedTheme) {
  applyTheme(savedTheme);
} else {
  applyTheme(matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
}

document.getElementById('theme').addEventListener('click', () => {
  applyTheme(document.documentElement.getAttribute('theme') === 'dark' ? 'light' : 'dark');
});

const syncExpandButtonText = () => {
  document.getElementById('expand').textContent = document.querySelector('.tree input[type="checkbox"]:checked') ? 'collapse' : 'expand';
};

document.querySelector('.tree')?.addEventListener('change', syncExpandButtonText);

document.getElementById('expand').addEventListener('click', () => {
  const shouldExpand = !document.querySelector('.tree input[type="checkbox"]:checked');
  document.querySelectorAll('.tree input[type="checkbox"]').forEach(cb => { cb.checked = shouldExpand; });
  syncExpandButtonText();
});

document.querySelector('.dropdown-menu')?.addEventListener('click', (e) => {
  const targetID = e.target.closest('.dropdown-link')?.dataset.for;
  if (!targetID) return;
  const targetLi = document.getElementById(targetID).closest('li');
  targetLi.parentElement.querySelectorAll(':scope > li > input').forEach(cb => { cb.checked = false; }); // uncheck only sibling module roots, not their subtrees
  let el = targetLi;
  while (el) {
    el.querySelector(':scope > input').checked = true;
    el = el.parentElement?.closest('li');
  }
  document.getElementById('modules-menu').checked = false;
  syncExpandButtonText();
});

const applyLineNumbers = (on) => {
  document.documentElement.setAttribute('line-numbers', on ? '1' : '0');
  store.set('line-numbers', on ? '1' : '0');
  codeFrame.contentWindow.postMessage({ type: 'TOGGLE_LINE_NUMBERS', lineNumbers: on }, location.origin);
};

applyLineNumbers(store.get('line-numbers', '1') === '1');

document.getElementById('lines').addEventListener('click', () => {
  applyLineNumbers(document.documentElement.getAttribute('line-numbers') !== '1');
});

document.getElementById('funcs').addEventListener('click', () => {
  const boxes       = codeFrame.contentDocument.querySelectorAll('.func input[type="checkbox"]');
  const shouldCheck = [...boxes].some(b => !b.checked);
  for (const b of boxes) b.checked = shouldCheck;
});
