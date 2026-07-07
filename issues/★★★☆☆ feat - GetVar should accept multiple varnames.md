# feat - GetVar should accept multiple varnames

Currently, vss GetVar (including in the VSTP protocol) only accept one varname per request. Of course you can use the wildcard '*' to get children vars, but sometimes you need to get many specific vars, and it can only be done by many requests. So, GetVar should accept an array of varnames or a new Command (GetVar's' or GetMultipleVars) should be created. The best option, if possible, is adapting the GetVar to accept both a string (single varname) or an array of strings (multiple varnames).

If more complex system, it coult reduce the number of tcp interactions needed to get many vars, improving performance and simplifying the client code.

