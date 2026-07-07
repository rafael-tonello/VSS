# feat - return 404 (optional, via settings), when http api receives get for empty vars
Currently, HTTP api every time receives a GET for a variable returns 200 ok, even if the variable does not exist. This can lead to confusion in clients that expect a 404 when the variable is not found.

## proposal
add a setting to allow returning 404 when a GET request is made for a variable that does not exist (or is empty). This setting can be enabled or disabled based on user preference, allowing for more flexible error handling in client applications.
