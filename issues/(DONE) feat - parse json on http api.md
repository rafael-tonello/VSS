# feat - accept and parse json on http api
Currently, http api only accepts single values via post, allowing to set only one variable at a time. This limitation makes it difficult to set multiple variables in a single request, leading to increased complexity and potential errors in client implementations.

## proposal
use the the header Content-Type: application/json to identify json payloads, and parse the json to set multiple variables in a single request.
