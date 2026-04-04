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
    cursor.innerHTML = `<svg width="32" height="32" viewBox="0 0 32 32">
<path d="M10 7v11.188l2.53-2.442 2.901 5.254 1.765-.941-2.775-5.202h3.604L10 7z" fill="black" stroke="white" stroke-width="1.5"/>
</svg>`;
    // TODO(jeff): stop the cursor from shrinking in size once it traverses into the main frame
    Object.assign(cursor.style, {
      position: 'fixed', top: '0', left: '0', width: '32px', height: '32px',
      zIndex: '1000', pointerEvents: 'none', transform: 'translate(0,0)'
    });
    document.documentElement.appendChild(cursor);
    const path     = cursor.querySelector('path');
    const setGray  = () => { path.style.fill = 'gray'  };
    const setBlack = () => { path.style.fill = 'black' };
    window.mouseX = initialX;
    window.mouseY = initialY;
    const updatePos = (e) => { window.mouseX = e.clientX; window.mouseY = e.clientY }
    window.addEventListener('scroll',    updatePos);
    window.addEventListener('mousemove', updatePos);
    window.addEventListener('mousedown', setGray);
    window.addEventListener('mouseup',   setBlack);
    window.addEventListener('message', (event) => {
      if (event.data.type === 'POINTER_CLICK') {
        if (event.data.action === 'down') setGray();
        if (event.data.action === 'up'  ) setBlack();
      }
    });
    function track() { // animation loop: updates at 60fps to follow the GhostCursor cursor's path
      cursor.style.transform = `translate3d(${window.mouseX}px, ${window.mouseY}px, 0)`;
      requestAnimationFrame(track);
    }
    track();
  }, VIEWPORT.width  / 2, VIEWPORT.height / 2);
}

// register event listeners to track GhostCursor's cursor movements and clicks
export async function registerListeners(page) {
  const frames = page.frames();
  for (const f of frames) {
    if (f === page.mainFrame()) continue; 
    await	f.waitForSelector('body').catch(() => {});
    await f.evaluate(() => {
      const forwardEvent = (type) => {
        window.parent.postMessage({ type: 'POINTER_CLICK', action: type }, '*');
      };
      const updateParentPos = (e) => {
        const rect = window.frameElement.getBoundingClientRect();
        window.parent.mouseX = e.clientX + rect.left;
        window.parent.mouseY = e.clientY + rect.top;
      };
      window.addEventListener('scroll',    updateParentPos);
      window.addEventListener('mousemove', updateParentPos);
      window.addEventListener('mousedown', () => { forwardEvent('down') });
      window.addEventListener('mouseup',   () => { forwardEvent('up'  ) });
    });
  }
}

export async function getAbsoluteCoords(frame, element) {
  return await frame.evaluate((el) => {
    const rect = el.getBoundingClientRect();
    let x = rect.left + rect.width  / 2;
    let y = rect.top  + rect.height / 2;
    let win = window;
    while (win !== window.top) { // walk up through the iframes to the top-level window
      const frameElement = win.frameElement;
      if (frameElement) {
        const frameRect = frameElement.getBoundingClientRect();
        x += frameRect.left;
        y += frameRect.top;
      }
      win = win.parent;
    }
    return { x, y };
  }, element);
}
