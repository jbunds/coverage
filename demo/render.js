import puppeteer from 'puppeteer';
import { GhostCursor } from 'ghost-cursor';
import { URL, VIEWPORT, launchChrome, createMousePointer, registerListeners } from './helpers.js';

const OUTPUT = 'demo.webm';

(async () => {
  const browser = await launchChrome(URL);
  const [page]  = await browser.pages();

  // there is apparently no trivial way to constrain GhostCursor's random
  // movements to ensure the cursor always remains within the viewport
  // page.setViewport({ width: VIEWPORT.width, height: VIEWPORT.height });

  await createMousePointer(page);

  await registerListeners(page);

  const cursor = new GhostCursor(page, {
    start: {
      x: VIEWPORT.width  / 2,
      y: VIEWPORT.height / 2,
    },
    defaultOptions: {
      paddingPercentage:  100,
      moveDelay:          1000,
      overshootThreshold: 10000,
    }
  });

  await page.waitForNetworkIdle();

  const treeFrame = await (await page.waitForSelector('iframe#tree')).contentFrame();
  const recorder  = await page.screencast({ path: OUTPUT });

  // GhostCursor can't click on invisible elements like checkboxes,
  // so click on the visible labels applied to those checkboxes

  // click the "pkg" subdir
  const pkgLabel = await treeFrame.waitForSelector('label[for="tree-item-133"]');
  await cursor.move(pkgLabel);                // move the cursor to the "pkg" subdir label, but don't immediately click the "pkg" subdir label
  await new Promise(r => setTimeout(r, 400)); // brief pause before clicking the "pkg" subdir label
  await cursor.click(pkgLabel);

  // click the "kubelet" subdir
  const kubeletLabel = await treeFrame.waitForSelector('label[for="tree-item-527"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'nearest'
    });
  }, kubeletLabel);
  await new Promise(r => setTimeout(r, 600)); // wait for scroll to finish, and pause briefly before moving to and clicking the "kubelet" subdir label
  await cursor.move(kubeletLabel);            // move the cursor to "kubelet" subdir, but don't immediately click the label
  const { x, y } = cursor.getLocation();      // move the cursor to "kubelet" subdir
  await cursor.moveTo({ x, y });              // cursor.moveTo(cursor.getLocation()) won't work because the coordinates will be passed by reference
  // WHY does the cursor ALWAYS move BELOW the bottom of the "tree" frame here ??!
  await new Promise(r => setTimeout(r, 400)); // brief pause before clicking the "kubelet" subdir label
  await cursor.click(kubeletLabel);

  // click the "kubelet_network.go" link
  const goSrcLink = await treeFrame.waitForSelector('a[href="pkg/kubelet/kubelet_network.go.html"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'center',
    });
  }, goSrcLink);
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "kubelet_network.go" link
  await cursor.click(goSrcLink);

  // move to the "code" iframe and scroll down to its lower boundary
  const codeFrameHandle = await page.waitForSelector('iframe#code');
  const codeFrame       = await codeFrameHandle.contentFrame();
  await codeFrame.waitForSelector('body');        // or await page.waitForNetworkIdle();
  await codeFrame.evaluate(() => {                // add mousemove listener to "code" iframe
    window.addEventListener('mousemove', (e) => { // offset coordinates by the iframe's position on the main page
      const rect = window.frameElement.getBoundingClientRect();
      window.parent.mouseX = e.clientX + rect.left;
      window.parent.mouseY = e.clientY + rect.top;
    });
  });
  await cursor.move(codeFrameHandle);
  await cursor.scrollTo('bottom', { scrollSpeed: 8 });

  // click the "theme" button
  const indexBodyHandle = await page.waitForSelector('body#index');
  await cursor.move(indexBodyHandle);
  const themeButton     = await page.waitForSelector('#theme');
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "theme" button
  await cursor.click(themeButton);

  // click the "expand" button
  const expandButton = await page.waitForSelector('#expand');
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "expand" button
  await cursor.click(expandButton);

  // click the "theme" button again
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "theme" button
  await cursor.click(themeButton);

  // click the "expand" button again
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "expand" button again
  await cursor.click(expandButton);

  // click the "expand" button yet again
  await new Promise(r => setTimeout(r, 500)); // brief pause before clicking the "expand" button yet again
  await cursor.click(expandButton);

  // give the audience a moment to inspect the expanded tree
  await new Promise(r => setTimeout(r, 1000));

  await recorder.stop();
  await browser.close();
})();
