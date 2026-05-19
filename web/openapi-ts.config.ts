import { defineConfig } from '@hey-api/openapi-ts'

export default defineConfig({
  input: '../api/openapi.yaml',
  output: {
    path: 'src/api',
  },
  plugins: ['@hey-api/client-fetch'],
})
