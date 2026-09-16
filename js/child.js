try {
  const parent = window.parent.document.documentElement;
  if (parent.getAttribute('theme'))
    document.documentElement.setAttribute('theme', parent.getAttribute('theme'));
  document.body.classList.toggle('line-numbers', parent.getAttribute('line-numbers') !== '0');
} catch {
  console.warn('direct parent access blocked by browser; awaiting postMessage');
}

window.addEventListener('message', (event) => {
  if (event.source !== window.parent || !event.data) return;
  if      (event.data.type === 'TOGGLE_THEME'       ) document.documentElement.setAttribute('theme', event.data.theme);
  else if (event.data.type === 'TOGGLE_LINE_NUMBERS') document.body.classList.toggle('line-numbers', event.data.lineNumbers);
});
