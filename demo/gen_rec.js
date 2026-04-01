import fs from 'fs';
import puppeteer from 'puppeteer';

import {
  URL,
  VIEWPORT,
  launchChrome,
  installMouseHelper,
  interactWith,
  scrollDownCompletely } from './helpers.js';

// main
(async () => {
  const browser = await launchChrome();
  const pages   = await browser.pages();
  const page    = pages[0];
  
  //await page.setViewport(VIEWPORT);

  // place mouse pointer in the middle of the viewport
  const initialX = VIEWPORT.width  / 2;
  const initialY = VIEWPORT.height / 2;
  await installMouseHelper(page, initialX, initialY);

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

  // 2. move to "pkg/kubelet" subdirectory
  // TODO(jeff): actually scroll down the tree iframe until "pkg/kubelet" is in the middle of the frame
  //             but already as-is, this added step arguably (maybe?) makes the recording look a bit more human and natural
  //const subdirSelector = 'label[for="tree-item-527"]';
  await interactWith(recording, page, labelsToExpand[0].selector, {
    iframeSelector,
    frameIndex: 0,
    type:       'none',
	});

  // 3. move to "pkg/kubelet" subdirectory and click it
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
  }, { timeout: 5000 });
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
