//import { unlinkSync   } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { setTimeout   } from 'node:timers/promises'
import { GhostCursor  } from 'ghost-cursor'
import {
  URL,
  VIEWPORT,
  launchChrome,
  installMousePointer,
  findLabel,
  clickOffset,
  registerListeners,
} from './helpers.ts'

const RAW_OUTPUT = 'demo.webm'
const OUTPUT     = 'demo.mp4'

const browser = await launchChrome(URL)
const [page]  = await browser.pages()

await installMousePointer(page)

const scrollOptions = {
  behavior: 'smooth' as const,
  block:    'center' as const,
  inline:   'center' as const,
}

const moveOptions = {
  moveDelay:            500, // 500ms delay after moving the mouse pointer;                                          default: 0 for move, 2000 for click
  moveSpeed:             40, // constant speed across all mouse pointer (cursor) movements;                          default: random
  overshootThreshold: 10000, // distance between current location and destination which triggers overshoot to occur; default: 500
  paddingPercentage:    100, // move the mouse pointer to the center of the element;                                 default: 0
  randomizeMoveDelay: false, // use moveDelay instead of a random 0..moveDelay duration;                             default: true
  scrollDelay:          300, // pause for 300ms after scrolling before performing the next interaction;              default: 200
  scrollSpeed:           30, // smooth scroll speed (0 == slow, 100 == instant);                                     default: 100
  waitForClick:         100, // delay between mousedown and mouseup to ensure that at least one frame of
                             // the mouse pointer changing color when clicking an element will be captured;          default: 0
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

const treeDiv  = (await page.waitForSelector('#tree'))!
const recorder =  await page.screencast({ path: RAW_OUTPUT })

await registerListeners(page)

await setTimeout(600)

const k8sIOLabel = (await findLabel(treeDiv, 'k8s.io'    ))! // k8s.io                subdir checkbox label
const k8sLabel   = (await findLabel(treeDiv, 'kubernetes'))! // k8s.io/kubernetes     subdir checkbox label
const pkgLabel   = (await findLabel(treeDiv, 'pkg'       ))! // k8s.io/kubernetes/pkg subdir checkbox label

await clickOffset(cursor, k8sIOLabel, { hesitate: 400 })
await clickOffset(cursor, k8sLabel,   { hesitate: 200 })
await clickOffset(cursor, pkgLabel,   { hesitate: 200 })

await setTimeout(400)

const goSrcLink    = (await treeDiv.waitForSelector('a[href="k8s.io/kubernetes/pkg/kubelet/kubelet_network.go.html"]'))! // kubelet_network.go link
const kubeletLabel =  await goSrcLink.evaluateHandle(el => el.closest('ul')!.closest('li')!.querySelector(':scope > div.tree-node > label')!)

await kubeletLabel.evaluate((el, opts) => el.scrollIntoView(opts), scrollOptions)
await clickOffset(cursor, kubeletLabel, { hesitate: 500 })

await setTimeout(400)

await goSrcLink.evaluate((el, opts) => el.scrollIntoView(opts), scrollOptions)
await clickOffset(cursor, goSrcLink, { hesitate: 400 })

// move the mouse pointer into the #code iframe and scroll down to its lower boundary
const codeFrameHandle = (await page.waitForSelector('iframe#code'))!
const codeFrame       = (await codeFrameHandle.contentFrame())!
await codeFrame.waitForSelector('body')                                       // wait for quiescence (alternatively, could "await page.waitForNetworkIdle()", etc)
await codeFrame.evaluate(() => {                                              // add mousemove listener to #code iframe
  window.addEventListener('mousemove', (e) => {                               // register a listener for mousemove events within the #code iframe
    const rect = (window.frameElement as HTMLElement).getBoundingClientRect() // offset coordinates by the iframe's position on the main page
    window.parent.mouseX = e.clientX + rect.left
    window.parent.mouseY = e.clientY + rect.top
  })
})
await cursor.move(codeFrameHandle)               // move the mouse pointer to the middle of the #code frame
await codeFrame.evaluate(() => window.scrollTo({ // scroll to the bottom of the #code frame
  behavior: 'smooth',
  top:      document.documentElement.scrollHeight
}))

const themeButton  = (await page.waitForSelector('#theme'   ))!
const expandButton = (await page.waitForSelector('#expand'  ))!
const funcsButton  = (await page.waitForSelector('#funcs'   ))!
const treeButton   = (await page.waitForSelector('#tree-btn'))!
const linesButton  = (await page.waitForSelector('#lines'   ))!

await clickOffset(cursor, themeButton,  undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, expandButton, undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, themeButton,  undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, funcsButton,  undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, funcsButton,  { moveDelay: 100 }, { jitter: true, pauseBefore: 200 })
await clickOffset(cursor, expandButton, undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, treeButton,   undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, linesButton,  undefined,          {               pauseBefore: 250 })
await clickOffset(cursor, linesButton,  { moveDelay: 100 }, { jitter: true, pauseBefore: 200 })

await setTimeout(1000) // give the audience a moment to inspect the final frame
await recorder.stop()

await setTimeout(3000) // give Puppeteer's internal ffmpeg process 3s to flush and close demo.webm
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
