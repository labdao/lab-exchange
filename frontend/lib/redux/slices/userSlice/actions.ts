import { getAccessToken } from "@privy-io/react-auth"
import { createAction } from "@reduxjs/toolkit"
import backendUrl from "lib/backendUrl"

export const setError = createAction<string | null>('user/setError')

export const saveUserDataToServer = async (walletAddress: string): Promise<{ walletAddress: string }> => {
  let authToken;
  try {
    authToken = await getAccessToken()
    console.log('authToken', authToken)
  } catch (error) {
    console.log('Failed to get access token: ', error)
    throw new Error('Authentication failed')
  }

  const response = await fetch(`${backendUrl()}/user`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${authToken}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ walletAddress }),
  })

  if (!response.ok) {
    let errorMsg = 'An error occurred'
    try {
      const errorResult = await response.json()
      errorMsg = errorResult.message || errorMsg;
    } catch (e) {
      // Parsing JSON failed, retain the default error message.
    }
    console.log('errorMsg', errorMsg)
    throw new Error(errorMsg)
  }

  const result = await response.json()
  console.log('result', result)
  return result;
}
