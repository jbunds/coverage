import puppeteer from 'puppeteer';
import { GhostCursor } from 'ghost-cursor';
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMousePointer,
  getAbsoluteCoords,
  registerListeners } from './helpers.js';

const OUTPUT = 'demo.webm';

(async () => {
  const browser = await launchChrome(URL);
  const [page]  = await browser.pages();

  // there is apparently no trivial way to constrain GhostCursor's random
  // movements to ensure the cursor always remains within the viewport
  // page.setViewport({ width: VIEWPORT.width, height: VIEWPORT.height });

  await installMousePointer(page);

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

  // the following implementation minimizes GhostCursor's randomized cursor overshoots
  // and deviations relative to the centers of the elements interacted with
  //
  // to that end, all recorded interactions follow the same basic formula:
  //
  //   1. use Puppeteer's Frame.waitForSelector method to get an ElementHandle 
  //
  //   2. use JavaScript's native scrollIntoView method to scroll the element
  //      into the center of its parent frame
  //
  //   3. use the getAbsoluteCoords helper function to determine the post-scroll
  //      absolute coordinates of the the element
  //
  //   4. use GhostCursor's moveTo method to move the cursor to the element
  //
  //   5. pause for one second to allow viewers of the demo to follow the interaction
  //
  //   6. use Puppeteer's Mouse.click method to click the element, with a 100ms
  //      delay between mousedown and mouseup events
  //
  // the 100ms delay between "mousedown" and "mouseup" events is necessary to capture
  // at least one frame of the mouse pointer changing color when clicking an element

  // click the "pkg" subdir checkbox label
  const pkgLabel  = await treeFrame.waitForSelector('label[for="tree-item-133"]');
  const pkgCoords = await getAbsoluteCoords(treeFrame, pkgLabel);
  await cursor.moveTo(pkgCoords);                                   // move the cursor to the "pkg" subdir label, but don't immediately click it
  await new Promise(r => setTimeout(r, 1000));                      // 1s pause before clicking
  await page.mouse.click(pkgCoords.x, pkgCoords.y, { delay: 100 }); // click on the middle of the label, with a 100ms delay between mousedown and mouseup events

  // click the "kubelet" subdir checkbox label
  const kubeletLabel  = await treeFrame.waitForSelector('label[for="tree-item-527"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth', // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#behavior
      block:    'center', // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#block
      inline:   'center'  // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#inline
    });
  }, kubeletLabel);
  await new Promise(r => setTimeout(r, 1000));                              // 1s pause to wait for scroll to finish, before moving the cursor to the label
  const kubeletCoords = await getAbsoluteCoords(treeFrame, kubeletLabel);   // get the post-scroll coordinates of the label
  await cursor.moveTo(kubeletCoords);                                       // move the cursor to the label, but don't immediately click it
  await new Promise(r => setTimeout(r, 1000));                              // 1s pause before clicking the label
  await page.mouse.click(kubeletCoords.x, kubeletCoords.y, { delay: 100 }); // click the label with a 100ms delay between mousedown and mouseup events

  // click the "kubelet_network.go" link
  const goSrcLink = await treeFrame.waitForSelector('a[href="pkg/kubelet/kubelet_network.go.html"]');
  await treeFrame.evaluate((el) => {
    el.scrollIntoView({
      behavior: 'smooth', // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#behavior
      block:    'center', // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#block
      inline:   'center', // https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView#inline
    });
  }, goSrcLink);
  await new Promise(r => setTimeout(r, 1000));                                  // 1s pause to wait for scrol to finish, before moving the cursor to the link
  const goSrcLinkCoords = await getAbsoluteCoords(treeFrame, goSrcLink);        // get the post-scroll coordinates of the link
  await cursor.moveTo(goSrcLinkCoords);                                         // move the cursor to the link, but don't immediately click it
  await new Promise(r => setTimeout(r, 1000));                                  // 1s pause before clicking the link
  await page.mouse.click(goSrcLinkCoords.x, goSrcLinkCoords.y, { delay: 100 }); // click the link with a 100ms delay between mousedown and mouseup events

  // move the cursor into the "code" iframe and scroll down to its lower boundary
  const codeFrameHandle = await page.waitForSelector('iframe#code');
  const codeFrame       = await codeFrameHandle.contentFrame();
  await codeFrame.waitForSelector('body');                      // wait for quiescence; alternatively, could "await page.waitForNetworkIdle()", etc
  await codeFrame.evaluate(() => {                              // add mousemove listener to "code" iframe
    window.addEventListener('mousemove', (e) => {               // register a listener for mousemove events within the "code" iframe
      const rect = window.frameElement.getBoundingClientRect(); // offset coordinates by the iframe's position on the main page
      window.parent.mouseX = e.clientX + rect.left;
      window.parent.mouseY = e.clientY + rect.top;
    });
  });
  await cursor.move(codeFrameHandle);                           // move the cursor somewhere within the "code" frame
  await cursor.scrollTo('bottom', { scrollSpeed: 8 });          // slowly scroll to the bottom of the "code" frame

  // click the "theme" button
  const indexBodyHandle   = await page.waitForSelector('body#index');
  await cursor.move(indexBodyHandle);                                                 // move the cursor somewhere within the main frame
  const themeButton       = await page.waitForSelector('#theme');
  const themeButtonCoords = await getAbsoluteCoords(page.mainFrame(), themeButton);   // get the coordinates of the "theme" button
  await cursor.moveTo(themeButtonCoords);                                             // move the cursor to the "theme" button
  await new Promise(r => setTimeout(r, 1000));                                        // 1s pause before clicking the button
  await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 });   // click the button with a 100ms delay between mousedown and mouseup events

  // click the "expand" button
  const expandButton       = await page.waitForSelector('#expand');
  const expandButtonCoords = await getAbsoluteCoords(page.mainFrame(), expandButton);
  await cursor.moveTo(expandButtonCoords);                                            // move the cursor to the "expand" button
  await new Promise(r => setTimeout(r, 1000));                                        // 1s pause before clicking the button
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 }); // click the button with a 100ms delay between mousedown and mouseup events

  // click the "theme" button again
  await cursor.moveTo(themeButtonCoords);                                             // move the cursor to the "theme" button
  await new Promise(r => setTimeout(r, 1000));                                        // 1s pause before clicking the button
  await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 });   // click the button with a 100ms delay between mousedown and mouseup events

  // click the "expand" button again
  await cursor.moveTo(expandButtonCoords);                                            // move the cursor to the "expand" button again
  await new Promise(r => setTimeout(r, 1000));                                        // 1s pause before clicking the button again
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 }); // click the button with a 100ms delay between mousedown and mouseup events

  // click the "expand" button yet again
  await new Promise(r => setTimeout(r, 2000));                                        // 1s pause before clicking the "expand" button yet again
  await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 }); // click the button with a 100ms delay between mousedown and mouseup events

  // give the audience a moment to inspect the expanded tree
  await new Promise(r => setTimeout(r, 1000));

  await recorder.stop();
  await browser.close();
})();
