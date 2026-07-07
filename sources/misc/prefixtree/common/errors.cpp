#include  "errors.h" 

namespace Errors{
    Error NoError = "";
    Error TimeoutError = "Timeout reached";
    Error ConnectionIsNotStablishedError = "The connection is not stablished";
}

Errors::Error Errors::createError(string message){ return message; }
Errors::Error Errors::createError(string message, Errors::Error nestedError)
{
    //return message + ":\n" + "  >" + Utils::stringReplace(nestedError, "\n", "\n  "); 
    return message + ":\n" + "  >" + Utils::stringReplace(nestedError, "\n", "\n  "); 
}

Errors::Error Errors::derivateError(string message, Errors::Error nestedError){ 
    return Errors::createError(message, nestedError); 
}

void Errors::forNestedErrors(Error errorWithNestedErrors, function<void(Error err)> f)
{
    while (true)
    {
        auto cutPos = errorWithNestedErrors.find('>');
        if( cutPos != string::npos)
        {
            auto currErr = Utils::trim(errorWithNestedErrors.substr(0, cutPos));                
            errorWithNestedErrors = errorWithNestedErrors.substr(cutPos +1);
            if (currErr != "")
                f(currErr);
        }
        else
            break;
    }

    if (errorWithNestedErrors != "")
        f(errorWithNestedErrors);
}
