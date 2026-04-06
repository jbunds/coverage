import globals          from 'globals';
import js               from '@eslint/js';
import progress         from 'eslint-plugin-file-progress';
import { defineConfig } from 'eslint/config';

export default defineConfig([
  js.configs.recommended,
  progress.configs.recommended, {
    plugins: {
      progress,
    },
    languageOptions: {
      globals: { ...globals.browser },
    },
  }
]);
