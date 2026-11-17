import { setTimeout } from 'node:timers/promises'
import   puppeteer    from 'puppeteer'

export const URL      = 'http://127.0.0.1:8000'
export const VIEWPORT = {
  deviceScaleFactor: 1,
  height:            720,
  width:             1280,
}

// launch Chrome with flags enabling all required permissions (possibly overscoped; see TODO comment below)
export async function launchChrome (url = URL) {
  const options = {
    // TODO(jeff): determine which specific flags are actually required,
    //             and remove those that are not strictly required
    args: [
      `--window-size=${VIEWPORT.width},${VIEWPORT.height}`,
      '--disable-web-security',
      '--disable-blink-features=AutomationControlled',
      url
    ],
    channel:        'chrome',
    defaultViewport: null,
    headless:        false
  }
  return puppeteer.launch(options)
}

// install a custom mouse pointer (analogous to GhostCursor's installMouseHelper)
export async function installMousePointer (page) {
  await page.evaluate((initialX = 0, initialY = 0) => {
    const cursor = document.createElement('div')
    cursor.id        = 'custom-pointer'
    cursor.innerHTML = `<svg width="32" height="32" viewBox="0 0 32 32">
<path d="M10 7v11.188l2.53-2.442 2.901 5.254 1.765-.941-2.775-5.202h3.604L10 7z" fill="black" stroke="white" stroke-width="1.5"/>
</svg>`
    // TODO(jeff): stop the cursor from shrinking in size once it traverses into the main frame
    //
    //             it might actually be an illusion since it seems to coincide with the
    //             application of the light theme, and i cannot think of any reason such
    //             odd behavior would occur
    Object.assign(cursor.style, {
      height: '32px', left:      '0',              pointerEvents: 'none', position: 'fixed',
      top:    '0',    transform: 'translate(0,0)', width:         '32px', zIndex:   '1000',
    })
    document.documentElement.appendChild(cursor)
    const path     = cursor.querySelector('path')
    const setGray  = () => { path.style.fill = 'gray'  }
    const setBlack = () => { path.style.fill = 'black' }
    window.mouseX = initialX
    window.mouseY = initialY
    const updatePos = (e) => { window.mouseX = e.clientX; window.mouseY = e.clientY }
    window.addEventListener('mousemove', updatePos)
    window.addEventListener('mousedown', setGray)
    window.addEventListener('mouseup',   setBlack)
    window.addEventListener('message', (event) => {
      if (event.data.type === 'POINTER_CLICK') {
        if (event.data.action === 'down') setGray()
        if (event.data.action === 'up'  ) setBlack()
      }
    })
    function track () { // animation loop: updates at 60fps to follow the GhostCursor cursor's path
      cursor.style.transform = `translate3d(${window.mouseX}px, ${window.mouseY}px, 0)`
      requestAnimationFrame(track)
    }
    track()
  }, VIEWPORT.width  / 2, VIEWPORT.height / 2)
}

// register event listeners to track GhostCursor's cursor movements and clicks
export async function registerListeners (page) {
  const frames = page.frames()
  for (const f of frames) {
    if (f === page.mainFrame()) continue
    await f.waitForSelector('body').catch(() => {})
    await f.evaluate(() => {
      const forwardEvent = (type) => {
        window.parent.postMessage({ action: type, type: 'POINTER_CLICK' }, '*')
      }
      const updateParentPos = (e) => {
        const rect = window.frameElement.getBoundingClientRect()
        window.parent.mouseX = e.clientX + rect.left
        window.parent.mouseY = e.clientY + rect.top
      }
      window.addEventListener('mousemove', updateParentPos)
      window.addEventListener('mousedown', () => { forwardEvent('down') })
      window.addEventListener('mouseup',   () => { forwardEvent('up'  ) })
    })
  }
}

// moves the cursor to a point 8px left and 8px up from the center of the specified
// element to compensate for the custom pointer's anchor offset, and then clicks it
//
//   @param {GhostCursor}            cursor
//   @param {string | ElementHandle} selector
//   @param {ClickOptions}           [options]
//   @param {{ jitter?: boolean }}   [behavior]
export async function clickOffset(cursor, selector, options, { jitter = false, pauseBefore = 0 } = {}) {
  if (pauseBefore > 0) { await setTimeout(pauseBefore) }

  const element = await cursor.getElement(selector)
  const box     = await element.boundingBox()

  const cx = box.x + box.width  / 2 - 8 // the 8px offset appears (to me) to be more centered
  const cy = box.y + box.height / 2 - 8

  if (jitter) { // drift slightly off-target, then back
    await cursor.moveTo({ x: cx - 10, y: cy - 2 }, { moveDelay: 0, moveSpeed: 100 })
    await cursor.moveTo({ x: cx,      y: cy     }, {               moveSpeed: 100, ...options })
  } else {
    await cursor.moveTo({ x: cx,      y: cy     }, options)
  }

  await cursor.click(undefined, options)
}
