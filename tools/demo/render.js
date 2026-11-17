//import { unlinkSync   } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { setTimeout   } from 'node:timers/promises'
import { GhostCursor  } from 'ghost-cursor'
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMousePointer,
  getCenter,
  registerListeners,
} from './helpers.js'

const RAW_OUTPUT = 'demo.webm'
const OUTPUT     = 'demo.mp4'

const browser = await launchChrome(URL)
const [page]  = await browser.pages()

await installMousePointer(page)

const moveOptions = {
  moveDelay:           1000, // one-second delay after moving the mouse
  moveSpeed:             40, // constant speed across all mouse pointer (cursor) movements
  overshootThreshold: 10000, // distance between current location and destination which triggers overshoot to occur
  paddingPercentage:    100, // move the mouse pointer to the center of the element
  randomizeMoveDelay: false, // use moveDelay instead of a random 0..moveDelay duration
  scrollDelay:          300, // pause for 300ms after scrolling before performing the next interaction
  scrollSpeed:           30, // smooth scroll speed (0 == slow, 100 == instant)
}

const cursor = new GhostCursor(page, {
  defaultOptions: {
    click:  moveOptions,
    move:   moveOptions,
    moveTo: moveOptions,
    scroll: {
      scrollDelay: 300,
      scrollSpeed:  30,
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

const recorder = await page.screencast({ path: RAW_OUTPUT })

await setTimeout(600)

const k8sIOLabel  = await treeDiv.waitForSelector('::-p-text(k8s.io)') // k8s.io subdir checkbox label
await cursor.click(k8sIOLabel, { hesitate: 400, waitForClick: 100 })

const k8sLabel = await treeDiv.waitForSelector('::-p-text(kubernetes)') // k8s.io/kubernetes subdir checkbox label
await cursor.click(k8sLabel, { hesitate: 200, waitForClick: 100 })

const pkgLabel = await treeDiv.waitForSelector('::-p-text(pkg)') // k8s.io/kubernetes/pkg subdir checkbox label
await cursor.click(pkgLabel, { hesitate: 200, waitForClick: 100 })

await setTimeout(400)

const goSrcLink    = await treeDiv.waitForSelector('a[href="k8s.io/kubernetes/pkg/kubelet/kubelet_network.go.html"]') // kubelet_network.go link
const kubeletLabel = await goSrcLink.evaluateHandle(el => el.closest('ul').closest('li').querySelector(':scope > div.tree-node > label'))
await kubeletLabel.evaluate(el => el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' }))
await cursor.click(kubeletLabel, { hesitate: 500, waitForClick: 100 })

await setTimeout(400)

await goSrcLink.evaluate(el => el.scrollIntoView({ behavior: 'smooth', block: 'center', inline: 'center' }))
await cursor.click(goSrcLink, { hesitate: 400, waitForClick: 100 })

// move the cursor into the #code iframe and scroll down to its lower boundary
const codeFrameHandle = await page.waitForSelector('iframe#code')
const codeFrame       = await codeFrameHandle.contentFrame()
await codeFrame.waitForSelector('body')                      // wait for quiescence; alternatively, could "await page.waitForNetworkIdle()", etc
await codeFrame.evaluate(() => {                             // add mousemove listener to #code iframe
  window.addEventListener('mousemove', (e) => {              // register a listener for mousemove events within the #code iframe
    const rect = window.frameElement.getBoundingClientRect() // offset coordinates by the iframe's position on the main page
    window.parent.mouseX = e.clientX + rect.left
    window.parent.mouseY = e.clientY + rect.top
  })
})
await cursor.move(codeFrameHandle)               // move the cursor to the middle of the #code frame
await codeFrame.evaluate(() => window.scrollTo({ // slowly scroll to the bottom of the #code frame
  behavior: 'smooth',
  top:      document.documentElement.scrollHeight
}))

const themeButton  = await page.waitForSelector('#theme')
const expandButton = await page.waitForSelector('#expand')
const funcsButton  = await page.waitForSelector('#funcs')
const treeButton   = await page.waitForSelector('#tree-btn')
const linesButton  = await page.waitForSelector('#lines')

const themeButtonCoords  = await getCenter(themeButton)
const expandButtonCoords = await getCenter(expandButton)
const funcsButtonCoords  = await getCenter(funcsButton)
const treeButtonCoords   = await getCenter(treeButton)
const linesButtonCoords  = await getCenter(linesButton)

// the 100ms delay between mousedown and mouseup events ensures that at least one frame
// of the mouse pointer changing color when clicking an element will be captured

await setTimeout(400)
await cursor.moveTo(themeButtonCoords) // "theme" button
await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(expandButtonCoords) // "expand" button
await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(themeButtonCoords) // "theme" button (again)
await page.mouse.click(themeButtonCoords.x, themeButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(funcsButtonCoords) // "funcs" button
await page.mouse.click(funcsButtonCoords.x, funcsButtonCoords.y, { delay: 100 })
await cursor.moveTo({ // small mouse pointer movement between clicks
  x: funcsButtonCoords.x - 10,
  y: funcsButtonCoords.y -  4 }, { moveDelay: 0, moveSpeed: 100 })
await cursor.moveTo(funcsButtonCoords, { moveSpeed: 100 })
await page.mouse.click(funcsButtonCoords.x, funcsButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(expandButtonCoords) // "collapse" ("expand") button
await page.mouse.click(expandButtonCoords.x, expandButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(treeButtonCoords) // "tree" button
await page.mouse.click(treeButtonCoords.x, treeButtonCoords.y, { delay: 100 })

await setTimeout(400)
await cursor.moveTo(linesButtonCoords) // "lines" button
await page.mouse.click(linesButtonCoords.x, linesButtonCoords.y, { delay: 100 })
await cursor.moveTo({ // small mouse pointer movement between clicks
  x: linesButtonCoords.x + 8,
  y: linesButtonCoords.y + 4 }, { moveDelay: 0, moveSpeed: 100})
await cursor.moveTo(linesButtonCoords, { moveSpeed: 100 })
await page.mouse.click(linesButtonCoords.x, linesButtonCoords.y, { delay: 100 })

await setTimeout(1000) // give the audience a moment to inspect the final frame
await recorder.stop()
await setTimeout(5000) // give Puppeteer's internal ffmpeg process time to flush and close demo.webm
await browser.close()

execFileSync('ffmpeg', [
  '-y',                                               // overwrite OUTPUT
  '-fflags',          '+genpts',                      // generate clean timestamps from Puppeteer's VFR WebM; must be specified before -i
  '-i',               RAW_OUTPUT,                     // input file
  '-fps_mode',        'cfr',                          // force ffmpeg to duplicate static frames across idle pauses, preserving the Puppeteer timeline
  '-r',               '60',                           // 60 FPS
  '-c:v',             'libx264',                      // use the x264 encoder (libx264) to produce a H.264-encoded video
  '-vf',              'scale=out_color_matrix=bt709', // convert RGB pixel values to YUV using the BT.709 HD color formula
  '-pix_fmt',         'yuv420p',                      // use 8-bit YUV with 4:2:0 chroma subsampling
  '-colorspace',      'bt709',                        // write BT.709 color tags into the MP4 container to provide the color profile to browsers when rendering
  '-color_primaries', 'bt709',                        // ..
  '-color_trc',       'bt709',                        // ..
  '-crf',             '22',                           // set the libx264 constant rate factor (quality; 0 == lossless; 51 == worst)
  '-preset',          'slow',                         // speed-to-compression ratio for libx264 (see `x264 --help`)
  '-movflags',        '+faststart',                   // moves the MP4 index ("moov" atom) from EOF to the beginning to enable playback before the whole video has been downloaded
  OUTPUT,
], { stdio: 'inherit' })

//unlinkSync(RAW_OUTPUT)
