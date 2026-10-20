import { SignUp } from '@clerk/react'
import { AuthPageLayout } from '@/pages/LoginPage'

export default function SignUpPage() {
  return (
    <AuthPageLayout>
      <SignUp routing="path" path="/sign-up" signInUrl="/login" fallbackRedirectUrl="/welcome" />
    </AuthPageLayout>
  )
}
