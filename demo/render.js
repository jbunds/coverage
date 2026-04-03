import puppeteer from 'puppeteer';
import { URL, VIEWPORT, launchChrome } from './helpers.js';
import { GhostCursor, installMouseHelper } from 'ghost-cursor';

const OUTPUT = 'demo.webm';

(async () => {
  const browser = await launchChrome(URL);
  const [page]  = await browser.pages();

  // create SVG mouse pointer
  await page.evaluate((initialX = 0, initialY = 0) => {
    const cursor = document.createElement('div');
    cursor.id        = 'manual-pointer';
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

  // register event listeners to track GhostCursor's cursor movements
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

  const cursor = new GhostCursor(page, {
    start: { x: VIEWPORT.width  / 2, y: VIEWPORT.height / 2 },
    defaultOptions: {
      paddingPercentage:  100,
      moveDelay:          1000,
      overshootThreshold: 100000,
      //moveSpeed: 10,
    }
  });

  // hack to workaround GhostCursor's cursor origin defaulting to 0, 0,
  // despite explicitly telling it to originate in the middle of the viewport directly above
  //
  // TODO(jeff): either one of these calls might be redundant; test to determine what's acutally needed
  //page.mouse.move(  VIEWPORT.width / 2,    VIEWPORT.height / 2);
  cursor.moveTo({x: VIEWPORT.width / 2, y: VIEWPORT.height / 2});

  await page.waitForNetworkIdle();

  const treeFrame = await (await page.waitForSelector('iframe#tree')).contentFrame();
  const recorder  = await page.screencast({ path: OUTPUT });

  // GhostCursor can't click on invisible elements like checkboxes,
  // so click on the visible labels applied to those checkboxes

  // "pkg" subdir
  const pkgLabel = await treeFrame.waitForSelector('label[for="tree-item-133"]');
  await cursor.move(pkgLabel)
  await cursor.click(pkgLabel);

  // "kubelet" subdir
  const kubeletLabel = await treeFrame.waitForSelector('label[for="tree-item-527"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'nearest'
    });
  }, kubeletLabel);
  await new Promise(r => setTimeout(r, 1000)); // wait for scroll to finish
  const { x, y } = cursor.getLocation();
  await cursor.moveTo({ x, y }); // cursor.moveTo(cursor.getLocation()) won't work because the coordinates will be passed by reference
  await cursor.click(kubeletLabel);

  // "kubelet_network.go" link
  const goSrcLink = await treeFrame.waitForSelector('a[href="pkg/kubelet/kubelet_network.go.html"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'center',
    });
  }, goSrcLink);

  await cursor.move(goSrcLink);
  await cursor.click(goSrcLink);

  const codeFrameHandle = await page.waitForSelector('iframe#code');
  const codeFrame       = await codeFrameHandle.contentFrame();
  await codeFrame.waitForSelector('body'); // or await page.waitForNetworkIdle();
  await codeFrame.evaluate(() => {
    window.addEventListener('mousemove', (e) => { // offset the coordinates by the iframe's position on the main page
      const rect = window.frameElement.getBoundingClientRect();
      window.parent.mouseX = e.clientX + rect.left;
      window.parent.mouseY = e.clientY + rect.top;
    });
  });
  await cursor.move(codeFrameHandle);
  await cursor.scrollTo('bottom', { scrollSpeed: 8 });

  // "theme" button
  const indexBody = await page.waitForSelector('body#index');
  await cursor.move(indexBody);
  // maybe pause here for a brief moment
  const themeButton = await page.waitForSelector('#theme');
  await cursor.move(themeButton);
  // maybe pause here for a brief moment
  await cursor.click(themeButton);

  // "expand" button
  const expandButton = await page.waitForSelector('#expand');
  await cursor.move(expandButton);
  // maybe pause here for a brief moment
  await cursor.click(expandButton);
  // maybe pause here for a brief moment

  await new Promise(r => setTimeout(r, 1000)); // give the audience a moment to look at the expanded tree

  await recorder.stop();
  await browser.close();
})();
