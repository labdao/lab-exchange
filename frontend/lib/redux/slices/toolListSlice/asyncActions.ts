import backendUrl from "lib/backendUrl"

export const listTools = async (): Promise<any> => {
  const response = await fetch(`${backendUrl()}/tools`, {
    method: 'Get',
    headers: {
      'Content-Type': 'application/json',
    },
  })

  if (!response) {
    let errorText = "Failed to list Tools"
    try {
      errorText = await response.text()
      console.log(errorText)
    } catch (e) {
      // Parsing JSON failed, retain the default error message.
    }
    throw new Error(errorText)
  }

  const result = await response.json()
  return result;
}
