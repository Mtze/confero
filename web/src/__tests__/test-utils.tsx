import { ReactNode } from 'react'
import { render } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

interface RenderOptions {
  initialEntries?: string[]
  routePath?: string
}

export function renderWithProviders(ui: ReactNode, options: RenderOptions = {}) {
  const { initialEntries = ['/'], routePath } = options
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  if (routePath) {
    return render(
      <QueryClientProvider client={qc}>
        <MemoryRouter initialEntries={initialEntries}>
          <Routes>
            <Route path={routePath} element={ui} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
  }

  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={initialEntries}>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>,
  )
}
