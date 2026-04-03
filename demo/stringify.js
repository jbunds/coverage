import fs from 'fs';
import { stringify } from '@puppeteer/replay';

const script = await stringify(JSON.parse(fs.readFileSync('./recording.json', 'utf8')));

fs.writeFileSync('./recording.js', script);
