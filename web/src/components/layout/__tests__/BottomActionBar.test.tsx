import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import BottomActionBar from '../BottomActionBar'

describe('BottomActionBar', () => {
  it('renders explain button and disables it when disabled is true', () => {
    const onExplain = vi.fn()
    render(<BottomActionBar disabled onExplain={onExplain} />)

    const explainButton = screen.getByRole('button', { name: /explain this/i })
    expect(explainButton).toBeInTheDocument()
    expect(explainButton).toBeDisabled()
  })

  it('calls onExplain when explain button is clicked', async () => {
    const user = userEvent.setup()
    const onExplain = vi.fn()
    render(<BottomActionBar onExplain={onExplain} />)

    const explainButton = screen.getByRole('button', { name: /explain this/i })
    await user.click(explainButton)
    expect(onExplain).toHaveBeenCalledTimes(1)
  })

  it('labels the speech button and shows the icon for its state', () => {
    const onSpeech = vi.fn()
    const { container, rerender } = render(<BottomActionBar onSpeech={onSpeech} />)

    expect(screen.getByRole('button', { name: 'Play selected text' })).toBeEnabled()
    expect(container.querySelector('.lucide-volume-2')).toBeInTheDocument()

    rerender(<BottomActionBar onSpeech={onSpeech} isPlaying />)
    expect(screen.getByRole('button', { name: 'Stop playback' })).toBeEnabled()
    expect(container.querySelector('.lucide-square')).toBeInTheDocument()

    rerender(<BottomActionBar onSpeech={onSpeech} isPlaying isSpeechLoading />)
    expect(screen.getByRole('button', { name: 'Stop playback' })).toBeDisabled()
    expect(container.querySelector('.animate-spin')).toBeInTheDocument()
    expect(container.querySelector('.lucide-square')).not.toBeInTheDocument()
  })
})
