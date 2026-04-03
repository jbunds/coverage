// steps to record (omitting pauses between steps):
//
//  1. move the mouse pointer from middle of the viewport (i.e. within the code iframe)
//     to the "pkg" subdirectory (a checkbox element labeled "pkg") in the tree iframe:
//
//       <input type="checkbox" id="tree-item-133"/>
//       <div class="tree-node">
//         <label for="tree-item-133">pkg</label>
//         <span class="cov">49.9%</span>
//       </div>
//
//  2. click the "pkg" subdirectory (checkbox) to expand it
//
//  3. scroll down the "tree" iframe until the "kubelet" subdirectory is roughly
//     in the vertical center of the "tree" iframe:
//
//       <input type="checkbox" id="tree-item-527"/>
//       <div class="tree-node">
//         <label for="tree-item-527">kubelet</label>
//         <span class="cov">59.5%</span>
//       </div>
//
//  4. move the mouse pointer to the center of the "kubelet" subdirectory checkbox label
//
//  5. click "kubelet" subdirectory checkbox to expand it
//
//  6. move the mouse pointer down the "tree" iframe to the "li" element
//     which contains the link to the "kubelet_network.go" file:
//
//       <li>
//         <div class="tree-node">
//           <span class="src"><a href="pkg/kubelet/kubelet_network.go.html">kubelet_network.go</a></span>
//           <span class="cov">91.7%</span>
//         </div>
//       </li>
//
//  7. move the mouse pointer to the middle of the "kubelet_network.go" link
//
//  8. click the "kubelet_network.go" link to render the "pkg/kubelet/kubelet_network.go.html"
//     Go source code HTML file within the code iframe
//
//  9. move the mouse pointer from the "tree" iframe into the "code" iframe
//
// 10. scroll down to the bottom of the "code" iframe
//
// 11. move the mouse pointer to the "theme" button
//
// 12. click the "theme" button
//
// 13. move the mouse pointer to the "expand" button
//
// 14. click the "expand" button

import fs from 'fs';
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMouse,
  interactWith,
  scrollTo,
  scrollToBottom,
  typeWithRandomDelays } from './helpers.js';

(async () => {
  const browser = await launchChrome();
  const [page]  = await browser.pages();
  
  const initialX = VIEWPORT.width  / 2;
  const initialY = VIEWPORT.height / 2;
  await installMouse(page, initialX, initialY);

  console.log('navigating to:', URL);
  await page.goto(URL, { waitUntil: 'networkidle0' });

  const recording = {
    title: "demo",
    steps: [{ type: "navigate", url: URL }],
  };

  const iframeSelector = 'iframe#tree';
  const targetFile     = 'pkg/kubelet/kubelet_network.go.html';

  await page.waitForSelector(iframeSelector);
  
  // TODO(jeff): turn this block into a function in helpers.js
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

  // expand "pkg" subdir
  if (labelsToExpand.length > 0) {
    await interactWith(recording, page, labelsToExpand[0].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20, // click on the folder icon
      waitBefore: 800,
      waitAfter:  1000
    });
  }

  // scroll down to "kubelet" subdir
  await scrollTo(
    recording,
    page,
    iframeSelector,
    'label[for="tree-item-527"]',
    { frameIndex: 0 },
  );

  // TODO(jeff): move mouse pointer to "kubelet" subdir (checkbox)

  // click "kubelet" subdir to expand it
  if (labelsToExpand.length > 1) {
    await interactWith(recording, page, labelsToExpand[1].selector, {
      iframeSelector,
      frameIndex: 0,
      clickAtX:   20,
      waitBefore: 1000,
      waitAfter : 1200
    });
  }

  // TODO(jeff): scroll down "tree" iframe until the "li" element which contains the
  // "kubelet_network.go" link ("pkg/kubelet/kubelet_network.go.html") is roughly in
  // the vertical middle of the "tree" iframe

  // TODO(jeff): move the mouse pointer to the middle of the "kubelet_network.go" link

  // click the "kubelet_network.go" ("pkg/kubelet/kubelet_network.go.html") target file
  const fileSelector = `a[href="${targetFile}"]`;
  await interactWith(recording, page, fileSelector, {
    iframeSelector,
    frameIndex: 0,
    clickAtX:   20,
    waitBefore: 1200,
    waitAfter:  1500
  });

  // wait for the "code" iframe
  console.log('waiting for code iframe...');
  await page.waitForFunction(() => {
    const frame = document.querySelector('iframe#code');
    return frame?.contentDocument?.querySelector('pre') !== null;
  }, { timeout: 10000 });

  // scroll down to the bottom of the "code" iframe
  await scrollToBottom(recording, page, 'iframe#code', { 
    frameIndex: 1,
    waitBefore: 1500,
    waitAfter:  1500
  });

  // click the "theme" button
  await interactWith(recording, page, '#theme', {
    pixelsPerStep: 5.0,
    waitBefore:    1000,
    waitAfter:     1000 
  });

  // click the "expand" button
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

  console.log('closing browser in 1 second...');
  await new Promise(r => setTimeout(r, 1000));
  await browser.close();
})();
