import      { setTimeout                } from 'node:timers/promises'
import type { ClickOptions, GhostCursor } from 'ghost-cursor'
import puppeteer, {
  type Browser,
  type ElementHandle,
  type Frame,
  type LaunchOptions,
  type Page,
  type Viewport,
} from 'puppeteer'

declare global {
  interface Window {
    mouseX: number
    mouseY: number
  }
}

export const URL: string = 'http://127.0.0.1:8000'

export const VIEWPORT: Viewport = {
  deviceScaleFactor: 1,
  height:          720,
  width:          1280,
}

// launch Chrome with all required permissions enabled (see TODO)
export async function launchChrome(url: string = URL): Promise<Browser> {
  const options: LaunchOptions = {
    // TODO(jbunds): determine which specific flags are actually required,
    //               and remove those that are not strictly required
    args: [
      `--window-size=${VIEWPORT.width},${VIEWPORT.height}`,
      '--disable-web-security',
      '--disable-blink-features=AutomationControlled',
      url,
    ],
    channel:         'chrome',
    defaultViewport: null,
    headless:        false,
  }
  return puppeteer.launch(options)
}

// install a custom mouse pointer (analogous to GhostCursor's installMouseHelper)
export async function installMousePointer(page: Page): Promise<void> {
  await page.evaluate((initialX: number, initialY: number) => {
    const cursor = document.createElement('div')
    cursor.id = 'custom-pointer'
    cursor.innerHTML = `<svg width="32" height="32" viewBox="0 0 32 32">
<path d="M10 7v11.188l2.53-2.442 2.901 5.254 1.765-.941-2.775-5.202h3.604L10 7z" fill="black" stroke="white" stroke-width="1.5"/>
</svg>`
    Object.assign(cursor.style, {
      height: '32px', left:      '0',              pointerEvents: 'none', position: 'fixed',
      top:    '0',    transform: 'translate(0,0)', width:         '32px', zIndex:   '1000',
    })
    document.documentElement.appendChild(cursor)

    const path = cursor.querySelector('path')!
    const setGray  = () => { path.style.fill = 'gray'  }
    const setBlack = () => { path.style.fill = 'black' }

    window.mouseX = initialX
    window.mouseY = initialY

    const updatePos = (e: MouseEvent) => { window.mouseX = e.clientX; window.mouseY = e.clientY }
    window.addEventListener('mousemove', updatePos)
    window.addEventListener('mousedown', setGray)
    window.addEventListener('mouseup',   setBlack)
    window.addEventListener('message',   (event: MessageEvent) => {
      if (event.data.type === 'POINTER_CLICK') {
        if (event.data.action === 'down') setGray()
        if (event.data.action === 'up'  ) setBlack()
      }
    })

    function track() { // 60fps loop following GhostCursor's path
      cursor.style.transform = `translate3d(${window.mouseX}px, ${window.mouseY}px, 0)`
      requestAnimationFrame(track)
    }
    track()
  }, VIEWPORT.width / 2, VIEWPORT.height / 2)
}

// register event listeners in the child #code frame to forward cursor events to the main frame
export async function registerListeners(page: Page): Promise<void> {
  const frames: Frame[] = page.frames()
  for (const f of frames) {
    if (f === page.mainFrame()) continue
    await f.waitForSelector('body').catch(() => {})
    await f.evaluate(() => {
      const forwardEvent = (type: string) => {
        window.parent.postMessage({ action: type, type: 'POINTER_CLICK' }, '*')
      }
      const updateParentPos = (e: MouseEvent) => {
        const rect = (window.frameElement as HTMLElement).getBoundingClientRect()
        window.parent.mouseX = e.clientX + rect.left
        window.parent.mouseY = e.clientY + rect.top
      }
      window.addEventListener('mousemove', updateParentPos)
      window.addEventListener('mousedown', () => { forwardEvent('down') })
      window.addEventListener('mouseup',   () => { forwardEvent('up')   })
    })
  }
}

// move the cursor 8px left and up from the element's center (to compensate for the pointer's anchor offset), then click
export async function clickOffset(
  cursor:   GhostCursor,
  selector: ElementHandle,
  options?: ClickOptions,
  { jitter = false, pauseBefore = 0 }: { jitter?: boolean; pauseBefore?: number } = {},
): Promise<void> {
  if (pauseBefore > 0) { await setTimeout(pauseBefore) }

  const box = (await selector.boundingBox())!

  const cx = box.x + box.width  / 2 - 8 // the 8px offset appears (to me) to be more centered
  const cy = box.y + box.height / 2 - 8

  if (jitter) {
    await cursor.moveTo({ x: cx - 10, y: cy - 2 }, { moveDelay: 0, moveSpeed: 100 })
    await cursor.moveTo({ x: cx,      y: cy     }, {               moveSpeed: 100, ...options })
  } else {
    await cursor.moveTo({ x: cx,      y: cy     }, options)
  }

  await cursor.click(undefined, options)
}

// find a label in the tree whose trimmed text exactly matches `text`
export const findLabel = (tree: ElementHandle, text: string): Promise<ElementHandle> =>
  tree.evaluateHandle(
    (root, txt) => [...root.querySelectorAll('.tree-node > label')].find(label => label.textContent?.trim() === txt),
    text
  ) as Promise<ElementHandle>
