import { beforeEach, describe, expect, it, vi } from 'vitest'
import { getAllSavedAnnotationsList, loadAnnotation } from '../storage'

describe('annotation storage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('ignores malformed saved annotation records', () => {
    vi.mocked(localStorage.getItem).mockReturnValue(JSON.stringify({
      1: {
        id: 1,
        user_id: 1,
        highlighted_text: 'テスト',
        nuance_data: {
          meaning: 'test',
          usageExample: 'テストです',
          usageTiming: 'when needed',
          wordBreakdown: 'テスト',
          alternativeMeaning: 'exam',
        },
        is_bookmarked: true,
        created_at: '2026-02-08T20:30:41+07:00',
      },
      2: {
        id: 2,
        highlighted_text: 'broken',
      },
    }))

    expect(loadAnnotation('1')?.highlighted_text).toBe('テスト')
    expect(loadAnnotation('2')).toBeNull()
    expect(getAllSavedAnnotationsList()).toHaveLength(1)
  })
})
