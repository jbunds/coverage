import puppeteer from 'puppeteer';
import { GhostCursor, installMouseHelper } from 'ghost-cursor';
import { URL, VIEWPORT, launchChrome, createMousePointer, registerListeners } from './helpers.js';

const OUTPUT = 'demo.webm';

(async () => {
  const browser = await launchChrome(URL);
  const [page]  = await browser.pages();

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

  // "pkg" subdir
  const pkgLabel = await treeFrame.waitForSelector('label[for="tree-item-133"]');
  await cursor.move(pkgLabel);
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
