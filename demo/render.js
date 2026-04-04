import puppeteer from 'puppeteer';
import { GhostCursor } from 'ghost-cursor';
import {
  URL,
  VIEWPORT,
  launchChrome,
  createMousePointer,
  getAbsoluteCoords,
  registerListeners } from './helpers.js';

const OUTPUT = 'demo.webm';

(async () => {
  const browser = await launchChrome(URL);
  const [page]  = await browser.pages();

  // there is apparently no trivial way to constrain GhostCursor's random
  // movements to ensure the cursor always remains within the viewport
  // page.setViewport({ width: VIEWPORT.width, height: VIEWPORT.height });

  await createMousePointer(page);

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

  await registerListeners(page);

  const recorder = await page.screencast({ path: OUTPUT });

  // GhostCursor can't click on invisible elements like checkboxes,
  // so click on the visible labels applied to those checkboxes

  // click the "pkg" subdir
  const pkgLabel  = await treeFrame.waitForSelector('label[for="tree-item-133"]');
  const pkgCoords = await getAbsoluteCoords(treeFrame, pkgLabel);
  await cursor.moveTo(pkgCoords);             // move the cursor to the "pkg" subdir label, but don't immediately click the "pkg" subdir label
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "pkg" subdir label
  await page.mouse.click(pkgCoords.x, pkgCoords.y, { delay: 100 });

  // click the "kubelet" subdir
  const kubeletLabel  = await treeFrame.waitForSelector('label[for="tree-item-527"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'nearest'
    });
  }, kubeletLabel);
  await new Promise(r => setTimeout(r, 1000)); // wait for scroll to finish, and pause briefly before moving to and clicking the "kubelet" subdir label
  const kubeletCoords = await getAbsoluteCoords(treeFrame, kubeletLabel);
  await cursor.moveTo(kubeletCoords);          // move the cursor to "kubelet" subdir, but don't immediately click the label
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "kubelet" subdir label
  await page.mouse.click(kubeletCoords.x, kubeletCoords.y, { delay: 100 });

  // click the "kubelet_network.go" link
  const goSrcLink = await treeFrame.waitForSelector('a[href="pkg/kubelet/kubelet_network.go.html"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth',
      block:    'center',
      inline:   'center',
    });
  }, goSrcLink);
  await new Promise(r => setTimeout(r, 1000));
  const goSrcLinkCoords = await getAbsoluteCoords(treeFrame, goSrcLink);
  await cursor.moveTo(goSrcLinkCoords);
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "kubelet_network.go" link
  await page.mouse.click(goSrcLinkCoords.x, goSrcLinkCoords.y, { delay: 100 });

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
  const themeButtonCoords = await getAbsoluteCoords(page.mainFrame(), themeButton);
  await cursor.moveTo(themeButtonCoords);
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "theme" button
  await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 });

  // click the "expand" button
  const expandButton = await page.waitForSelector('#expand');
  const expandButtonCoords = await getAbsoluteCoords(page.mainFrame(), expandButton);
  await cursor.moveTo(expandButtonCoords);
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "expand" button
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 });

  // click the "theme" button again
  await cursor.moveTo(themeButtonCoords);
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "theme" button
  await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 });

  // click the "expand" button again
  await cursor.moveTo(expandButtonCoords);
  await new Promise(r => setTimeout(r, 1000)); // brief pause before clicking the "expand" button again
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 });

  // click the "expand" button yet again
  await new Promise(r => setTimeout(r, 2000)); // brief pause before clicking the "expand" button yet again
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 });

  // give the audience a moment to inspect the expanded tree
  await new Promise(r => setTimeout(r, 1000));

  await recorder.stop();
  await browser.close();
})();
