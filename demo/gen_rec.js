import fs from 'fs';
import puppeteer from 'puppeteer';

const URL         = 'http://127.0.0.1:8000';
const MOUSE_SPEED = 8;
const VIEWPORT    = {
  deviceScaleFactor: 1,
  width:             1280,
  height:            720 };

// global state
let currentPos = {
  x: VIEWPORT.width  / 2,
  y: VIEWPORT.height / 2 };

// helpers
async function installMouseHelper(page, initialX = 0, initialY = 0) {
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
      document.body.appendChild(box);
      window.updatePuppeteerCursor = (newX, newY, clicking = false) => {
        box.style.left = (newX - 3) + 'px';
        box.style.top  = (newY - 3) + 'px';
        const path = box.querySelector('path');
        if (clicking) path.style.fill = 'gray';
        else path.style.fill = 'black';
      };
    });
  }, initialX, initialY);
}

async function moveMouse(page, targetX, targetY, pixelsPerStep = MOUSE_SPEED) {
  const startX   = currentPos.x;
  const startY   = currentPos.y;
  const distance = Math.hypot(targetX - startX, targetY - startY);
  const steps    = Math.max(1, Math.floor(distance / pixelsPerStep));
  console.log(`moving to (${targetX.toFixed(1)}, ${targetY.toFixed(1)}) | Steps: ${steps}`);
  for (let i = 1; i <= steps; i++) {
    const x = startX + (targetX - startX) * (i / steps);
    const y = startY + (targetY - startY) * (i / steps);
    await page.mouse.move(x, y);
    await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y); }, x, y);
    if (i % 2 === 0) await new Promise(r => setTimeout(r, 6));
  }
  await page.mouse.move(targetX, targetY);
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y); }, targetX, targetY);
  currentPos = { x: targetX, y: targetY };
  await new Promise(r => setTimeout(r, 200)); 
}

async function performVisualClick(page, x, y) {
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y, true); }, x, y);
  await page.mouse.click(x, y);
  await new Promise(r => setTimeout(r, 250));
  await page.evaluate((x, y) => { if (window.updatePuppeteerCursor) window.updatePuppeteerCursor(x, y, false); }, x, y);
}

async function launchChrome(url = URL) {
  return await puppeteer.launch({
    headless:        false,
    defaultViewport: null,
    executablePath:  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    args: [
      '--window-size=' + VIEWPORT.width + ',' + VIEWPORT.height,
      '--remote-debugging-port=9222',
      '--disable-web-security',
      '--disable-features=IsolateOrigins,site-per-process',
      '--disable-blink-features=AutomationControlled',
      url,
    ],
  });
}

// generic function to interact with an element (main page or iframe)
async function interactWith(recording, page, selector, options = {}) {
  const {
    iframeSelector = null,
    type           = 'click', // 'click', 'hover', or 'none' (just move)
    pixelsPerStep  = MOUSE_SPEED,
    frameIndex     = null,
    scrollIntoView = true,
    paddingX       = 0,       // offset from center X
    paddingY       = 0,       // offset from center Y
    waitBefore     = 1000,
    waitAfter      = 1000,
    clickAtX       = null,    // absolute X within element (overrides paddingX)
    clickAtY       = null     // absolute Y within element (overrides paddingY)
  } = options;

  console.log(`interacting with ${selector}${iframeSelector ? ` in ${iframeSelector}` : ''}...`);

  let targetFrame = page;
  if (iframeSelector) {
    const iframeHandle = await page.waitForSelector(iframeSelector);
    targetFrame = await iframeHandle.contentFrame();
  }

  const element = await targetFrame.waitForSelector(selector);

  if (scrollIntoView) {
    await element.evaluate(el => el.scrollIntoView({ block: 'center', inline: 'center' }));
    await new Promise(r => setTimeout(r, 500));
  }

  // get coordinates after potential scroll
  const rect = await element.boundingBox();
  if (!rect) throw new Error(`Bounding box null for ${selector}`);
  
  const targetX = clickAtX !== null ? rect.x + clickAtX : rect.x + rect.width  / 2 + paddingX;
  const targetY = clickAtY !== null ? rect.y + clickAtY : rect.y + rect.height / 2 + paddingY;

  // move to target
  console.log(`moving to ${selector}...`);
  await moveMouse(page, targetX, targetY, pixelsPerStep);
  
  const selectors = iframeSelector ? [[iframeSelector, selector]] : [[selector]];
  
  // recording hover
  if (type !== 'none') {
    recording.steps.push({
      selectors,
      type:   'hover',
      target: 'main',
      ...(frameIndex !== null ? { frame: [frameIndex] } : {})
    });
    await new Promise(r => setTimeout(r, waitBefore));
  }

  // recording click
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

  await new Promise(r => setTimeout(r, waitAfter));
}

// generic scroll function
async function scrollDownCompletely(recording, page, iframeSelector, options = {}) {
  const {
    frameIndex    = null,
    pixelsPerStep = MOUSE_SPEED,
    waitBefore    = 1000,
    waitAfter     = 1000
  } = options;

  console.log(`scrolling down ${iframeSelector} completely...`);
  
  const iframeHandle = await page.waitForSelector(iframeSelector);
  const frame        = await iframeHandle.contentFrame();
  
  // entry point for mouse
  const rect   = await iframeHandle.boundingBox();
  const entryX = rect.x + 200;
  const entryY = rect.y + 300;
  await moveMouse(page, entryX, entryY, pixelsPerStep);
  await new Promise(r => setTimeout(r, waitBefore));

  // slow smooth scroll
  const scrollHeight = await frame.evaluate(async () => {
    const totalHeight    = document.documentElement.scrollHeight;
    const viewportHeight = window.innerHeight;
    const distance       = 80; // smaller distance for slower scroll
    let currentPos = 0;
    while(currentPos < (totalHeight - viewportHeight)) {
      window.scrollBy(0, distance);
      currentPos += distance;
      await new Promise(r => setTimeout(r, 100)); // slightly longer delay
    }
    // ensure we reach the absolute bottom
    window.scrollTo(0, totalHeight);
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

// main
(async () => {
  const browser = await launchChrome();
  const pages   = await browser.pages();
  const page    = pages[0];
  
  await page.setViewport(VIEWPORT);
  await installMouseHelper(page, currentPos.x, currentPos.y);

  console.log('navigating to:', URL);
  await page.goto(URL, { waitUntil: 'networkidle0' });

  const recording = {
    title: "coverage demo",
    steps: [
      { type:        "setViewport", ...VIEWPORT,
        isMobile:    false,
        hasTouch:    false,
        isLandscape: false },
      { type:        "navigate", url: URL }
    ],
  };

  const iframeSelector = 'iframe#tree';
  const targetFile     = 'pkg/kubelet/kubelet_network.go.html';

  // find the sequence of labels to expand for the target file
  const labelsToExpand = await page.evaluate((href) => {
    const treeDoc = document.querySelector('iframe#tree').contentDocument;
    const link    = treeDoc.querySelector(`a[href="${href}"]`);
    if (!link) return [];
    const res = [];
    let curr = link.parentElement;
    while (curr) {
      if (curr.tagName === 'LI') {
        const input = curr.querySelector(':scope > input[type="checkbox"]');
        const label = curr.querySelector(':scope > .tree-node label');
        if (input && label) {
          res.unshift({
            selector: `label[for="${input.id}"]`,
            text:     label.textContent
          });
        }
      }
      curr = curr.parentElement;
    }
    return res;
  }, targetFile);

  console.log('labels to expand:', labelsToExpand.map(l => l.text).join(' > '));

  // 1. "pkg" subdirectory
  if (labelsToExpand.length > 0) {
    await interactWith(recording, page, labelsToExpand[0].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20, // click on the left side (folder icon)
      waitBefore: 1200,
      waitAfter:  1200
    });
  }

  // 2. "pkg/kubelet" subdirectory
  if (labelsToExpand.length > 1) {
    await interactWith(recording, page, labelsToExpand[1].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20, // click on the left side
      waitBefore: 1200,
      waitAfter : 1200
    });
  }

  // 3. "pkg/kubelet/kubelet_network.go.html"
  const fileSelector = `a[href="${targetFile}"]`;
  await interactWith(recording, page, fileSelector, {
    iframeSelector,
    frameIndex: 0,
    clickAtX:   20, // click on the left side
    waitBefore: 1200,
    waitAfter:  1200
  });

  // 4. scroll "code" iframe
  await page.waitForFunction(() => {
    const frame = document.querySelector('iframe#code');
    return frame && frame.contentDocument && frame.contentDocument.querySelector('pre');
  }, { timeout: 30000 });
  await scrollDownCompletely(recording, page, 'iframe#code', { frameIndex: 1 });

  // 5. theme button
  await interactWith(recording, page, '#theme', {
    pixelsPerStep: 6.0,
    waitBefore:    1200,
    waitAfter:     1200 });

  // 6. expand button
  await interactWith(recording, page, '#expand', {
    pixelsPerStep: 4.0,
    waitBefore:    1200,
    waitAfter:     1200 });

  console.log('saving recording...');

  fs.writeFileSync('recording.json', JSON.stringify(recording, null, 2));

  console.log('done. closing in 2s...');

  await new Promise(r => setTimeout(r, 2000));
  await browser.close();
})();
