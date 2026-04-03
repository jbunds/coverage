import puppeteer from 'puppeteer';

export const URL      = 'http://127.0.0.1:8000';
export const VIEWPORT = {
  deviceScaleFactor: 1,
  width:             1280,
  height:            720 };

export async function launchChrome(url = URL) {
  const options = {
    headless:        false,
    defaultViewport: null,
    executablePath:  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    args: [
      '--window-size=' + VIEWPORT.width + ',' + VIEWPORT.height,
      '--disable-web-security',
      '--disable-features=IsolateOrigins,site-per-process',
      '--disable-blink-features=AutomationControlled',
      url,
    ],
  };
  return await puppeteer.launch(options);
}
