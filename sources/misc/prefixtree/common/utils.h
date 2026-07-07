#ifndef __UTILSs__H__
#define __UTILSs__H__

#include <string>
#include <fstream>
#include <future>
#include <map>
#include <unistd.h>
#include <exception>
#ifdef THREAD_POOL_H
    #include <ThreadPool.h>
#endif
#include <sys/wait.h>
#include <sstream>
#include <algorithm>
#include <iostream>
#include <cctype>
#include <vector>

using namespace std;
    /**
     * @brief create a property and a function that both get and setts its value
     * 
     * @param TYPE is the type of the new property
     * @param NAME is the name of the property. Property will be named as _NAME and the get/set function will be NAME
     * * @param VALUE_FOR_NO_CHANGE a value that set function will user for detect if no changes is needed. Will be used as default value of the set/get function
     * */
    #define Utils_CreateGetSetProp(NAME, TYPE, VALUE_FOR_NO_CHANGE) protected: TYPE _##NAME; public: TYPE NAME(TYPE new_value = VALUE_FOR_NO_CHANGE){if (new_value != VALUE_FOR_NO_CHANGE) _##NAME = new_value; return _##NAME;}

    /**
     * @brief create a property and a function that both get and setts its value
     * 
     * @param TYPE is the type of the new property
     * @param NAME is the name of the property. Property will be named as _NAME and the get/set function will be NAME
     * * @param VALUE_FOR_NO_CHANGE a value that set function will user for detect if no changes is needed. Will be used as default value of the set/get function
     * @param ADDITIONAL_IMPL an aditional implementation to be inserted imediatelly after the set block and before the return of the property value
     * */
    #define Utils_CreateGetSetProp2(NAME, TYPE, VALUE_FOR_NO_CHANGE, ADDITIONAL_IMPL) protected: TYPE _##NAME; public: TYPE NAME(TYPE new_value = VALUE_FOR_NO_CHANGE){if (new_value != VALUE_FOR_NO_CHANGE) _##NAME = new_value; ADDITIONAL_IMPL; return _##NAME;}
    
    /**
     * @brief create a property and a function that both get and setts its value
     * 
     * @param TYPE is the type of the new property
     * @param NAME is the name of the property. Property will be named as _NAME and the get/set function will be NAME
     * @param VALUE_FOR_NO_CHANGE a value that set function will user for detect if no changes is needed. Will be used as default value of the set/get function
     * @param ADDITIONAL_IMPL an aditional implementation to be inserted imediatelly after the set block and before the return of the property value
     * @param ADDITIONAL_IMPL_ONLY_FOR_SET an aditional implementation to be inserted imediately before the property value setting process. You can use it to validate the value. The value will came in an argument named 'new_value' and you can use this session to change the 'new_value' before the property be changed.
     * @ ADDITIONAL_IMPL_IMEDIATELY_BEFORE_RETURN and additional implementation to be inserted imediatelly before the return of the get/set function. An local variable named 'result' is returned, so you can use this block to change this variable before the return;
     */
    #define Utils_CreateGetSetProp3(NAME, TYPE, VALUE_FOR_NO_CHANGE, ADDITIONAL_IMPL, ADDITIONAL_IMPL_ONLY_FOR_SET, ADDITIONAL_IMPL_IMEDIATELY_BEFORE_RETURN) private: TYPE _##NAME; public: TYPE NAME(TYPE new_value = VALUE_FOR_NO_CHANGE){if (new_value != VALUE_FOR_NO_CHANGE) {ADDITIONAL_IMPL_ONLY_FOR_SET; _##NAME = new_value; }; ADDITIONAL_IMPL; auto result = _##NAME; ADDITIONAL_IMPL_IMEDIATELY_BEFORE_RETURN;  return result;}
    
    #define Utils_CreateReadOnly(NAME, TYPE) protected: TYPE _##NAME; public: TYPE NAME(){ return _##NAME;}
    
    using named_lock_f = function<void()>;
    
    class Utils{
    private:
    #ifdef D__TESTING__
    private: 
    #endif
        static mutex names_locks_mutexes_mutex;
        static map<string, timed_mutex*> names_locks_mutexes;
        static int idCount;
        static bool srandOk;

        static void runSRand();
        static void initializeProxyList();


        static vector <string> validProxies;
        static char proxyListInitializedState;
    public:
        static void named_lock(string session_name, named_lock_f f, int timeout_ms = -1);
        static void named_lock_forceunlock(string session_name);
        static int64_t getCurrentTimeMicroseconds();
        static int64_t getCurrentTimeMilliseconds();
        static int64_t getCurrentTimeSeconds();

        static string StringToHex(string& input, size_t size);
        static string StringToHex(string& input);
        static string charVecToHex(char* data, size_t size);
        static string charVecToHex(const char* data, size_t size);
        static vector<string> splitString(string source, string split_by);
        static string strToUpper(std::string source);
        static string strToLower(std::string source);
        static string getOnly(string source, string validChars);
        
        struct SSystemReturn{
            string output;
            int exitCode;
        };
        static SSystemReturn ssystem (string, bool removeTheLastLF = true);
        static future<SSystemReturn> asystem(string, bool removeTheLastLF = true);
        static future<string> httpGet(string url, map<string, string> headers = {});
        static future<string> httpPost(string url, string body, string contentType = "application/json", map<string, string> headers = {});
        static void process_mem_usage(double& vm_usage, double& resident_set);
        static double process_vm_usage();
        static double process_resident_usage();
        /**
         * @brief Create a Unique Id object
         * 
         * @param validChars valid chars in the new id
         * @param size the maximum of chars in thenew id
         * @param prefix a text to bet put in the start of the new id (in addition to 'size' generated chars)
         * @param sufix a text to bet put in the end of the new id (in addition to 'size' generated chars)
         * @param includeTimeStampAtBegining include a chrono millisseconds count since ephoc
         * @return string 
         */
        static string createUniqueId(string validChars = "abcedfghijklmnopqrsuvxywzABCDEFGHIJKLMNOPQRSTUVXYWZ", int size = 20, string prefix = "UID", string sufix = "", bool includeTimeStampAtBegining = true);
        /**
         * @brief Create a Unique Id object using the specified format (aaaahhh000)
         * 
         * @param format format of the new id. Use 'A' for a-z char, 'H' for hex (0-f) char, '0' to a number (0-9) or '?' to get any char (number or letter)
         * @param prefix a text to bet put in the start of the new id (in addition generated chars)
         * @param sufix a text to bet put in the end of the new id (in addition generated chars)
         * @return string 
         */
        static string createUniqueId_customFormat(string format, string prefix = "", string sufix = "");
        static string createUnidqueId_guidFormat();

        static string readTextFileContent(string fileName);
        static void writeTextFileContent(string fileName, string content);
        static void appendTextFileContent(string fileName, string content);

        // static string findAndReplaceAll(std::string data, std::string toSearch, std::string replaceStr);

        static string downloadWithRandomProxy(string url, string destFileName, int maxTries = 5);

        static string stringReplace(string source, string replace, string by);
        static string stringReplace(string source, vector<tuple<string, string>> replaceAndByTuples);
        static string sr(string source, string replace, string by){return stringReplace(source, replace, by);};
        static string sr(string source, vector<tuple<string, string>> replaceAndByTuples){return stringReplace(source, replaceAndByTuples);};

        //replaces each char '?' by one of item of 'by' vector. ANother or string can be replaced by '?' in the 'marker' argument
        static string stringReplaceMarker(string source, vector<string> by, string marker = "?", bool use_TheArgBy_Circularly = false);
        //replaces each char '?' by one of item of 'by' vector. ANother or string can be replaced by '?' in the 'marker' argument
        static string srm(string source, vector<string> by, string marker = "?", bool use_TheArgBy_Circularly = false){ return stringReplaceMarker(source, by, marker, use_TheArgBy_Circularly);};
        
        static bool isNumber(string source);

        //static map<void*, string> getANameDB;
        //enum NameType{ALGORITHM_GENERATED, REAL_NAME_COMBINATION};
        //static string getAName(int number, NameType typeOfName = ALGORITHM_GENERATED, int AlgoGenMaxSyllables = 3);
        //static string getAName(void* p, NameType typeOfName = ALGORITHM_GENERATED, int AlgoGenMaxSyllables = 3);

        static string getNestedExceptionText(exception &e, string prefix ="", int level = 0);

        static string ltrim(string s);
        static string rtrim(string s);
        static string trim(std::string s);

        static string escapeString(string source);
        static string unescapeString(string source);

        template <typename S, typename T>
        static vector<T> mapVector(vector<S> source, function<T(S)> f)
        {
            vector<T> ret;
            ret.resize(source.size());
            std::transform(source.begin(), source.end(), ret.begin(), f);
            return ret;
        }

        template <typename T>
        static vector<T> filterVector(vector<T> source, function<bool(T)> f)
        {
            vector<T> result;
            std::copy_if(source.begin(), source.end(), std::back_inserter(result), f);
            return result;
        }

        #ifdef THREAD_POOL_H
            template<typename T>
            static future<void> parallel_foreach(vector<T> items, function<void(T, void* additionalArgs)> f, ThreadPool *tasker, void* additionalArgs = NULL)
            {
                vector<future<void>> pendingTasks = {};
                for (auto &c: items)
                {
                    pendingTasks.push_back(tasker->enqueue([&](T &item, void* argsp){
                        f(item, argsp);
                    }, c, additionalArgs));
                };

                /*return tasker->enqueue([&](auto pendingTasks2){
                    for (auto &c: pendingTasks2)
                        c.wait();
                }, pendingTasks);*/

                return tasker->enqueue([&](){
                    for (auto &c: pendingTasks)
                        c.wait();
                });
            }

            static future<void> parallel_for(int from, int to, function<void(int)> f, ThreadPool * tasker)
            {
                vector<future<void>> pendingTasks;
                for (int c = from; c != to; from > to ? c-- : c++)
                {
                    if (c != to)
                        pendingTasks.push_back(tasker->enqueue([&](int index){
                            f(index);
                        }, c));
                }

                return tasker->enqueue([&](){
                    for (auto &c: pendingTasks)
                        c.wait();
                });
            }
        #endif


    };

#endif
