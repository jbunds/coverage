import { setTimeout  } from 'node:timers/promises'
import { GhostCursor } from 'ghost-cursor'
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMousePointer,
  getAbsoluteCoords,
  registerListeners,
} from './helpers.js'

const OUTPUT = 'demo.webm'

const browser = await launchChrome(URL)
const [page]  = await browser.pages()

await installMousePointer(page)

const cursor = new GhostCursor(page, {
  defaultOptions: {
    move: {
      moveDelay:          1000,  // one-second delay after moving the mouse
      overshootThreshold: 10000, // distance between current location and destination which triggers overshoot to occur
      paddingPercentage:  100,   // move the mouse pointer to the center of the element
    },
  },
  start: {
    x: VIEWPORT.width  / 2,
    y: VIEWPORT.height / 2,
  }
})

await page.waitForNetworkIdle()

const treeDiv = await page.waitForSelector('#tree')

await registerListeners(page)

const recorder = await page.screencast({ path: OUTPUT })

const k8sIOLabel  = await treeDiv.waitForSelector('::-p-text(k8s.io)') // k8s.io subdir checkbox label
await cursor.click(k8sIOLabel, { hesitate: 200, moveSpeed: 25, waitForClick: 100 })

const k8sLabel = await treeDiv.waitForSelector('::-p-text(kubernetes)') // k8s.io/kubernetes subdir checkbox label
await cursor.click(k8sLabel, { hesitate: 50, moveSpeed: 40, waitForClick: 100 })

const pkgLabel = await treeDiv.waitForSelector('::-p-text(pkg)') // k8s.io/kubernetes/pkg subdir checkbox label
await cursor.click(pkgLabel, { hesitate: 250, moveSpeed: 40, waitForClick: 100 })

await setTimeout(200)
const kubeletLabel = await treeDiv.waitForSelector( // k8s.io/kubernetes/pkg/kubelet subdir checkbox label
  'li:has(> ul > li > div.tree-node > span.src > a[href="k8s.io/kubernetes/pkg/kubelet/kubelet_network.go.html"]) > div.tree-node > label'
)
await kubeletLabel.evaluate(el => el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' }))
await setTimeout(200)
await cursor.click(kubeletLabel, { hesitate: 200, moveSpeed: 40, waitForClick: 100 })
await setTimeout(800)

const goSrcLink = await treeDiv.waitForSelector('a[href="k8s.io/kubernetes/pkg/kubelet/kubelet_network.go.html"]') // kubelet_network.go link
await goSrcLink.evaluate(el => el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' }))
await cursor.click(goSrcLink, { hesitate: 400, moveSpeed: 40, waitForClick: 100 })

// move the cursor into the "code" iframe and scroll down to its lower boundary
const codeFrameHandle = await page.waitForSelector('iframe#code')
const codeFrame       = await codeFrameHandle.contentFrame()
await codeFrame.waitForSelector('body')                      // wait for quiescence; alternatively, could "await page.waitForNetworkIdle()", etc
await codeFrame.evaluate(() => {                             // add mousemove listener to "code" iframe
  window.addEventListener('mousemove', (e) => {              // register a listener for mousemove events within the "code" iframe
    const rect = window.frameElement.getBoundingClientRect() // offset coordinates by the iframe's position on the main page
    window.parent.mouseX = e.clientX + rect.left
    window.parent.mouseY = e.clientY + rect.top
  })
})
await cursor.move(codeFrameHandle)                  // move the cursor somewhere within the "code" frame
await cursor.scrollTo('bottom', { scrollSpeed: 8 }) // slowly scroll to the bottom of the "code" frame

const themeButton        = await page.waitForSelector('#theme')
const themeButtonCoords  = await getAbsoluteCoords(page.mainFrame(), themeButton)

const expandButton       = await page.waitForSelector('#expand')
const expandButtonCoords = await getAbsoluteCoords(page.mainFrame(), expandButton)

const funcsButton        = await page.waitForSelector('#funcs')
const funcsButtonCoords  = await getAbsoluteCoords(page.mainFrame(), funcsButton)

// the 100ms delay between mousedown and mouseup events ensures that at least one frame
// of the mouse pointer changing color when clicking an element will be captured

await cursor.moveTo(themeButtonCoords, { moveSpeed: 40 }) // "theme" button
await setTimeout(800)
await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 })

await setTimeout(800)
await cursor.moveTo(expandButtonCoords, { moveSpeed: 40 }) // "expand" button
await setTimeout(800)
await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 })

await setTimeout(800)
await cursor.moveTo(themeButtonCoords, { moveSpeed: 40 }) // "theme" button (again)
await setTimeout(800)
await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 })

await cursor.moveTo(funcsButtonCoords, { moveSpeed: 40 }) // "funcs" button
await setTimeout(800)
await page.mouse.click(funcsButtonCoords.x, funcsButtonCoords.y, { delay: 100 })
await setTimeout(600)
await cursor.moveTo({ x: funcsButtonCoords.x - 10, y: funcsButtonCoords.y + 10 }, { moveSpeed: 100 }) // small movement between clicks
await cursor.moveTo(funcsButtonCoords, { moveSpeed: 100 })
await setTimeout(1200)
await page.mouse.click(funcsButtonCoords.x, funcsButtonCoords.y, { delay: 100 })

await cursor.moveTo(expandButtonCoords, { moveSpeed: 40 }) // "expand" button
await setTimeout(800)
await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 })

await setTimeout(800) // give the audience a moment to inspect the expanded tree

await recorder.stop()
await browser.close()
