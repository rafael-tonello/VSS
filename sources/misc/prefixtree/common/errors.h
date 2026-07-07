#ifndef __ERRORS__H__ 
#define __ERRORS__H__ 

#include <string>
#include <utils.h>

using namespace std;
namespace Errors{
    using Error = string;

    extern Error NoError;
    extern Error TimeoutError;
    extern Error ConnectionIsNotStablishedError;

    Error createError(string message);
    Error createError(string message, Error nestedError);
    void forNestedErrors(Error errorWithNestedErrors, function<void(Error err)> f);

    //helper to createError(message, nestedError)
    Error derivateError(string message, Error nestedError);

    void forNestedErrors(Error errorWithNestedErrors, function<void(Error err)> f);

    //identify the errors using the first line (nested errors are not considered here)
    //        e1 = e1.substr(e1.find("\n"));
    //    if (e2.find("\n") != string::npos)
    //        e2 = e2.substr(e2.find("\n"));
//
    //    return Utils::trim(e1) == Utils::trim(e2);
    //}
//
    template <typename T>
    class ResultWithStatus{
    public:
        T result;
        Error status = Errors::NoError;

        ResultWithStatus(){}
        ResultWithStatus(T result, Error error): result(result), status(error){}
    };

}
 
#endif 
