'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { cn, formatDate } from '@/lib/utils'
import { User } from '@/types'
import { useAuthStore } from '@/lib/store'

type ModalMode = null | 'create' | 'edit' | 'delete'

export default function UsersPage() {
  const currentUser = useAuthStore((s) => s.user)
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [modal, setModal] = useState<ModalMode>(null)
  const [selected, setSelected] = useState<User | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  // Create form
  const [createForm, setCreateForm] = useState({ name: '', email: '', password: '', confirm_password: '' })

  // Edit form
  const [editForm, setEditForm] = useState<{ name: string; role: string; status: string }>({ name: '', role: '', status: '' })

  const fetchUsers = async () => {
    const res = await api.get<User[]>('/users')
    if (res.success && res.data) setUsers(res.data)
    setLoading(false)
  }

  useEffect(() => { fetchUsers() }, [])

  const openCreate = () => {
    setCreateForm({ name: '', email: '', password: '', confirm_password: '' })
    setError('')
    setModal('create')
  }

  const openEdit = (user: User) => {
    setSelected(user)
    setEditForm({ name: user.name, role: user.role, status: user.status })
    setError('')
    setModal('edit')
  }

  const openDelete = (user: User) => {
    setSelected(user)
    setError('')
    setModal('delete')
  }

  const closeModal = () => {
    setModal(null)
    setSelected(null)
    setError('')
  }

  const handleCreate = async () => {
    setSaving(true)
    setError('')
    const res = await api.post('/auth/register', createForm)
    setSaving(false)
    if (res.success) {
      closeModal()
      fetchUsers()
    } else {
      setError(res.error?.message || 'Failed to create user')
    }
  }

  const handleEdit = async () => {
    if (!selected) return
    setSaving(true)
    setError('')
    const res = await api.put(`/users/${selected.id}`, editForm)
    setSaving(false)
    if (res.success) {
      closeModal()
      fetchUsers()
    } else {
      setError(res.error?.message || 'Failed to update user')
    }
  }

  const handleDelete = async () => {
    if (!selected) return
    setSaving(true)
    setError('')
    const res = await api.delete(`/users/${selected.id}`)
    setSaving(false)
    if (res.success) {
      closeModal()
      fetchUsers()
    } else {
      setError(res.error?.message || 'Failed to remove user')
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Users</h1>
        <button
          onClick={openCreate}
          className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700"
        >
          New User
        </button>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        {loading ? (
          <div className="p-8 text-center text-gray-400">Loading...</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Email</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Role</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Last Login</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {users.map((user) => (
                  <tr key={user.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">{user.name}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{user.email}</td>
                    <td className="px-6 py-4">
                      <span className={cn('px-2 py-1 text-xs font-medium rounded-full',
                        user.role === 'ROOT' ? 'bg-purple-100 text-purple-800' : 'bg-gray-100 text-gray-800')}>
                        {user.role}
                      </span>
                    </td>
                    <td className="px-6 py-4">
                      <span className={cn('px-2 py-1 text-xs font-medium rounded-full',
                        user.status === 'ACTIVE' ? 'bg-green-100 text-green-800' :
                        user.status === 'SUSPENDED' ? 'bg-yellow-100 text-yellow-800' :
                        'bg-red-100 text-red-800')}>
                        {user.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {user.last_login_at ? formatDate(user.last_login_at) : '-'}
                    </td>
                    <td className="px-6 py-4 text-right space-x-2">
                      <button
                        onClick={() => openEdit(user)}
                        className="text-sm text-primary-600 hover:text-primary-800 font-medium"
                      >
                        Edit
                      </button>
                      {user.id !== currentUser?.id && (
                        <button
                          onClick={() => openDelete(user)}
                          className="text-sm text-red-600 hover:text-red-800 font-medium"
                        >
                          Remove
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal Backdrop */}
      {modal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="fixed inset-0 bg-black/50" onClick={closeModal} />

          {/* Create Modal */}
          {modal === 'create' && (
            <div className="relative bg-white rounded-xl shadow-xl w-full max-w-md p-6 z-10">
              <h2 className="text-lg font-semibold text-gray-900 mb-4">New User</h2>
              {error && <p className="mb-3 text-sm text-red-600 bg-red-50 rounded-lg p-3">{error}</p>}
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
                  <input
                    type="text"
                    value={createForm.name}
                    onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
                  <input
                    type="email"
                    value={createForm.email}
                    onChange={(e) => setCreateForm({ ...createForm, email: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                  <input
                    type="password"
                    value={createForm.password}
                    onChange={(e) => setCreateForm({ ...createForm, password: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
                  <input
                    type="password"
                    value={createForm.confirm_password}
                    onChange={(e) => setCreateForm({ ...createForm, confirm_password: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>
              </div>
              <div className="flex justify-end space-x-3 mt-6">
                <button onClick={closeModal} className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200">
                  Cancel
                </button>
                <button onClick={handleCreate} disabled={saving} className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50">
                  {saving ? 'Creating...' : 'Create'}
                </button>
              </div>
            </div>
          )}

          {/* Edit Modal */}
          {modal === 'edit' && selected && (
            <div className="relative bg-white rounded-xl shadow-xl w-full max-w-md p-6 z-10">
              <h2 className="text-lg font-semibold text-gray-900 mb-1">Edit User</h2>
              <p className="text-sm text-gray-500 mb-4">{selected.email}</p>
              {error && <p className="mb-3 text-sm text-red-600 bg-red-50 rounded-lg p-3">{error}</p>}
              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
                  <input
                    type="text"
                    value={editForm.name}
                    onChange={(e) => setEditForm({ ...editForm, name: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Role</label>
                  <select
                    value={editForm.role}
                    onChange={(e) => setEditForm({ ...editForm, role: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  >
                    <option value="USER">USER</option>
                    <option value="ROOT">ROOT</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
                  <select
                    value={editForm.status}
                    onChange={(e) => setEditForm({ ...editForm, status: e.target.value })}
                    className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
                  >
                    <option value="ACTIVE">ACTIVE</option>
                    <option value="INACTIVE">INACTIVE</option>
                    <option value="SUSPENDED">SUSPENDED</option>
                  </select>
                </div>
              </div>
              <div className="flex justify-end space-x-3 mt-6">
                <button onClick={closeModal} className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200">
                  Cancel
                </button>
                <button onClick={handleEdit} disabled={saving} className="px-4 py-2 text-sm font-medium text-white bg-primary-600 rounded-lg hover:bg-primary-700 disabled:opacity-50">
                  {saving ? 'Saving...' : 'Save'}
                </button>
              </div>
            </div>
          )}

          {/* Delete Confirmation Modal */}
          {modal === 'delete' && selected && (
            <div className="relative bg-white rounded-xl shadow-xl w-full max-w-sm p-6 z-10">
              <div className="flex items-center justify-center w-12 h-12 mx-auto mb-4 bg-red-100 rounded-full">
                <svg className="w-6 h-6 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                </svg>
              </div>
              <h2 className="text-lg font-semibold text-gray-900 text-center mb-1">Remove User</h2>
              <p className="text-sm text-gray-500 text-center mb-4">
                Are you sure you want to remove <strong>{selected.name}</strong> ({selected.email})?
              </p>
              {error && <p className="mb-3 text-sm text-red-600 bg-red-50 rounded-lg p-3">{error}</p>}
              <div className="flex justify-center space-x-3">
                <button onClick={closeModal} className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200">
                  Cancel
                </button>
                <button onClick={handleDelete} disabled={saving} className="px-4 py-2 text-sm font-medium text-white bg-red-600 rounded-lg hover:bg-red-700 disabled:opacity-50">
                  {saving ? 'Removing...' : 'Remove'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
