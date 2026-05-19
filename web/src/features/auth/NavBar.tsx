import { Link, NavLink } from 'react-router-dom'
import { useCurrentUser } from '../../lib/auth'

export function NavBar() {
  const { data: user } = useCurrentUser()

  return (
    <nav className="border-b border-gray-200 bg-white">
      <div className="mx-auto max-w-5xl px-4 flex h-14 items-center justify-between">
        <div className="flex items-center gap-6">
          <Link to="/" className="text-lg font-bold text-blue-700">Confero</Link>
          <NavLink
            to="/"
            end
            className={({ isActive }) =>
              `text-sm ${isActive ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}`
            }
          >
            Conferences
          </NavLink>
          {user && (
            <>
              <NavLink
                to="/stars"
                className={({ isActive }) =>
                  `text-sm ${isActive ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}`
                }
              >
                My Stars
              </NavLink>
              <NavLink
                to="/settings"
                className={({ isActive }) =>
                  `text-sm ${isActive ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}`
                }
              >
                Settings
              </NavLink>
              <NavLink
                to="/calendar-tokens"
                className={({ isActive }) =>
                  `text-sm ${isActive ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}`
                }
              >
                Calendar
              </NavLink>
              {user.roles.includes('admin') && (
                <NavLink
                  to="/audit"
                  className={({ isActive }) =>
                    `text-sm ${isActive ? 'text-blue-700 font-medium' : 'text-gray-600 hover:text-gray-900'}`
                  }
                >
                  Audit
                </NavLink>
              )}
            </>
          )}
        </div>
        <div className="flex items-center gap-4 text-sm">
          {user ? (
            <>
              <span className="text-gray-600">{user.name}</span>
              <a href="/auth/logout" className="text-gray-600 hover:text-gray-900">Logout</a>
            </>
          ) : (
            <a href="/auth/login" className="text-blue-600 hover:text-blue-800 font-medium">Login</a>
          )}
        </div>
      </div>
    </nav>
  )
}
