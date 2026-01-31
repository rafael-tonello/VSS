# feat - HTTP DELETE should return count of removed items
Currently, when an HTTP DELETE request is made to remove a variable or a list (using '*' wildcard), the API returns a empty body and a 200 OK status code. However, it does not provide any information about how many items were actually removed, which can be useful for clients to know.

## proposal
Modify the HTTP DELETE response to include a count of the number of items that were removed. Return a json object with the count of removed items, for example:

```json
{
    "deleted":[
        "var1",
        "var2",
        "var3"
    ]
}
```


```
deleted.count=3
deleted.0=var1
deleted.1=var2
deleted.2=var3
```
