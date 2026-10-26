import { defineConfig } from 'i18next-cli'

export default defineConfig({
  locales: ['en', 'fr'],
  extract: {
    input: ['src/**/*.{ts,tsx}'],
    ignore: ['src/**/*.test.{ts,tsx}', 'src/api/schema.d.ts'],
    output: 'src/locales/{{language}}/{{namespace}}.json',
    defaultNS: 'common',
    primaryLanguage: 'en',
    removeUnusedKeys: true,
    sort: true,
    indentation: 2,
  },
})
