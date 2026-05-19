import { Routes, Route } from 'react-router-dom'
import { NavBar } from './features/auth/NavBar'
import { HomePage } from './pages/HomePage'
import { ConferenceDetailPage } from './pages/ConferenceDetailPage'
import { MyStarsPage } from './pages/MyStarsPage'
import { SettingsPage } from './pages/SettingsPage'
import { CalendarTokensPage } from './pages/CalendarTokensPage'
import { AuditPage } from './pages/AuditPage'
import { NotAuthorizedPage } from './pages/NotAuthorizedPage'

export default function App() {
  return (
    <div className="min-h-screen bg-gray-50">
      <NavBar />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/conferences/:id" element={<ConferenceDetailPage />} />
        <Route path="/stars" element={<MyStarsPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/calendar-tokens" element={<CalendarTokensPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/not-authorized" element={<NotAuthorizedPage />} />
      </Routes>
    </div>
  )
}
