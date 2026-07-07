# feat - max depth for apis
Currently, when you request variables from an API using wildcard or rules that returns multiple variables, vss returns all the variables that match the request. It works very well in small databases, but with longer sets of variables (big databases) it can lead to memory and perfromance issues.

## proposal
add a header fot apis (both HTTP and VSP now supports headers) called "maxdepth" that will limit the amount of chldren levels that vss will look for when matching variables. 

Also, add a header called "maxresults" that will limit the amount of variables that vss will return for a request.

Furthermore, add configurations equivalent to both headers. When setting are present, it will be mandatory over the headers (if requests ask for more depth or results than the configuration, the request will be rejected with an error message or will return only the amount of variables allowed by the configuration. This behaviour - return error or limited set of variables - can also be setted with a configuration option).