if (window.location.protocol !== 'file:') document.getElementById('buttons').classList.add('visible');

const store = {
  get(key, fallback) { try { return localStorage.getItem(key) ?? fallback; } catch { return fallback; } },
  set(key, value)    { try {        localStorage.setItem(key, value);      } catch {                  } },
};

const codeFrame = document.getElementById('code');

const applyTheme = (theme) => {
  document.documentElement.setAttribute('theme', theme);
  store.set('user-theme', theme);
  codeFrame.contentWindow.postMessage({ type: 'TOGGLE_THEME', theme }, '*');
};

const savedTheme = store.get('user-theme');
if (savedTheme) applyTheme(savedTheme);

document.getElementById('theme').addEventListener('click', () => {
  const current = document.documentElement.getAttribute('theme');
  const isDark  = current === 'dark' || (!current && matchMedia('(prefers-color-scheme: dark)').matches);
  applyTheme(isDark ? 'light' : 'dark');
});

let expanded = false;
document.getElementById('expand').addEventListener('click', () => {
  expanded = !expanded;
  document.getElementById('expand').textContent = expanded ? 'collapse' : 'expand';
  document.querySelectorAll('.tree input[type="checkbox"]').forEach(cb => { cb.checked = expanded; });
});

const applyLineNumbers = (on) => {
  document.documentElement.setAttribute('line-numbers', on ? '1' : '0');
  store.set('line-numbers', on ? '1' : '0');
  codeFrame.contentWindow.postMessage({ type: 'TOGGLE_LINE_NUMBERS', lineNumbers: on }, '*');
};

applyLineNumbers(store.get('line-numbers', '1') === '1');

document.getElementById('lines').addEventListener('click', () => {
  applyLineNumbers(document.documentElement.getAttribute('line-numbers') !== '1');
});
