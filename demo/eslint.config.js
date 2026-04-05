import globals          from 'globals';
import js               from '@eslint/js';
import { defineConfig } from 'eslint/config';

export default defineConfig([
  js.configs.recommended, {
    languageOptions: {
      globals: { ...globals.browser },
    },
  }
]);
