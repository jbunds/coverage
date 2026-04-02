import fs from 'fs';
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMouseHelper,
  interactWith,
  scrollTo,
  scrollToBottom,
  typeWithRandomDelays } from './helpers.js';

(async () => {
  const browser = await launchChrome();
  const [page]  = await browser.pages();
  
  const initialX = VIEWPORT.width  / 2;
  const initialY = VIEWPORT.height / 2;
  await installMouseHelper(page, initialX, initialY);

  console.log('navigating to:', URL);
  await page.goto(URL, { waitUntil: 'networkidle0' });

  const recording = {
    title: "demo",
    steps: [{ type: "navigate", url: URL }],
  };

  const iframeSelector = 'iframe#tree';
  const targetFile     = 'pkg/kubelet/kubelet_network.go.html';

  await page.waitForSelector(iframeSelector); // iframe#tree
  
  // find the sequence of labels to expand for the target file
  const labelsToExpand = await page.evaluate((href) => {
    const treeDoc = document.querySelector('iframe#tree')?.contentDocument;
    if (!treeDoc) return [];
    const link = treeDoc.querySelector(`a[href="${href}"]`);
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
            text:     label.textContent.trim()
          });
        }
      }
      curr = curr.parentElement;
    }
    return res;
  }, targetFile);

  console.log('labels to expand:', labelsToExpand.map(l => l.text).join(' > '));

  // expand "pkg"
  if (labelsToExpand.length > 0) {
    await interactWith(recording, page, labelsToExpand[0].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20, // click on the folder icon
      waitBefore: 800,
      waitAfter:  1000
    });
  }

  // scroll to see more items
  await scrollTo(
    recording,
    page,
    iframeSelector,
    'label[for="tree-item-527"]', {
      frameIndex: 0,
    });

  // expand "kubelet"
  if (labelsToExpand.length > 1) {
    await interactWith(recording, page, labelsToExpand[1].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20,
      waitBefore: 1000,
      waitAfter : 1200
    });
  }

  // click the target file
  const fileSelector = `a[href="${targetFile}"]`;
  await interactWith(recording, page, fileSelector, {
    iframeSelector,
    frameIndex: 0,
    clickAtX:   20,
    waitBefore: 1200,
    waitAfter:  1500
  });

  // wait for and scroll the "code" iframe
  console.log('Waiting for code iframe...');
  await page.waitForFunction(() => {
    const frame = document.querySelector('iframe#code');
    return frame?.contentDocument?.querySelector('pre') !== null;
  }, { timeout: 10000 });

  await scrollToBottom(recording, page, 'iframe#code', { 
    frameIndex: 1,
    waitBefore: 1500,
    waitAfter:  1500
  });

  // click theme button
  await interactWith(recording, page, '#theme', {
    pixelsPerStep: 5.0,
    waitBefore:    1000,
    waitAfter:     1000 
  });

  // click expand button
  await interactWith(recording, page, '#expand', {
    pixelsPerStep: 4.0,
    waitBefore:    1200,
    waitAfter:     1500 
  });

  // final pause before saving
  console.log('final pause...');
  await new Promise(r => setTimeout(r, 1000));

  console.log('saving recording.json...');
  fs.writeFileSync('recording.json', JSON.stringify(recording, null, 2));

  console.log('closing browser in 2s...');
  await new Promise(r => setTimeout(r, 2000));
  await browser.close();
})();
