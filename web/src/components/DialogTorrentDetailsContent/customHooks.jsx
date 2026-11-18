import { useEffect, useRef, useState } from 'react'
import { cacheHost, settingsHost } from 'utils/Hosts'
import axios from 'axios'

export const useUpdateCache = hash => {
  const [cache, setCache] = useState({})
  const componentIsMounted = useRef(true)
  const timerID = useRef(null)

  useEffect(
    () => () => {
      // this function is required to notify "updateCache" when NOT to make state update
      componentIsMounted.current = false
    },
    [],
  )

  useEffect(() => {
    if (!hash) {
      clearInterval(timerID.current)
      return undefined
    }
    let inFlight = false
    const updateCache = newCache => componentIsMounted.current && setCache(newCache)
    timerID.current = setInterval(() => {
      if (inFlight) return
      inFlight = true
      axios
        .post(cacheHost(), { action: 'get', hash })
        .then(({ data }) => updateCache(data))
        // empty cache if error
        .catch(() => updateCache({}))
        .finally(() => {
          inFlight = false
        })
    }, 100)

    return () => clearInterval(timerID.current)
  }, [hash])

  return cache
}

export const useCreateCacheMap = cache => {
  const [cacheMap, setCacheMap] = useState([])

  useEffect(() => {
    const { PiecesCount, Pieces, Readers } = cache

    const map = []

    for (let i = 0; i < PiecesCount; i++) {
      const { Size, Length, Priority } = Pieces[i] || {}

      const newPiece = { id: i, percentage: (Size / Length) * 100 || 0, priority: Priority || 0 }

      Readers.forEach(r => {
        if (i === r.Reader) newPiece.isReader = true
        if (i >= r.Start && i < r.End) newPiece.isReaderRange = true
      })

      map.push(newPiece)
    }
    setCacheMap(map)
  }, [cache])

  return cacheMap
}

export const useGetSettings = () => {
  const [settings, setSettings] = useState()
  useEffect(() => {
    let active = true
    axios
      .post(settingsHost(), { action: 'get' })
      .then(({ data }) => {
        if (active) setSettings(data)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [])

  return settings
}
