'use client'

import React, { useEffect, useState } from 'react'
import { useSelector } from 'react-redux'

import {
  selectJobDetail,
} from '@/lib/redux'

import backendUrl from 'lib/backendUrl'

const LogViewer = () => {
  const [logs, setLogs] = useState('')
  const job = useSelector(selectJobDetail)

  useEffect(() => {
    // remove http:// or https:// from backendUrl for websocket
    let formattedBackendUrl = backendUrl().replace('http://', '')
    formattedBackendUrl = formattedBackendUrl.replace('https://', '')
    console.log(formattedBackendUrl)
    const ws = new WebSocket(`ws://${formattedBackendUrl}/jobs/${job.BacalhauJobID}/logs`)

    ws.onopen = () => {
      console.log('connected')
    }

    ws.onmessage = (event) => {
      setLogs((prevLogs) => prevLogs + event.data)
    }

    ws.onclose = () => {
      console.log('disconnected')
    }

    return () => {
      ws.close()
    }
  }, [job])

  return (
    <pre>
      {logs}
    </pre>
  )
}

export default LogViewer
