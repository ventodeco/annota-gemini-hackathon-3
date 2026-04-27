import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import Header from '@/components/layout/Header'
import BottomNavigation from '@/components/layout/BottomNavigation'
import { useAuth } from '@/contexts/useAuth'
import { useNavigate } from 'react-router-dom'
import { deleteAccount, updateUserPreferences } from '@/lib/api'
import type { User } from '@/lib/types'

type Language = User['preferred_language']

const LANGUAGE_LABELS: Record<Language, string> = {
  ID: 'Bahasa Indonesia',
  JP: '日本語',
  EN: 'English',
}

export default function ProfilePage() {
  const { user, logout, refreshUser } = useAuth()
  const navigate = useNavigate()

  const [lang, setLang] = useState<Language>(
    (user?.preferred_language as Language) ?? 'EN',
  )
  const [saveError, setSaveError] = useState<string | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const saveMutation = useMutation({
    mutationFn: (language: Language) =>
      updateUserPreferences({ preferredLanguage: language }),
    onSuccess: () => {
      setSaveError(null)
      void refreshUser()
    },
    onError: (err: Error) => {
      setSaveError(err.message || 'Failed to save preferences')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteAccount,
    onSuccess: () => {
      logout()
      navigate('/login')
    },
    onError: (err: Error) => {
      setDeleteError(err.message || 'Failed to delete account')
    },
  })

  const initial = user?.email?.[0]?.toUpperCase() ?? '?'
  const isSaveDisabled =
    lang === user?.preferred_language || saveMutation.isPending

  return (
    <div className="min-h-screen bg-gray-50 pb-28">
      <Header title="Profile" />

      <main className="pt-4 px-4 max-w-md mx-auto space-y-4">
        {/* Identity */}
        <Card className="p-4 flex items-center gap-3">
          <Avatar className="size-12">
            {user?.avatar_url && (
              <AvatarImage src={user.avatar_url} alt={user.email} />
            )}
            <AvatarFallback className="bg-gray-200 text-gray-600 text-base font-medium">
              {initial}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <p className="font-medium text-sm truncate">{user?.email}</p>
            <p className="text-xs text-gray-500 capitalize">
              Signed in via {user?.provider}
            </p>
          </div>
        </Card>

        {/* Edit preferred language */}
        <Card className="p-4 space-y-3">
          <div>
            <h2 className="text-sm font-medium">Preferred language</h2>
            <p className="text-xs text-gray-500 mt-0.5">
              Used for OCR and annotation responses.
            </p>
          </div>

          <Select
            value={lang}
            onValueChange={(v) => setLang(v as Language)}
          >
            <SelectTrigger className="w-full">
              <SelectValue>
                {LANGUAGE_LABELS[lang]}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {(Object.entries(LANGUAGE_LABELS) as [Language, string][]).map(
                ([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {label}
                  </SelectItem>
                ),
              )}
            </SelectContent>
          </Select>

          {saveError && (
            <p className="text-xs text-red-600">{saveError}</p>
          )}

          <Button
            onClick={() => saveMutation.mutate(lang)}
            disabled={isSaveDisabled}
            className="w-full"
          >
            {saveMutation.isPending ? 'Saving…' : 'Save'}
          </Button>
        </Card>

        {/* Danger zone */}
        <Card className="p-4 space-y-2 border border-red-100">
          <h2 className="text-sm font-medium text-red-700">Danger zone</h2>
          <p className="text-xs text-gray-500">
            Permanently delete your account and all stored ANNOTA data.
          </p>

          {deleteError && (
            <p className="text-xs text-red-600">{deleteError}</p>
          )}

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="ghost"
                className="text-red-600 hover:text-red-700 px-0 h-auto text-xs"
                disabled={deleteMutation.isPending}
              >
                {deleteMutation.isPending ? 'Deleting account…' : 'Delete account'}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete your account?</AlertDialogTitle>
                <AlertDialogDescription>
                  This permanently removes your account and all stored ANNOTA
                  data. This cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  className="bg-red-600 hover:bg-red-700"
                  onClick={() => deleteMutation.mutate()}
                >
                  Delete account
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </Card>
      </main>

      <BottomNavigation />
    </div>
  )
}
