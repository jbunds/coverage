import puppeteer from 'puppeteer';

export const URL      = 'http://127.0.0.1:8000';
export const VIEWPORT = {
  deviceScaleFactor: 1,
  width:             1280,
  height:            720 };

export async function launchChrome(url = URL) {
  const options = {
    headless:        false,
    defaultViewport: null,
    executablePath:  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    args: [
      '--window-size=' + VIEWPORT.width + ',' + VIEWPORT.height,
      '--disable-web-security',
      '--disable-features=IsolateOrigins,site-per-process',
      '--disable-blink-features=AutomationControlled',
      url,
    ],
  };
  return await puppeteer.launch(options);
}

export async function createMousePointer(page) {
  await page.evaluate((initialX = 0, initialY = 0) => {
    const cursor = document.createElement('div');
    cursor.id        = 'custom-pointer';
    cursor.innerHTML = `<svg width="32" height="32" viewBox="0 0 32 32"><path d="M10 7v11.188l2.53-2.442 2.901 5.254 1.765-.941-2.775-5.202h3.604L10 7z" fill="black" stroke="white" stroke-width="1.5"/></svg>`;
    Object.assign(cursor.style, {
      position: 'fixed', top: '0', left: '0', width: '32px', height: '32px',
      zIndex: '100', pointerEvents: 'none', transform: 'translate(0,0)'
    });
    document.documentElement.appendChild(cursor);
    window.mouseX = initialX;
    window.mouseY = initialY;
    const updatePos = (e) => { window.mouseX = e.clientX; window.mouseY = e.clientY }
    window.addEventListener('scroll',    updatePos);
    window.addEventListener('mousemove', updatePos);
    window.addEventListener('mousedown', () => { cursor.querySelector('path').style.fill = 'gray'  });
    window.addEventListener('mouseup',   () => { cursor.querySelector('path').style.fill = 'black' });
    function track() { // animation loop: updates at 60fps to follow the GhostCursor cursor's path
      cursor.style.transform = `translate3d(${window.mouseX}px, ${window.mouseY}px, 0)`;
      requestAnimationFrame(track);
    }
    track();
  }, VIEWPORT.width  / 2, VIEWPORT.height / 2);
}

// register event listeners to track GhostCursor's cursor movements
export async function registerListeners(page) {
  const frames = page.frames();
  for (const f of frames) {
    if (f === page.mainFrame()) continue; // skip the main frame (handled above)
    await f.evaluate(() => {
      window.addEventListener('scroll', (e) => {
        const rect = window.frameElement.getBoundingClientRect();
        window.parent.mouseX = e.clientX + rect.left;
        window.parent.mouseY = e.clientY + rect.top;
      });
      window.addEventListener('mousemove', (e) => {
        const rect = window.frameElement.getBoundingClientRect();
        window.parent.mouseX = e.clientX + rect.left;
        window.parent.mouseY = e.clientY + rect.top;
      });
    });
  }
}
