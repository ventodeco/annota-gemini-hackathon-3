import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { AllowanceView, ScanAllowanceLine } from '../AllowancePanel'
import type { Entitlements, FeatureAllowance } from '@/lib/entitlements'

const resetAt = '2026-10-21T00:00:00Z'

function entitlements(features: FeatureAllowance[]): Entitlements {
  return { plan: 'free', usage: { period: 'daily', timezone: 'UTC', resetAt, features } }
}

const ocr: FeatureAllowance = { feature: 'ocr', unit: 'page', limit: 20, used: 3, remaining: 17, globalAvailable: true }
const explanation: FeatureAllowance = { feature: 'explanation', unit: 'request', limit: 100, used: 100, remaining: 0, globalAvailable: true }
const speech: FeatureAllowance = { feature: 'speech', unit: 'character', limit: 0, used: 0, remaining: 0, globalAvailable: true }

describe('AllowanceView', () => {
  it('shows each feature with remaining allowance, limits and reset', () => {
    render(<AllowanceView status="success" entitlements={entitlements([ocr, explanation, speech])} onRetry={vi.fn()} />)

    const items = within(screen.getByRole('list')).getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(items[0]).toHaveTextContent('Image scans')
    expect(items[0]).toHaveTextContent('17 pages left of 20')
    expect(items[1]).toHaveTextContent('Explanations')
    expect(items[1]).toHaveTextContent(/Daily limit reached\. Resets at/)
    expect(items[2]).toHaveTextContent('Not available during the beta.')
    expect(screen.getByText(/reset daily at .+ \(00:00 UTC\)/)).toBeInTheDocument()
  })

  it('announces loading', () => {
    render(<AllowanceView status="pending" onRetry={vi.fn()} />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
  })

  it('offers a retry when the allowance cannot load', async () => {
    const onRetry = vi.fn()
    render(<AllowanceView status="error" onRetry={onRetry} />)

    expect(screen.getByRole('alert')).toHaveTextContent("Your allowance couldn't be loaded.")
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('shows global capacity pauses separately from the personal limit', () => {
    render(
      <AllowanceView
        status="success"
        entitlements={entitlements([{ ...ocr, globalAvailable: false }])}
        onRetry={vi.fn()}
      />,
    )
    expect(screen.getByRole('listitem')).toHaveTextContent(/Beta capacity is used up for today/)
  })
})

describe('ScanAllowanceLine', () => {
  it('summarises remaining scans', () => {
    render(<ScanAllowanceLine allowance={ocr} resetAt={resetAt} />)
    expect(screen.getByText('17 of 20 image scans left today')).toBeInTheDocument()
  })

  it('announces an exhausted allowance and the PDF alternative', () => {
    render(<ScanAllowanceLine allowance={{ ...ocr, used: 20, remaining: 0 }} resetAt={resetAt} />)
    expect(screen.getByRole('status')).toHaveTextContent(/Daily limit reached\. Resets at .+ PDF reading is still available\./)
  })
})
