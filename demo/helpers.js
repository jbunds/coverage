import puppeteer from 'puppeteer';

export {
  URL,
  MOUSE_SPEED,
  VIEWPORT,
  launchChrome,
  installMouse,
  interactWith,
  scrollTo,
  scrollToBottom,
  typeWithRandomDelays,
  moveMouse,
  moveMouseNaturally,
  performVisualClick,
  //performNaturalClick,
};

const URL         = 'http://127.0.0.1:8000';
const MOUSE_SPEED = 8;
const VIEWPORT    = {
  deviceScaleFactor: 1,
  width:             1280,
  height:            720 };

// global state; tracks the position of the mouse pointer as it moves
let currentPos = {
  x: VIEWPORT.width  / 2,
  y: VIEWPORT.height / 2 };

// helpers
async function installMouse(page, initialX = currentPos.x, initialY = currentPos.y) {
  await page.evaluateOnNewDocument((x, y) => {
    if (window !== window.parent) return;
    window.addEventListener('DOMContentLoaded', () => {
      const box = document.createElement('puppeteer-mouse-pointer');
      box.innerHTML = `
        <svg width="32" height="32" viewBox="0 0 32 32">
          <path d="M10 7v11.188l2.53-2.442 2.901 5.254 1.765-.941-2.775-5.202h3.604L10 7z" fill="black" stroke="white" stroke-width="1.5" stroke-linejoin="round"/>
        </svg>
      `;
      box.style.pointerEvents = 'none';
      box.style.position      = 'fixed';
      box.style.zIndex        = '100';
      box.style.width         = '25px';
      box.style.height        = '25px';
      box.style.left          = (x - 3) + 'px';
      box.style.top           = (y - 3) + 'px';
      box.style.transition    = 'fill 0.1s ease';
      document.body.appendChild(box);

      window.updatePuppeteerCursor = (newX, newY, clicking = false) => {
        box.style.left = (newX - 3) + 'px';
        box.style.top  = (newY - 3) + 'px';
        const path = box.querySelector('path');
        if (clicking) path.style.fill = 'gray';
        else          path.style.fill = 'black';
      };
    
      //window.addEventListener('message', (event) => {
      //  if (event.data.type === 'UPDATE_MOUSE') {
      //    window.updatePuppeteerCursor(event.data.x, event.data.y);
      //  } else if (event.data.type === 'MOUSE_DOWN') {
      //    window.updatePuppeteerCursor(null, null, true);
      //  } else if (event.data.type === 'MOUSE_UP') {
      //    window.updatePuppeteerCursor(null, null, false);
      //  }
      //});
    });
  }, initialX, initialY);
  await page.evaluate(() => {
    document.addEventListener('mousemove', (e) => {
      window.mouseX = e.clientX;
      window.mouseY = e.clientY;
    });
  });
}

//async function performNaturalClick(page) {
//  await page.evaluate(() => { window.postMessage({ type: 'MOUSE_DOWN' }, '*') });
//  await new Promise(r => setTimeout(r,  50 + Math.random() *  50));
//  await page.evaluate(() => { window.postMessage({ type: 'MOUSE_UP'   }, '*') });
//  await new Promise(r => setTimeout(r, 100 + Math.random() * 100));
//}

// bezier curve implementation for natural mouse movement
function getBezierPoint(t, p0, p1, p2) {
  const x = (1 - t) * (1 - t) * p0.x + 2 * (1 - t) * t * p1.x + t * t * p2.x;
  const y = (1 - t) * (1 - t) * p0.y + 2 * (1 - t) * t * p1.y + t * t * p2.y;
  return { x, y };
}

async function moveMouse(page, targetX, targetY, pixelsPerStep = MOUSE_SPEED) {
  const startX = currentPos.x;
  const startY = currentPos.y;

  // control point for quadratic bezier to make it a curve instead of a line
  // pick a point somewhat offset from the midpoint
  const midX = (startX + targetX) / 2;
  const midY = (startY + targetY) / 2;
  const cpX  = midX + (Math.random() - 0.5) * Math.abs(targetX - startX) * 0.4;
  const cpY  = midY + (Math.random() - 0.5) * Math.abs(targetY - startY) * 0.4;

  const distance = Math.hypot(targetX - startX, targetY - startY);
  const steps    = Math.max(5, Math.floor(distance / pixelsPerStep));
  
  console.log(`moving to (${targetX.toFixed(1)}, ${targetY.toFixed(1)}) | steps: ${steps}`);
  
  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    // use an easing function for more natural speed (accelerate/decelerate)
    const easedT = t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t;
    const { x, y } = getBezierPoint(easedT, { x: startX, y: startY }, { x: cpX, y: cpY }, { x: targetX, y: targetY });
    
    await page.mouse.move(x, y);
    await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y); }, x, y);
    
    // variable delay to mimic human interaction
    if (i % 3 === 0) await new Promise(r => setTimeout(r, 5 + Math.random() * 5));
  }
  
  await page.mouse.move(targetX, targetY);
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y); }, targetX, targetY);
  currentPos = { x: targetX, y: targetY };
  await new Promise(r => setTimeout(r, 150 + Math.random() * 100)); 
}

async function moveMouseNaturally(page, targetX, targetY) {
  const { mouseX: startX, mouseY: startY } = await page.evaluate(() => ({
    mouseX: window.mouseX,
    mouseY: window.mouseY,
  }));

  const midX = (startX + targetX) / 2;
  const midY = (startY + targetY) / 2;
  const cpX  = midX + (Math.random() - 0.5) * Math.abs(targetX - startX) * 0.3;
  const cpY  = midY + (Math.random() - 0.5) * Math.abs(targetY - startY) * 0.3;

  const distance = Math.hypot(targetX - startX, targetY - startY);
  const steps    = Math.max(10, Math.floor(distance / 12));

  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    const easedT = t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t;
    const { x, y } = getBezierPoint(easedT, { x: startX, y: startY }, { x: cpX, y: cpY }, { x: targetX, y: targetY });

    await page.mouse.move(x, y);

    await page.evaluate((pos) => {
      window.postMessage({ type: 'UPDATE_MOUSE', x: pos.x, y: pos.y }, '*');
    }, { x, y });

    if (i % 4 === 0) await new Promise(r => setTimeout(r, 8 + Math.random() * 5));
  }
}

//async function performVisualClick(page, x, y) {
//  await page.evaluate((x, y) => { window.postMessage({ type: 'MOUSE_DOWN', x, y }, '*') }, x, y);
//  await new Promise(r => setTimeout(r,  50 + Math.random() *  50));
//  await page.evaluate((x, y) => { window.postMessage({ type: 'MOUSE_UP', x, y }, '*') }, x, y);
//  await new Promise(r => setTimeout(r, 100 + Math.random() * 100));
//}

async function performVisualClick(page, x, y) {
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y, true); }, x, y);
  await page.mouse.down();
  await new Promise(r => setTimeout(r,  50 + Math.random() *  50));
  await page.mouse.up();
  await new Promise(r => setTimeout(r, 100 + Math.random() * 100));
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y, false); }, x, y);
}

async function typeWithRandomDelays(page, text, delay = 100) {
  for (const char of text) {
    await page.keyboard.type(char, { delay: delay + (Math.random() - 0.5) * delay * 0.5 });
    if (Math.random() > 0.9) await new Promise(r => setTimeout(r, 200 + Math.random() * 300)); // random pause
  }
}

async function launchChrome(url = URL) {
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

async function interactWith(recording, page, selector, options = {}) {
  const defaults = {
    iframeSelector: null,
    type:           'click',
    pixelsPerStep:  MOUSE_SPEED,
    frameIndex:     null,
    scrollIntoView: true,
    paddingX:       0,
    paddingY:       0,
    waitBefore:     1000,
    waitAfter:      1000,
    clickAtX:       null,
    clickAtY:       null
  };
  const {
    iframeSelector, type, pixelsPerStep, frameIndex, scrollIntoView,
    paddingX, paddingY, waitBefore, waitAfter, clickAtX, clickAtY
  } = { ...defaults, ...options };

  console.log(`interacting with ${selector}${iframeSelector ? ` in ${iframeSelector}` : ''}...`);

  let frame = page;
  if (iframeSelector) {
    const iframeHandle = await page.locator(iframeSelector).waitHandle();
    frame = await iframeHandle.contentFrame();
    if (!frame) throw new Error(`cannot get contentFrame for ${iframeSelector}`);
  }

  const element = await frame.locator(selector).waitHandle();

  if (scrollIntoView) {
    await element.evaluate(el => el.scrollIntoView({ block: 'center', behavior: 'smooth' }));
    await new Promise(r => setTimeout(r, 600 + Math.random() * 200));
  }

  const rect = await element.boundingBox();
  if (!rect) throw new Error(`null bounding box for ${selector}`);
  
  // add some randomness to the click point (within a 5px radius of center or specified point)
  const jitterX = (Math.random() - 0.5) * 4;
  const jitterY = (Math.random() - 0.5) * 4;
  
  const targetX = (clickAtX !== null ? rect.x + clickAtX : rect.x + rect.width / 2 + paddingX) + jitterX;
  const targetY = (clickAtY !== null ? rect.y + clickAtY : rect.y + rect.height / 2 + paddingY) + jitterY;

  await moveMouse(page, targetX, targetY, pixelsPerStep);
  
  const selectors = iframeSelector ? [[iframeSelector, selector]] : [[selector]];
  const waitBeforeFinal = waitBefore + (Math.random() - 0.5) * 200;
  const waitAfterFinal  = waitAfter  + (Math.random() - 0.5) * 200;

  if (type !== 'none') {
    recording.steps.push({
      selectors,
      type:   'hover',
      target: 'main',
      ...(frameIndex !== null ? { frame: [frameIndex] } : {})
    });
    await new Promise(r => setTimeout(r, waitBeforeFinal));
  }

  if (type === 'click') {
    console.log(`clicking ${selector}...`);
    await performVisualClick(page, targetX, targetY);
    recording.steps.push({
      selectors,
      type:   'click',
      target: 'main',
      offsetX: targetX - rect.x,
      offsetY: targetY - rect.y,
      ...(frameIndex !== null ? { frame: [frameIndex] } : {})
    });
  }

  await new Promise(r => setTimeout(r, waitAfterFinal));
}

async function scrollTo(recording, page, iframeSelector, elementSelector, options = {}) {
  const { frameIndex = null } = options;
  console.log(`scrolling in ${iframeSelector} to ${elementSelector}...`);

  const iframeHandle = await page.locator(iframeSelector).waitHandle();
  const frame        = await iframeHandle.contentFrame();
  if (!frame) throw new Error(`cannot get contentFrame for ${iframeSelector}`);

  await frame.evaluate((sel) => {
    document.querySelector(sel)?.scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, elementSelector);
  
  await new Promise(r => setTimeout(r, 800 + Math.random() * 300));

  const scrollY = await frame.evaluate(() => window.scrollY);

  recording.steps.push({
    selectors: [[iframeSelector, 'body']],
    type:      'scroll',
    target:    'main',
    x: 0,
    y: scrollY,
    ...(frameIndex !== null ? { frame: [frameIndex] } : {})
  });
}

async function scrollToBottom(recording, page, iframeSelector, options = {}) {
  const {
    frameIndex    = null,
    pixelsPerStep = MOUSE_SPEED,
    waitBefore    = 1000,
    waitAfter     = 1000
  } = options;

  console.log(`scrolling down ${iframeSelector} completely...`);
  
  const iframeHandle = await page.locator(iframeSelector).waitHandle();
  const frame        = await iframeHandle.contentFrame();
  if (!frame) throw new Error(`cannot get contentFrame for ${iframeSelector}`);
  
  const rect   = await iframeHandle.boundingBox();
  const entryX = rect.x + 200 + (Math.random() - 0.5) * 100;
  const entryY = rect.y + 300 + (Math.random() - 0.5) * 100;
  await moveMouse(page, entryX, entryY, pixelsPerStep);
  await new Promise(r => setTimeout(r, waitBefore));

  const scrollHeight = await frame.evaluate(async () => {
    const totalHeight    = document.documentElement.scrollHeight;
    const viewportHeight = window.innerHeight;
    let currentPos = window.scrollY;
    while(currentPos < (totalHeight - viewportHeight)) {
      const step = 50 + Math.random() * 100;
      window.scrollBy({ top: step, behavior: 'smooth' });
      currentPos += step;
      await new Promise(r => setTimeout(r, 150 + Math.random() * 100));
    }
    window.scrollTo({ top: totalHeight, behavior: 'smooth' });
    return totalHeight;
  });

  await new Promise(r => setTimeout(r, waitAfter));

  recording.steps.push({
    selectors: [[iframeSelector, 'body']],
    type:      'scroll',
    target:    'main',
    x: 0,
    y: scrollHeight,
    ...(frameIndex !== null ? { frame: [frameIndex] } : {})
  });
}
