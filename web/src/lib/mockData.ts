import type { GetScanResponse } from './types'

const mockScans = new Map<string, GetScanResponse>()

const sampleJapaneseText = `以下は、上司にメールを送る際のガイドをまとめたものです。日本企業におけるメール作成に役立ちます。

上司向けのメールでは、件名を簡潔かつ具体的に設定し、冒頭と末尾には丁寧な挨拶を使い、本文は明瞭かつ簡潔にまとめます。また、信頼を構築し、透明性を保つために、重要な情報は最初に伝え、必要に応じて詳細を追加します。

件名の例：
- 「【報告】プロジェクト進捗について」
- 「【相談】来週の会議の件」
- 「【お願い】資料の確認をお願いします」

本文の構成：
1. 挨拶（おはようございます、お疲れ様ですなど）
2. 用件の簡潔な説明
3. 詳細情報（必要に応じて）
4. 締めの挨拶（よろしくお願いいたしますなど）

メールの送信タイミングも重要です。緊急でない場合は、営業時間内に送信することを心がけましょう。また、返信が必要な場合は、その旨を明確に伝えることも大切です。

課題/対応：
- 件名が不明確な場合、受信者が内容を理解しにくい
- 長文すぎるメールは読む負担が大きい
- 適切な敬語の使用が重要`

export function createMockScan(imageBlob: string, source: 'camera' | 'upload'): GetScanResponse {
  const id = `mock-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
  
  const mockScan: GetScanResponse = {
    scan: {
      id,
      sessionID: 'mock-session',
      source,
      status: 'ocr_done',
      createdAt: new Date().toISOString(),
    },
    ocrResult: {
      id: `ocr-${id}`,
      scanID: id,
      model: 'gemini-flash-1.5',
      language: 'ja',
      rawText: sampleJapaneseText,
      structuredJSON: JSON.stringify({ text: sampleJapaneseText }),
      promptVersion: '1.0',
      createdAt: new Date().toISOString(),
    },
    status: 'ocr_done',
  }

  mockScans.set(id, mockScan)
  
  if (typeof window !== 'undefined') {
    const scans = JSON.parse(sessionStorage.getItem('mockScans') || '[]')
    scans.push({ id, imageBlob, source })
    sessionStorage.setItem('mockScans', JSON.stringify(scans))
  }

  return mockScan
}

export function getMockScan(id: string): GetScanResponse | null {
  return mockScans.get(id) || null
}

export function getAllMockScans(): GetScanResponse[] {
  return Array.from(mockScans.values())
}

export function getMockImageUrl(id: string): string | null {
  if (typeof window === 'undefined') return null
  
  const scans = JSON.parse(sessionStorage.getItem('mockScans') || '[]')
  const scan = scans.find((s: { id: string }) => s.id === id)
  return scan?.imageBlob || null
}
