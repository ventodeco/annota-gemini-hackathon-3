import type { ReactNode } from 'react'
import { SignIn } from '@clerk/react'
import { useSearchParams } from 'react-router-dom'
import { safeRedirectPath } from '@/lib/authSession'

export function AuthPageLayout({ children }: { children: ReactNode }) {
  const [searchParams] = useSearchParams()
  const redirectPath = safeRedirectPath(searchParams.get('redirect_url'))
  // Carry the post-sign-in destination through the About detour.
  const aboutHref = redirectPath ? `/?redirect_url=${encodeURIComponent(redirectPath)}` : '/'

  return (
    <main className="min-h-screen bg-white flex flex-col items-center justify-center p-4 sm:p-6">
      <div className="flex flex-col items-center gap-6 max-w-md w-full">
        <img src="/logo-anota.png" alt="ANNOTA" className="h-16 w-auto" />
        <p className="text-center text-gray-700 text-base leading-relaxed">
          Read Japanese PDFs and page photos, and get English explanations of the words you select.{' '}
          <a href={aboutHref} className="inline-flex min-h-11 items-center font-medium underline underline-offset-2">
            About ANNOTA
          </a>
        </p>
        {children}
        <p className="text-center text-xs text-gray-500">
          By continuing, you agree to the{' '}
          <a href="/terms" className="font-medium underline underline-offset-2">
            Terms
          </a>{' '}
          and acknowledge the{' '}
          <a href="/privacy" className="font-medium underline underline-offset-2">
            Privacy Policy
          </a>
          .
        </p>
      </div>
    </main>
  )
}

export default function LoginPage() {
  return (
    <AuthPageLayout>
      {/* Clerk honours a same-origin redirect_url query parameter after sign-in. */}
      <SignIn routing="path" path="/login" signUpUrl="/sign-up" fallbackRedirectUrl="/welcome" />
    </AuthPageLayout>
  )
}
