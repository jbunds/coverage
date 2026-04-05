import { readFileSync   } from 'fs';
import { validateAction } from '@action-validator/core';

const actionSource = readFileSync('./action.yml', 'utf8');
const state        = validateAction(actionSource);

if (state.errors.length > 0) {
  console.error('validation failed:', state.errors);
  process.exit(1);
}
