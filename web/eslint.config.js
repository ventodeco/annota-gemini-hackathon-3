import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import react from 'eslint-plugin-react'
import eslintConfigPrettier from 'eslint-config-prettier/flat'
import eslintPluginBetterTailwindcss from 'eslint-plugin-better-tailwindcss'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    plugins: {
      react,
    },
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    rules: {
      'react-hooks/exhaustive-deps': 'error',
      'react-hooks/rules-of-hooks': 'error',

      '@typescript-eslint/no-explicit-any': 'error',

      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      '@typescript-eslint/no-unused-expressions': 'error',

      'react/no-unused-state': 'error',
      'react/prop-types': 'off',
    },
  },
  // Components using shadcn/ui patterns - disable better-tailwindcss/no-unknown-classes
  {
    files: ['**/components/**/*.{ts,tsx}'],
    ignores: ['**/node_modules/**'],
    plugins: {
      'better-tailwindcss': eslintPluginBetterTailwindcss,
    },
    rules: {
      'better-tailwindcss/no-unknown-classes': 'off',
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    ignores: ['**/components/**/*.{ts,tsx}', '**/node_modules/**'],
    plugins: {
      'better-tailwindcss': eslintPluginBetterTailwindcss,
    },
    rules: {
      ...eslintPluginBetterTailwindcss.configs.recommended.rules,
    },
    settings: {
      'better-tailwindcss': {
        entryPoint: 'src/index.css',
      },
    },
  },
  eslintConfigPrettier,
])
