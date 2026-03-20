import { useCallback, useEffect, useRef, useState } from 'react'

export function useSpeechPlayback() {
  const [isPlaying, setIsPlaying] = useState(false)
  const audioRef = useRef<HTMLAudioElement | null>(null)
  const audioUrlRef = useRef<string | null>(null)

  const stop = useCallback(() => {
    if (audioRef.current) {
      audioRef.current.pause()
      audioRef.current.onended = null
      audioRef.current = null
    }
    if (audioUrlRef.current) {
      URL.revokeObjectURL(audioUrlRef.current)
      audioUrlRef.current = null
    }
    setIsPlaying(false)
  }, [])

  const play = useCallback(async (audioBlob: Blob) => {
    stop()

    const audioUrl = URL.createObjectURL(audioBlob)
    const audio = new Audio(audioUrl)
    audioUrlRef.current = audioUrl
    audioRef.current = audio

    audio.onended = () => {
      audioRef.current = null
      if (audioUrlRef.current) {
        URL.revokeObjectURL(audioUrlRef.current)
        audioUrlRef.current = null
      }
      setIsPlaying(false)
    }

    await audio.play()
    setIsPlaying(true)
  }, [stop])

  useEffect(() => {
    return () => stop()
  }, [stop])

  return { isPlaying, play, stop }
}
