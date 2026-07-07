#include "utils.h"

mutex Utils::names_locks_mutexes_mutex;
map<string, timed_mutex*> Utils::names_locks_mutexes;
bool Utils::srandOk = false;
char Utils::proxyListInitializedState = 'a';
vector <string> Utils::validProxies;



void Utils::named_lock(string session_name, named_lock_f f, int timeout_ms)
{
    Utils::names_locks_mutexes_mutex.lock();
    if (Utils::names_locks_mutexes.count(session_name) <= 0)
    {
        Utils::names_locks_mutexes[session_name] = new timed_mutex();
    }
    Utils::names_locks_mutexes_mutex.unlock();

    if (timeout_ms > 0)
        Utils::names_locks_mutexes[session_name]->try_lock_for(std::chrono::milliseconds(timeout_ms));
    else
        Utils::names_locks_mutexes[session_name]->lock();
    try
    {
        f();
        Utils::names_locks_mutexes[session_name]->unlock();
    }
    catch(...)
    {
        Utils::names_locks_mutexes[session_name]->unlock(); 
        throw;
    }


}

void Utils::named_lock_forceunlock(string session_name)
{
    if (Utils::names_locks_mutexes.count(session_name))
        Utils::names_locks_mutexes[session_name]->unlock();
}

int64_t Utils::getCurrentTimeMicroseconds()
{
    return std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::system_clock::now().time_since_epoch()).count();
}

int64_t Utils::getCurrentTimeMilliseconds()
{
    return std::chrono::duration_cast<std::chrono::milliseconds>(std::chrono::system_clock::now().time_since_epoch()).count();
}

int64_t Utils::getCurrentTimeSeconds()
{
    return std::chrono::duration_cast<std::chrono::seconds>(std::chrono::system_clock::now().time_since_epoch()).count();
}

string Utils::StringToHex(string &input, size_t size)
{
    static const char hex_digits[] = "0123456789ABCDEF";

    std::string output;
    size_t outputSize = size * 2;
    output.reserve(outputSize);
    for (auto count = 0; count < size; count++)
    {
        auto c = input[count];
        output.push_back(hex_digits[c >> 4]);
        output.push_back(hex_digits[c & 15]);
    }
    return output;
}

string Utils::StringToHex(string& input) 
{
    return StringToHex(input, input.size());

}

string Utils::charVecToHex(char* data, size_t size)
{
    static const char hex_digits[] = "0123456789ABCDEF";

    std::string output;
    size_t outputSize = size * 2;
    output.reserve(outputSize);
    for (auto count = 0; count < outputSize; count++)
    {
        auto c = data[count];
        output.push_back(hex_digits[c >> 4]);
        output.push_back(hex_digits[c & 15]);
    }
    return output;
}

string Utils::charVecToHex(const char* data, size_t size)
{
    return charVecToHex((char*)data, size);
}

vector<string> Utils::splitString(string source, string split_by)
{
    vector<string> result;
    
    while (true)
    {
        auto pos = source.find(split_by);
        if (pos != string::npos)
        {
            result.push_back(source.substr(0, pos));
            source = source.substr(pos + split_by.size());
        }
        else
            break;
    }

    if (source != "")
        result.push_back(source);

    return result;
}

string Utils::strToUpper(std::string source)
{
    std::string result = source;

    for (char& c : result) {
        c = std::toupper(c);
    }

    return result;
}

string Utils::strToLower(std::string source)
{
    std::string result = source;

    for (char& c : result) {
        c = std::tolower(c);
    }

    return result;
}

string Utils::getOnly(string source, string validChars)
{
    string ret = "";
    for (auto &c: source)
        if (validChars.find(c) != string::npos)
            ret.push_back(c);

    return ret;
}

Utils::SSystemReturn Utils::ssystem (string command, bool removeTheLastLF) {

    const int bufferSize = 128;
    char buffer[bufferSize];
    std::string output;

    // Executar o comando e redirecionar stdout e stderr para o mesmo arquivo temporário
    FILE* pipe = popen((command + " 2>&1").c_str(), "r");
    if (pipe == nullptr) {
        std::cerr << "Erro ao executar o comando." << std::endl;
        return {"", -1};
    }

    // Ler a saída do subprocesso (stdout e stderr)
    while (fgets(buffer, bufferSize, pipe) != nullptr) {
        output += buffer;
    }

    // Fechar o subprocesso
    int retStatus = pclose(pipe);

    // Remover a quebra de linha final, se necessário
    if (removeTheLastLF && !output.empty() && output.back() == '\n') {
        output.pop_back();
    }

    return {output, retStatus};
}

future<Utils::SSystemReturn> Utils::asystem(string command, bool removeTheLastLF)
{
    return std::async(std::launch::async, [&](string c, bool rtllf){
        return ssystem(c, rtllf);
    }, command, removeTheLastLF);
}

future<string> Utils::httpGet(string url, map<string, string> headers)
{
    string cmd = "curl -sS -k -X GET " +
    url + " "+
    "-H 'Accept: */*' ";
    for (auto &c: headers)
        cmd += "-H '"+c.first+": "+c.second+"' ";
    
    return std::async(std::launch::async, [](string cmd2){
        auto ret = ssystem(cmd2);
        if (ret.output.find("curl") == string::npos)
            return ret.output;
        else
            throw std::runtime_error("Curl error: "+ret.output);

    }, cmd);

    

}

future<string> Utils::httpPost(string url, string body, string contentType, map<string, string> headers)
{
    string cmd = "curl -sS -k -X POST " +
    url + " "+
    "-H 'Accept: */*' " +
    "-H 'Content-Type: "+contentType+"' ";
    for (auto &c: headers)
        cmd += "-H '"+c.first+": "+c.second+"' " ;
    cmd += "-d '"+body+"'";

    return std::async(std::launch::async, [](string cmd2){
        auto ret = ssystem(cmd2);
        if (ret.output.find("curl") == string::npos)
            return ret.output;
        else
            throw std::runtime_error("Curl error: "+ret.output);

    }, cmd);

}

void Utils::process_mem_usage(double& vm_usage, double& resident_set)
{
    vm_usage     = 0.0;
    resident_set = 0.0;

    // the two fields we want
    unsigned long vsize;
    long rss;
    {
        std::string ignore;
        std::ifstream ifs("/proc/self/stat", std::ios_base::in);
        ifs >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore
                >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore >> ignore
                >> ignore >> ignore >> vsize >> rss;
    }

    long page_size_kb = sysconf(_SC_PAGE_SIZE) / 1024; // in case x86-64 is configured to use 2MB pages
    vm_usage = vsize / 1024.0;
    resident_set = rss * page_size_kb;
}

double Utils::process_vm_usage()
{
    double a, b;
    process_mem_usage(a, b);
    return a;
}

double Utils::process_resident_usage()
{
    double a, b;
    process_mem_usage(a, b);
    return b;
}


void Utils::runSRand()
{
    if (Utils::srandOk == false)
    {
        srand((unsigned)time(0)); 
        Utils::srandOk = true;
    }
}

string Utils::createUniqueId(string validChars, int size, string prefix, string sufix, bool includeTimeStampAtBegining)
{

    Utils::runSRand();
    auto i = to_string(std::chrono::duration_cast<std::chrono::microseconds>(std::chrono::system_clock::now().time_since_epoch()).count());
    //auto i2 = to_string(std::chrono::duration_cast<std::chrono::nanoseconds>(std::chrono::system_clock::now().time_since_epoch()).count());
    
    string i2 = "";


    int randomCharsCount = size - (includeTimeStampAtBegining ? i.size() : 0);
    for (int c = 0; c < randomCharsCount; c++)
    {
        i2 += validChars[rand() % validChars.size()];
    }

    string tmp = prefix + (includeTimeStampAtBegining ? i: "") + i2 + sufix;
    //return Utils::StringToHex(tmp);
    return tmp;

}

string Utils::createUniqueId_customFormat(string format, string prefix, string sufix)
{
    //auto chartList = Utils::createUniqueId(validChars, format.size(), "", "", includeTimeStampAtBegining);

    stringstream ret;

    Utils::runSRand();
    ret.clear();
    string chars = "0123456789abcdefghijklmnopqrstuvxywz";

    auto getAlpha = [chars](){ return chars[(rand() % (chars.size()-10))+10]; };
    auto getNum = [chars](){ return chars[rand() % 10]; };
    auto getHex = [chars](){ return chars[rand() % 16]; };
    auto getAlphaRandUpper = [getAlpha, chars](){
        if (((rand() % 2)+1) == 2)
            return (char)toupper(getAlpha());
        else
            return getAlpha();

    };

    
    for (size_t c = 0; c < format.size(); c++)
    {
        if (format[c] == 'a')
            ret << getAlpha();
        else if (format[c] == 'A')
            ret << (char)toupper(getAlpha());
        else if (format[c] == 'h')
            ret << getHex();
        else if (format[c] == 'H')
            ret << (char)toupper(getHex());
        else if (format[c] == '0')
            ret << getNum();
        else if (format[c] == '?')
            ret << (vector<function<char()>>({ getAlphaRandUpper, getNum, getHex}))[rand() % 3]();
        else
            ret << format[c];
    }
    string ret2 = ret.str();
    return prefix + ret2  + sufix;

}

string Utils::createUnidqueId_guidFormat()
{
    /*auto charList = Utils::createUniqueId("0123456789abcdef", 32, "", "", false);

    string ret = ""+
                    charList.substr(0, 8) +  "-"+
                    charList.substr(8, 4) +  "-"+
                    charList.substr(12, 4) + "-"+ 
                    charList.substr(16, 4) + "-"+ 
                    charList.substr(20); */
    string ret = Utils::createUniqueId_customFormat("hhhhhhhh-hhhh-hhhh-hhhh-hhhhhhhhhhhh");
    return ret;
}



string Utils::readTextFileContent(string fileName)
{
    ifstream f;
    stringstream content;
    string result = "";
    f.open(fileName);
    if (f.is_open())
    {
        content << f.rdbuf();
        result =  content.str();
        f.close();
    }

    return result;
}

void Utils::writeTextFileContent(string fileName, string content)
{
    ofstream f(fileName);
    if (f.is_open())
    {
        f << content;
        f.close();
    }
}

void Utils::appendTextFileContent(string fileName, string content)
{
    ofstream f(fileName, std::ios_base::app);
    if (f.is_open())
    {
        f << content;
        f.close();
    }
}

// // string Utils::findAndReplaceAll(std::string data, std::string toSearch, std::string replaceStr)
// // {
// //     // Get the first occurrence
// //     size_t pos = data.find(toSearch);
// //     // Repeat till end is reached
// //     while( pos != std::string::npos)
// //     {
// //         // Replace this occurrence of Sub String
// //         data.replace(pos, toSearch.size(), replaceStr);
// //         // Get the next occurrence from the current position
// //         pos =data.find(toSearch, pos + replaceStr.size());
// //     }

// //     return data;
// // }


// function<string()> UtilsGetNextProxy;
// mutex UtilsProxiesListLock, UtilsValidProxiesListLock;
// void Utils::initializeProxyList()
// {
// vector<string> UtilsProxiesToVerify = {"191.102.107.235:999","20.47.108.204:8888","103.169.70.58:3127","203.189.89.153:8080","175.106.17.62:57406","36.95.156.125:6969","120.26.123.95:8010","27.208.65.79:8060","8.242.142.182:999","95.216.64.229:20034","212.46.230.102:6969","36.138.54.45:1080","161.35.21.55:3128","116.80.58.110:3128","45.79.230.234:443","185.74.7.51:3128","183.238.50.225:80","172.104.34.170:8080","103.117.231.42:80","103.164.223.75:8080","223.96.90.216:8085","8.210.83.33:80","35.194.215.58:9090","178.88.185.2:3128","124.70.46.14:3128","164.70.116.39:3128","187.115.192.227:8081","182.61.32.240:9001","116.80.84.159:3128","151.22.181.212:8080","186.176.212.213:9080","101.68.17.211:8085","139.255.136.232:8080","140.227.238.217:3128","161.117.250.122:80","103.24.212.134:8080","190.69.153.82:999","208.52.137.171:5555","140.227.203.41:3128","118.140.160.85:80","103.76.253.66:3129","66.211.155.34:8080","121.226.212.177:7082","103.47.66.150:8080","140.227.228.30:3128","45.7.133.187:999","36.112.155.138:80","115.220.6.38:17890","39.99.129.85:30003","116.80.49.253:3128","190.90.86.65:999","217.16.188.90:3128","140.246.36.217:8888","23.94.220.121:80","183.247.202.208:30001","140.227.127.228:80","92.42.109.189:1080","77.82.88.58:8080","173.249.57.9:443","195.211.219.146:5555","103.148.39.50:83","218.90.23.93:8118","196.219.202.74:8080","184.180.90.226:8080","177.23.149.170:3128","185.222.123.98:3128","45.227.95.2:8085","45.162.126.1:999","190.61.43.210:999","115.220.1.139:17890","41.65.236.58:1981","206.253.164.122:80","45.7.132.194:999","95.216.194.46:1081","41.65.193.100:1976","47.89.153.213:80","120.71.13.38:8901","103.60.173.114:8080","34.146.157.204:9090","186.3.38.200:999","187.108.39.64:6666","103.161.164.107:8181","140.227.69.254:4321","112.6.117.135:8085","217.196.20.150:8080","178.217.172.206:55443","103.213.237.129:8080","154.236.168.181:1981","202.106.72.238:6666","177.101.195.154:3128","139.59.210.73:80","5.58.178.99:41890","114.93.24.102:9000","119.40.83.138:8080","103.153.230.2:80","183.89.188.159:8080","88.200.155.203:3128","201.150.117.189:999","14.170.154.10:8080","191.102.74.113:8080","168.90.121.151:8080","154.236.179.226:1976","190.90.83.209:999","125.73.131.137:9091","202.150.150.210:8080","168.227.56.104:8080","45.33.26.53:3128","144.22.225.229:8888","45.4.85.128:999","116.206.243.50:80","118.99.87.171:8080","59.120.147.82:3128","45.183.76.241:8080","170.239.255.2:55443","190.110.99.100:999","164.70.70.77:3128","103.19.58.158:3127","186.218.119.183:8080","190.26.201.194:8080","201.182.240.65:999","41.65.236.44:1976","193.29.59.233:80","139.255.123.3:8080","116.21.120.26:808","8.142.35.42:443","45.174.248.43:999","177.250.104.185:8080","1.10.141.220:54620","180.250.66.82:8080","182.18.83.42:6666","167.71.205.73:3128","140.246.224.68:8888","103.171.181.119:80","68.183.180.222:3128","201.20.110.35:666","36.251.148.41:8118","103.1.104.14:8080","14.207.0.67:8080","38.91.57.43:3128","128.199.108.29:3128","186.183.220.2:8080","80.90.173.118:8080","154.236.162.59:1981","176.32.185.22:8080","93.171.192.28:8080","185.82.99.150:9091","123.56.68.23:8080","203.81.87.186:10443","202.150.150.211:8080","164.70.121.242:3128","143.110.153.93:3128","213.135.118.150:3128","187.1.57.206:20183","183.88.212.184:8080","122.3.41.154:8090","118.99.103.100:8080","183.82.116.56:8080","182.43.183.11:8888","190.110.111.152:999","103.156.75.37:8181","185.233.37.83:80","103.119.60.12:80","200.39.63.206:999","174.138.54.143:3128","103.160.212.214:8080","111.90.179.74:8080","51.91.157.66:80","14.205.195.166:8085","41.190.11.94:8080","43.155.92.192:59394","222.74.73.202:42055","41.65.163.84:1981","45.115.175.112:57919","36.92.85.66:8080","103.13.29.17:8080","8.218.213.95:10809","114.249.112.171:9000","117.239.240.202:53281","135.181.216.20:5566","185.177.125.108:8001","46.44.41.153:8081","114.5.199.198:8182","147.182.205.248:3128","41.216.68.254:41890","201.77.110.1:999","222.241.252.104:8060","72.80.242.101:8080","190.110.99.101:999","14.207.59.105:8080","152.70.48.9:3128","41.65.181.133:8089","103.207.1.82:8080","190.110.111.133:999","114.88.242.234:55443","164.70.64.126:3128	","103.89.7.19:3128","190.124.30.43:999","163.172.157.7:80","124.153.20.110:8080","202.65.158.235:82","194.87.102.116:81","164.70.72.55:3128","189.129.107.116:999","12.151.56.30:80","152.228.163.151:80","101.132.186.175:9090","202.69.45.22:8080","186.103.179.50:60080","103.20.204.104:80","103.8.249.97:80","41.204.87.90:8080","82.66.126.34:8118","161.117.85.96:80","45.179.164.1:999","167.249.31.104:999","35.225.244.185:80","14.207.144.47:9080","203.150.128.66:8080","41.65.163.85:1976","176.193.113.57:55443","164.70.119.216:3128","95.216.12.141:20010","209.141.35.151:80","79.124.78.29:443","220.247.171.242:8080","186.13.50.16:8080","102.64.66.105:8080","58.246.58.150:9002","103.53.170.199:3128","121.36.31.146:3389","217.11.79.232:8080","120.55.190.250:3128","58.234.116.197:8193","190.93.209.202:3128","8.215.27.71:9009","152.231.25.126:8080","175.106.10.164:8089","195.154.243.54:80","129.159.88.228:80","23.88.97.167:3128","52.183.8.192:3128","116.80.70.3:3128","186.96.156.232:3128","203.191.8.67:8080","84.42.124.102:3128","41.65.0.195:1981","35.200.186.201:8081","59.66.17.14:7890","171.38.146.51:8085","189.199.106.202:999","54.36.176.76:1080","103.121.38.138:80","144.217.7.157:5566","45.115.178.137:80","92.45.19.46:8080","51.91.124.151:80","119.28.155.202:99","139.162.57.5:3128","171.228.149.100:8081","45.187.87.205:8080","103.76.184.203:3128","103.149.162.194:80","124.158.88.56:54555","103.125.162.134:84","36.67.27.189:39674","45.170.100.193:999","117.119.65.26:7890","110.232.80.11:8080","107.151.182.247:80","177.53.154.213:999","103.120.175.233:8080","5.189.128.171:80","47.119.167.255:7890","197.246.174.14:3030","58.96.150.30:8080","39.175.75.8:30001","103.124.138.162:3127","219.87.191.203:80","213.212.210.253:1976","43.255.113.232:84","41.65.236.57:1981","94.136.197.126:3128","43.228.125.189:8080","165.22.52.250:3128","116.0.4.50:8080","42.2.205.141:8080","113.125.149.214:8888","45.86.180.17:8080","203.86.236.148:3128","110.165.23.162:80","39.98.225.204:30002","164.70.77.211:3128","125.113.229.148:7890","206.189.149.59:80","45.175.160.50:999","45.184.155.253:6969","101.68.17.134:8085","123.241.38.158:80","111.178.66.103:1080","181.78.21.174:999","47.90.200.121:24008","83.221.208.217:1981","139.255.25.84:3128","195.128.103.104:80","45.179.164.9:80","47.251.5.248:80","140.227.229.54:3128","103.239.200.246:1337","172.105.97.26:3128","138.197.156.153:80","94.71.234.214:8080","159.65.3.235:3128","202.62.93.194:8080","45.174.79.1:999","182.79.20.54:3127","202.57.55.242:45112","35.233.228.51:3128","196.1.95.117:80","49.0.39.10:8080","47.88.15.217:80","200.25.48.72:3128","191.97.6.212:999","182.66.101.134:3130","198.11.183.14:80","209.141.56.127:80","51.15.54.21:3128","102.164.252.150:8080","45.189.19.7:45356","157.90.251.43:80","103.214.109.70:80","41.65.36.166:1981","104.45.128.122:80","8.136.244.49:8088","160.16.82.58:3128","149.154.70.112:3128","154.127.36.138:8080","74.208.128.22:80","140.227.225.120:3128","201.238.242.38:999","103.158.121.79:8085","47.100.171.141:8089","58.20.232.245:9091","140.227.239.34:3128","108.18.151.99:80","36.92.93.61:8080","116.80.41.12:80","186.101.99.82:999","186.67.26.178:999","74.143.245.221:80","190.211.81.211:80","54.37.160.91:1080","190.90.242.208:999","112.120.42.82:80","41.65.236.37:1976","162.55.84.170:3128","165.154.92.146:8888","182.253.235.165:8080","164.70.118.39:3128","140.227.200.4:3128","103.213.213.14:83","185.110.191.27:3128","201.184.171.244:999","95.216.64.229:20012","103.80.1.2:80","160.251.97.210:3128","58.82.154.3:8080","45.5.68.25:999","47.241.72.41:80","61.9.48.169:1337","180.178.130.238:8080","20.105.253.176:8080","211.187.22.126:8193","103.242.104.116:8181","201.186.132.189:999","5.59.136.230:8080","103.197.251.204:80","190.13.81.153:9992","103.226.169.14:83","5.183.182.34:3128","180.193.213.42:8080","110.166.228.238:8901","182.66.101.134:3128","47.242.242.32:80","113.28.90.66:9480","125.123.21.78:1088","119.28.30.159:60080","150.95.81.46:3128","154.236.168.169:1976","54.37.160.88:1080","118.99.102.215:8080"};
//     Utils::proxyListInitializedState = 'b';

//     UtilsGetNextProxy = [&](){
//         string ret = "";
//         UtilsProxiesListLock.lock();
//         if (UtilsProxiesToVerify.size() > 0)
//         {
//             ret = UtilsProxiesToVerify[0];
//             UtilsProxiesToVerify.erase(UtilsProxiesToVerify.begin());
//         }

//         UtilsProxiesListLock.unlock();

//         return ret;
//     };

//     for (int c = 0; c < 10; c++)
//     {
//         thread *th = new thread([&](){
//             string proxy = UtilsGetNextProxy();
//             while (proxy != "")
//             {

//                 string result = Utils::ssystem("curl -x "+proxy+" --connect-timeout 5 -sS \"https://www.google.com\" --output \"/tmp/proxyCheckResult\"");
//                 if (result == "")
//                 {
//                     UtilsValidProxiesListLock.lock();
//                     Utils::validProxies.push_back(proxy);
//                     UtilsValidProxiesListLock.unlock();
//                 }

//                 proxy = UtilsGetNextProxy();
//             }
//         });

//         th->detach();
//     }

// }

// string Utils::downloadWithRandomProxy(string url, string destFileName, int maxTries)
// {
//     if (Utils::proxyListInitializedState == 'a')
//     {
//         Utils::initializeProxyList();
//     }

//     while (Utils::validProxies.size() == 0)
//         usleep(10000);

    
    
//     string result;
//     string randomProxy;

//     while (maxTries > 0)
//     {
//         randomProxy = Utils::validProxies[rand() % Utils::validProxies.size()];

//         result = Utils::ssystem("curl -x "+randomProxy+" -sS \""+url+"\" --connect-timeout 25 --output \""+destFileName+"\"");
//         if (result == "")
//         {
//             return result;
//         }
//         else
//             maxTries--;
//     }

//     return result;

// }

string Utils::stringReplace(string source, string replace, string by)
{
    stringstream ret;
    auto pos = source.find(replace);
    while (pos != string::npos)
    {
        ret << source.substr(0, pos) << by;
        source = source.substr(pos + replace.size());
        pos = source.find(replace);
    }
    ret << source;
    return ret.str();
}

string Utils::stringReplace(string source, vector<tuple<string, string>> replaceAndByTuples)
{
    for (auto &c: replaceAndByTuples)
        source = Utils::stringReplace(source, std::get<0>(c), std::get<1>(c));
    return source;
}

string Utils::stringReplaceMarker(string source, vector<string> by, string marker, bool use_TheArgBy_Circularly)
{
    stringstream ret;
    auto pos = source.find(marker);
    int index = 0;
    while (pos != string::npos && index < by.size())
    {
        auto currReplacer = by[index];
        ret << source.substr(0, pos) << currReplacer;
        source = source.substr(pos + marker.size());
        pos = source.find(marker);

        index++;
        if (use_TheArgBy_Circularly && index >= by.size())
            index = 0;
    }
    
    ret << source;
    return ret.str();
}

bool Utils::isNumber(string source)
{
    string validChars = "0123456789";
    bool alreadyDot = false;
    for (int c = 0; c < source.size(); c++)
    {
        char curr = source[c];
        if ((curr == '+' || curr == '-') && c != 0)
            return false;
        else if (curr == '.')
        {
            if (alreadyDot)
                return false;
            else
                alreadyDot = true;
        }
        else if (validChars.find(curr) == string::npos)
            return false;
    }

    return true;
}

//map<void*, string> Utils::getANameDB;
//https://dzone.com/articles/name-lists-generating-test
// string Utils::getAName(int number, NameType typeOfName, int AlgoGenMaxSyllables){
//     return getAName((void*)number, typeOfName, AlgoGenMaxSyllables);
// }

// string Utils::getAName(void* p, NameType typeOfName, int AlgoGenMaxSyllables){
//     vector<string> surnames_list={"Smith","Johnson","Williams","Brown","Jones","Miller","Davis","Garcia","Rodriguez","Wilson","Martinez","Anderson","Taylor","Thomas","Hernandez","Moore","Martin","Jackson","Thompson","White","Lopez","Lee","Gonzalez","Harris","Clark","Lewis","Robinson","Walker","Perez","Hall","Young","Allen","Sanchez","Wright","King","Scott","Green","Baker","Adams","Nelson","Hill","Ramirez","Campbell","Mitchell","Roberts","Carter","Phillips","Evans","Turner","Torres","Parker","Collins","Edwards","Stewart","Flores","Morris","Nguyen","Murphy","Rivera","Cook","Rogers","Morgan","Peterson","Cooper","Reed","Bailey","Bell","Gomez","Kelly","Howard","Ward","Cox","Diaz","Richardson","Wood","Watson","Brooks","Bennett","Gray","James","Reyes","Cruz","Hughes","Price","Myers","Long","Foster","Sanders","Ross","Morales","Powell","Sullivan","Russell","Ortiz","Jenkins","Gutierrez","Perry","Butler","Barnes","Fisher","Henderson","Coleman","Simmons","Patterson","Jordan","Reynolds","Hamilton","Graham","Kim","Gonzales","Alexander","Ramos","Wallace","Griffin","West","Cole","Hayes","Chavez","Gibson","Bryant","Ellis","Stevens","Murray","Ford","Marshall","Owens","Mcdonald","Harrison","Ruiz","Kennedy","Wells","Alvarez","Woods","Mendoza","Castillo","Olson","Webb","Washington","Tucker","Freeman","Burns","Henry","Vasquez","Snyder","Simpson","Crawford","Jimenez","Porter","Mason","Shaw","Gordon","Wagner","Hunter","Romero","Hicks","Dixon","Hunt","Palmer","Robertson","Black","Holmes","Stone","Meyer","Boyd","Mills","Warren","Fox","Rose","Rice","Moreno","Schmidt","Patel","Ferguson","Nichols","Herrera","Medina","Ryan","Fernandez","Weaver","Daniels","Stephens","Gardner","Payne","Kelley","Dunn","Pierce","Arnold","Tran","Spencer","Peters","Hawkins","Grant","Hansen","Castro","Hoffman","Hart","Elliott","Cunningham","Knight","Bradley","Carroll","Hudson","Duncan","Armstrong","Berry","Andrews","Johnston","Ray","Lane","Riley","Carpenter","Perkins","Aguilar","Silva","Richards","Willis","Matthews","Chapman","Lawrence","Garza","Vargas","Watkins","Wheeler","Larson","Carlson","Harper","George","Greene","Burke","Guzman","Morrison","Munoz","Jacobs","Obrien","Lawson","Franklin","Lynch","Bishop","Carr","Salazar","Austin","Mendez","Gilbert","Jensen","Williamson","Montgomery","Harvey","Oliver","Howell","Dean","Hanson","Weber","Garrett","Sims","Burton","Fuller","Soto","Mccoy","Welch","Chen","Schultz","Walters","Reid","Fields","Walsh","Little","Fowler","Bowman","Davidson","May","Day","Schneider","Newman","Brewer","Lucas","Holland","Wong","Banks","Santos","Curtis","Pearson","Delgado","Valdez","Pena","Rios","Douglas","Sandoval","Barrett","Hopkins","Keller","Guerrero","Stanley","Bates","Alvarado","Beck","Ortega","Wade","Estrada","Contreras","Barnett","Caldwell","Santiago","Lambert","Powers","Chambers","Nunez","Craig","Leonard","Lowe","Rhodes","Byrd","Gregory","Shelton","Frazier","Becker","Maldonado","Fleming","Vega","Sutton","Cohen","Jennings","Parks","Mcdaniel","Watts","Barker","Norris","Vaughn","Vazquez","Holt","Schwartz","Steele","Benson","Neal","Dominguez","Horton","Terry","Wolfe","Hale","Lyons","Graves","Haynes","Miles","Park","Warner","Padilla","Bush","Thornton","Mccarthy","Mann","Zimmerman","Erickson","Fletcher","Mckinney","Page","Dawson","Joseph","Marquez","Reeves","Klein","Espinoza","Baldwin","Moran","Love","Robbins","Higgins","Ball","Cortez","Le","Griffith","Bowen","Sharp","Cummings","Ramsey","Hardy","Swanson","Barber","Acosta","Luna","Chandler","Blair","Daniel","Cross","Simon","Dennis","Oconnor","Quinn","Gross","Navarro","Moss","Fitzgerald","Doyle","Mclaughlin","Rojas","Rodgers","Stevenson","Singh","Yang","Figueroa","Harmon","Newton","Paul","Manning","Garner","Mcgee","Reese","Francis","Burgess","Adkins","Goodman","Curry","Brady","Christensen","Potter","Walton","Goodwin","Mullins","Molina","Webster","Fischer","Campos","Avila","Sherman","Todd","Chang","Blake","Malone","Wolf","Hodges","Juarez","Gill","Farmer","Hines","Gallagher","Duran","Hubbard","Cannon","Miranda","Wang","Saunders","Tate","Mack","Hammond","Carrillo","Townsend","Wise","Ingram","Barton","Mejia","Ayala","Schroeder","Hampton","Rowe","Parsons","Frank","Waters","Strickland","Osborne","Maxwell","Chan","Deleon","Norman","Harrington","Casey","Patton","Logan","Bowers","Mueller","Glover","Floyd","Hartman","Buchanan","Cobb","French","Kramer","Mccormick","Clarke","Tyler","Gibbs","Moody","Conner","Sparks","Mcguire","Leon","Bauer","Norton","Pope","Flynn","Hogan","Robles","Salinas","Yates","Lindsey","Lloyd","Marsh","Mcbride","Owen","Solis","Pham","Lang","Pratt","Lara","Brock","Ballard","Trujillo","Shaffer","Drake","Roman","Aguirre","Morton","Stokes","Lamb","Pacheco","Patrick","Cochran","Shepherd","Cain","Burnett","Hess","Li","Cervantes","Olsen","Briggs","Ochoa","Cabrera","Velasquez","Montoya","Roth","Meyers","Cardenas","Fuentes","Weiss","Wilkins","Hoover","Nicholson","Underwood","Short","Carson","Morrow","Colon","Holloway","Summers","Bryan","Petersen","Mckenzie","Serrano","Wilcox","Carey","Clayton","Poole","Calderon","Gallegos","Greer","Rivas","Guerra","Decker","Collier","Wall","Whitaker","Bass","Flowers","Davenport","Conley","Houston","Huff","Copeland","Hood","Monroe","Massey","Roberson","Combs","Franco","Larsen","Pittman","Randall","Skinner","Wilkinson","Kirby","Cameron","Bridges","Anthony","Richard","Kirk","Bruce","Singleton","Mathis","Bradford","Boone","Abbott","Charles","Allison","Sweeney","Atkinson","Horn","Jefferson","Rosales","York","Christian","Phelps","Farrell","Castaneda","Nash","Dickerson","Bond","Wyatt","Foley","Chase","Gates","Vincent","Mathews","Hodge","Garrison","Trevino","Villarreal","Heath","Dalton","Valencia","Callahan","Hensley","Atkins","Huffman","Roy","Boyer","Shields","Lin","Hancock","Grimes","Glenn","Cline","Delacruz","Camacho","Dillon","Parrish","Oneill","Melton","Booth","Kane","Berg","Harrell","Pitts","Savage","Wiggins","Brennan","Salas","Marks","Russo","Sawyer","Baxter","Golden","Hutchinson","Liu","Walter","Mcdowell","Wiley","Rich","Humphrey","Johns","Koch","Suarez","Hobbs","Beard","Gilmore","Ibarra","Keith","Macias","Khan","Andrade","Ware","Stephenson","Henson","Wilkerson","Dyer","Mcclure","Blackwell","Mercado","Tanner","Eaton","Clay","Barron","Beasley","Oneal","Small","Preston","Wu","Zamora","Macdonald","Vance","Snow","Mcclain","Stafford","Orozco","Barry","English","Shannon","Kline","Jacobson","Woodard","Huang","Kemp","Mosley","Prince","Merritt","Hurst","Villanueva","Roach","Nolan","Lam","Yoder","Mccullough","Lester","Santana","Valenzuela","Winters","Barrera","Orr","Leach","Berger","Mckee","Strong","Conway","Stein","Whitehead","Bullock","Escobar","Knox","Meadows","Solomon","Velez","Odonnell","Kerr","Stout","Blankenship","Browning","Kent","Lozano","Bartlett","Pruitt","Buck","Barr","Gaines","Durham","Gentry","Mcintyre","Sloan","Rocha","Melendez","Herman","Sexton","Moon","Hendricks","Rangel","Stark","Lowery","Hardin","Hull","Sellers","Ellison","Calhoun","Gillespie","Mora","Knapp","Mccall","Morse","Dorsey","Weeks","Nielsen","Livingston","Leblanc","Mclean","Bradshaw","Glass","Middleton","Buckley","Schaefer","Frost","Howe","House","Mcintosh","Ho","Pennington","Reilly","Hebert","Mcfarland","Hickman","Noble","Spears","Conrad","Arias","Galvan","Velazquez","Huynh","Frederick","Randolph","Cantu","Fitzpatrick","Mahoney","Peck","Villa","Michael","Donovan","Mcconnell","Walls","Boyle","Mayer","Zuniga","Giles","Pineda","Pace","Hurley","Mays","Mcmillan","Crosby","Ayers","Case","Bentley","Shepard","Everett","Pugh","David","Mcmahon","Dunlap","Bender","Hahn","Harding","Acevedo","Raymond","Blackburn","Duffy","Landry","Dougherty","Bautista","Shah","Potts","Arroyo","Valentine","Meza","Gould","Vaughan","Fry","Rush","Avery","Herring","Dodson","Clements","Sampson","Tapia","Bean","Lynn","Crane","Farley","Cisneros","Benton","Ashley","Mckay","Finley","Best","Blevins","Friedman","Moses","Sosa","Blanchard","Huber","Frye","Krueger","Bernard","Rosario","Rubio","Mullen","Benjamin","Haley","Chung","Moyer","Choi","Horne","Yu","Woodward","Ali","Nixon","Hayden","Rivers","Estes","Mccarty","Richmond","Stuart","Maynard","Brandt","Oconnell","Hanna","Sanford","Sheppard","Church","Burch","Levy","Rasmussen","Coffey","Ponce","Faulkner","Donaldson","Schmitt","Novak","Costa","Montes","Booker","Cordova","Waller","Arellano","Maddox","Mata","Bonilla","Stanton","Compton","Kaufman","Dudley","Mcpherson","Beltran","Dickson","Mccann","Villegas","Proctor","Hester","Cantrell","Daugherty","Cherry","Bray","Davila","Rowland","Madden","Levine","Spence","Good","Irwin","Werner","Krause","Petty","Whitney","Baird","Hooper","Pollard","Zavala","Jarvis","Holden","Haas","Hendrix","Mcgrath","Bird","Lucero","Terrell","Riggs","Joyce","Mercer","Rollins","Galloway","Duke","Odom","Andersen","Downs","Hatfield","Benitez","Archer","Huerta","Travis","Mcneil","Hinton","Zhang","Hays","Mayo","Fritz","Branch","Mooney","Ewing","Ritter","Esparza","Frey","Braun","Gay","Riddle","Haney","Kaiser","Holder","Chaney","Mcknight","Gamble","Vang","Cooley","Carney","Cowan","Forbes","Ferrell","Davies","Barajas","Shea","Osborn","Bright","Cuevas","Bolton","Murillo","Lutz","Duarte","Kidd","Key","Cooke"};
//     vector<string> male_names_list={"James","John","Robert","Michael","William","David","Richard","Charles","Joseph","Thomas","Christopher","Daniel","Paul","Mark","Donald","George","Kenneth","Steven","Edward","Brian","Ronald","Anthony","Kevin","Jason","Matthew","Gary","Timothy","Jose","Larry","Jeffrey","Frank","Scott","Eric","Stephen","Andrew","Raymond","Gregory","Joshua","Jerry","Dennis","Walter","Patrick","Peter","Harold","Douglas","Henry","Carl","Arthur","Ryan","Roger","Joe","Juan","Jack","Albert","Jonathan","Justin","Terry","Gerald","Keith","Samuel","Willie","Ralph","Lawrence","Nicholas","Roy","Benjamin","Bruce","Brandon","Adam","Harry","Fred","Wayne","Billy","Steve","Louis","Jeremy","Aaron","Randy","Howard","Eugene","Carlos","Russell","Bobby","Victor","Martin","Ernest","Phillip","Todd","Jesse","Craig","Alan","Shawn","Clarence","Sean","Philip","Chris","Johnny","Earl","Jimmy","Antonio","Danny","Bryan","Tony","Luis","Mike","Stanley","Leonard","Nathan","Dale","Manuel","Rodney","Curtis","Norman","Allen","Marvin","Vincent","Glenn","Jeffery","Travis","Jeff","Chad","Jacob","Lee","Melvin","Alfred","Kyle","Francis","Bradley","Jesus","Herbert","Frederick","Ray","Joel","Edwin","Don","Eddie","Ricky","Troy","Randall","Barry","Alexander","Bernard","Mario","Leroy","Francisco","Marcus","Micheal","Theodore","Clifford","Miguel","Oscar","Jay","Jim","Tom","Calvin","Alex","Jon","Ronnie","Bill","Lloyd","Tommy","Leon","Derek","Warren","Darrell","Jerome","Floyd","Leo","Alvin","Tim","Wesley","Gordon","Dean","Greg","Jorge","Dustin","Pedro","Derrick","Dan","Lewis","Zachary","Corey","Herman","Maurice","Vernon","Roberto","Clyde","Glen","Hector","Shane","Ricardo","Sam","Rick","Lester","Brent","Ramon","Charlie","Tyler","Gilbert","Gene","Marc","Reginald","Ruben","Brett","Angel","Nathaniel","Rafael","Leslie","Edgar","Milton","Raul","Ben","Chester","Cecil","Duane","Franklin","Andre","Elmer","Brad","Gabriel","Ron","Mitchell","Roland","Arnold","Harvey","Jared","Adrian","Karl","Cory","Claude","Erik","Darryl","Jamie","Neil","Jessie","Christian","Javier","Fernando","Clinton","Ted","Mathew","Tyrone","Darren","Lonnie","Lance","Cody","Julio","Kelly","Kurt","Allan","Nelson","Guy","Clayton","Hugh","Max","Dwayne","Dwight","Armando","Felix","Jimmie","Everett","Jordan","Ian","Wallace","Ken","Bob","Jaime","Casey","Alfredo","Alberto","Dave","Ivan","Johnnie","Sidney","Byron","Julian","Isaac","Morris","Clifton","Willard","Daryl","Ross","Virgil","Andy","Marshall","Salvador","Perry","Kirk","Sergio","Marion","Tracy","Seth","Kent","Terrance","Rene","Eduardo","Terrence","Enrique","Freddie","Wade"};
//     vector<string> female_names_list={"Mary","Patricia","Linda","Barbara","Elizabeth","Jennifer","Maria","Susan","Margaret","Dorothy","Lisa","Nancy","Karen","Betty","Helen","Sandra","Donna","Carol","Ruth","Sharon","Michelle","Laura","Sarah","Kimberly","Deborah","Jessica","Shirley","Cynthia","Angela","Melissa","Brenda","Amy","Anna","Rebecca","Virginia","Kathleen","Pamela","Martha","Debra","Amanda","Stephanie","Carolyn","Christine","Marie","Janet","Catherine","Frances","Ann","Joyce","Diane","Alice","Julie","Heather","Teresa","Doris","Gloria","Evelyn","Jean","Cheryl","Mildred","Katherine","Joan","Ashley","Judith","Rose","Janice","Kelly","Nicole","Judy","Christina","Kathy","Theresa","Beverly","Denise","Tammy","Irene","Jane","Lori","Rachel","Marilyn","Andrea","Kathryn","Louise","Sara","Anne","Jacqueline","Wanda","Bonnie","Julia","Ruby","Lois","Tina","Phyllis","Norma","Paula","Diana","Annie","Lillian","Emily","Robin","Peggy","Crystal","Gladys","Rita","Dawn","Connie","Florence","Tracy","Edna","Tiffany","Carmen","Rosa","Cindy","Grace","Wendy","Victoria","Edith","Kim","Sherry","Sylvia","Josephine","Thelma","Shannon","Sheila","Ethel","Ellen","Elaine","Marjorie","Carrie","Charlotte","Monica","Esther","Pauline","Emma","Juanita","Anita","Rhonda","Hazel","Amber","Eva","Debbie","April","Leslie","Clara","Lucille","Jamie","Joanne","Eleanor","Valerie","Danielle","Megan","Alicia","Suzanne","Michele","Gail","Bertha","Darlene","Veronica","Jill","Erin","Geraldine","Lauren","Cathy","Joann","Lorraine","Lynn","Sally","Regina","Erica","Beatrice","Dolores","Bernice","Audrey","Yvonne","Annette","June","Samantha","Marion","Dana","Stacy","Ana","Renee","Ida","Vivian","Roberta","Holly","Brittany","Melanie","Loretta","Yolanda","Jeanette","Laurie","Katie","Kristen","Vanessa","Alma","Sue","Elsie","Beth","Jeanne","Vicki","Carla","Tara","Rosemary","Eileen","Terri","Gertrude","Lucy","Tonya","Ella","Stacey","Wilma","Gina","Kristin","Jessie","Natalie","Agnes","Vera","Willie","Charlene","Bessie","Delores","Melinda","Pearl","Arlene","Maureen","Colleen","Allison","Tamara","Joy","Georgia","Constance","Lillie","Claudia","Jackie","Marcia","Tanya","Nellie","Minnie","Marlene","Heidi","Glenda","Lydia","Viola","Courtney","Marian","Stella","Caroline","Dora","Jo","Vickie","Mattie","Terry","Maxine","Irma","Mabel","Marsha","Myrtle","Lena","Christy","Deanna","Patsy","Hilda","Gwendolyn","Jennie","Nora","Margie","Nina","Cassandra","Leah","Penny","Kay","Priscilla","Naomi","Carole","Brandy","Olga","Billie","Dianne","Tracey","Leona","Jenny","Felicia","Sonia","Miriam","Velma","Becky","Bobbie","Violet","Kristina","Toni","Misty","Mae","Shelly","Daisy","Ramona","Sherri","Erika","Katrina","Claire","Lindsey","Lindsay","Geneva","Guadalupe","Belinda","Margarita","Sheryl","Cora","Faye","Ada","Natasha","Sabrina","Isabel","Marguerite","Hattie","Harriet","Molly","Cecilia","Kristi","Brandi","Blanche","Sandy","Rosie","Joanna","Iris","Eunice","Angie","Inez","Lynda","Madeline","Amelia","Alberta","Genevieve","Monique","Jodi","Janie","Maggie","Kayla","Sonya","Jan","Lee","Kristine","Candace","Fannie","Maryann","Opal","Alison","Yvette","Melody","Luz","Susie","Olivia","Flora","Shelley","Kristy","Mamie","Lula","Lola","Verna","Beulah","Antoinette","Candice","Juana","Jeannette","Pam","Kelli","Hannah","Whitney","Bridget","Karla","Celia","Latoya","Patty","Shelia","Gayle","Della","Vicky","Lynne","Sheri","Marianne","Kara","Jacquelyn","Erma","Blanca","Myra","Leticia","Pat","Krista","Roxanne","Angelica","Johnnie","Robyn","Francis","Adrienne","Rosalie","Alexandra","Brooke","Bethany","Sadie","Bernadette","Traci","Jody","Kendra","Jasmine","Nichole","Rachael","Chelsea","Mable","Ernestine","Muriel","Marcella","Elena","Krystal","Angelina","Nadine","Kari","Estelle","Dianna","Paulette","Lora","Mona","Doreen","Rosemarie","Angel","Desiree","Antonia","Hope","Ginger","Janis","Betsy","Christie","Freda","Mercedes","Meredith","Lynette","Teri","Cristina","Eula","Leigh","Meghan","Sophia","Eloise","Rochelle","Gretchen","Cecelia","Raquel","Henrietta","Alyssa","Jana","Kelley","Gwen","Kerry","Jenna","Tricia","Laverne","Olive","Alexis","Tasha","Silvia","Elvira","Casey","Delia","Sophie","Kate","Patti","Lorena","Kellie","Sonja","Lila","Lana","Darla","May","Mindy","Essie","Mandy","Lorene","Elsa","Josefina","Jeannie","Miranda","Dixie","Lucia","Marta","Faith","Lela","Johanna","Shari","Camille","Tami","Shawna","Elisa","Ebony","Melba","Ora","Nettie","Tabitha","Ollie","Jaime","Winifred","Kristie","Marina","Alisha","Aimee","Rena","Myrna","Marla","Tammie","Latasha","Bonita","Patrice","Ronda","Sherrie","Addie","Francine","Deloris","Stacie","Adriana","Cheri","Shelby","Abigail","Celeste","Jewel","Cara","Adele","Rebekah","Lucinda","Dorthy","Chris","Effie","Trina","Reba","Shawn","Sallie","Aurora","Lenora","Etta","Lottie","Kerri","Trisha","Nikki","Estella","Francisca","Josie","Tracie","Marissa","Karin","Brittney","Janelle","Lourdes","Laurel","Helene","Fern","Elva","Corinne","Kelsey","Ina","Bettie","Elisabeth","Aida","Caitlin","Ingrid","Iva","Eugenia","Christa","Goldie","Cassie","Maude","Jenifer","Therese","Frankie","Dena","Lorna","Janette","Latonya","Candy","Morgan","Consuelo","Tamika","Rosetta","Debora","Cherie","Polly","Dina","Jewell","Fay","Jillian","Dorothea","Nell","Trudy","Esperanza","Patrica","Kimberley","Shanna","Helena","Carolina","Cleo","Stefanie","Rosario","Ola","Janine","Mollie","Lupe","Alisa","Lou","Maribel","Susanne","Bette","Susana","Elise","Cecile","Isabelle","Lesley","Jocelyn","Paige","Joni","Rachelle","Leola","Daphne","Alta","Ester","Petra","Graciela","Imogene","Jolene","Keisha","Lacey","Glenna","Gabriela","Keri","Ursula","Lizzie","Kirsten","Shana","Adeline","Mayra","Jayne","Jaclyn","Gracie","Sondra","Carmela","Marisa","Rosalind","Charity","Tonia","Beatriz","Marisol","Clarice","Jeanine","Sheena","Angeline","Frieda","Lily","Robbie","Shauna","Millie","Claudette","Cathleen","Angelia","Gabrielle","Autumn","Katharine","Summer","Jodie","Staci","Lea","Christi","Jimmie","Justine","Elma","Luella","Margret","Dominique","Socorro","Rene","Martina","Margo","Mavis","Callie","Bobbi","Maritza","Lucile","Leanne","Jeannine","Deana","Aileen","Lorie","Ladonna","Willa","Manuela","Gale","Selma","Dolly","Sybil","Abby","Lara","Dale","Ivy","Dee","Winnie","Marcy","Luisa","Jeri","Magdalena","Ofelia","Meagan","Audra","Matilda","Leila","Cornelia","Bianca","Simone","Bettye","Randi","Virgie","Latisha","Barbra","Georgina","Eliza","Leann","Bridgette","Rhoda","Haley","Adela","Nola","Bernadine","Flossie","Ila","Greta","Ruthie","Nelda","Minerva","Lilly","Terrie","Letha","Hilary","Estela","Valarie","Brianna","Rosalyn","Earline","Catalina","Ava","Mia","Clarissa","Lidia","Corrine","Alexandria","Concepcion","Tia","Sharron","Rae","Dona","Ericka","Jami","Elnora","Chandra","Lenore","Neva","Marylou","Melisa","Tabatha","Serena","Avis","Allie","Sofia","Jeanie","Odessa","Nannie","Harriett","Loraine","Penelope","Milagros","Emilia","Benita","Allyson","Ashlee","Tania","Tommie","Esmeralda","Karina","Eve","Pearlie","Zelma","Malinda","Noreen","Tameka","Saundra","Hillary","Amie","Althea","Rosalinda","Jordan","Lilia","Alana","Gay","Clare","Alejandra","Elinor","Michael","Lorrie","Jerri","Darcy","Earnestine","Carmella","Taylor","Noemi","Marcie","Liza","Annabelle","Louisa","Earlene","Mallory","Carlene","Nita","Selena","Tanisha","Katy","Julianne","John","Lakisha","Edwina","Maricela","Margery","Kenya","Dollie","Roxie","Roslyn","Kathrine","Nanette","Charmaine","Lavonne","Ilene","Kris","Tammi","Suzette","Corine","Kaye","Jerry","Merle","Chrystal","Lina","Deanne","Lilian","Juliana","Aline","Luann","Kasey","Maryanne","Evangeline","Colette","Melva","Lawanda","Yesenia","Nadia","Madge","Kathie","Eddie","Ophelia","Valeria","Nona","Mitzi","Mari","Georgette","Claudine","Fran","Alissa","Roseann","Lakeisha","Susanna","Reva","Deidre","Chasity","Sheree","Carly","James","Elvia","Alyce","Deirdre","Gena","Briana","Araceli","Katelyn","Rosanne","Wendi","Tessa","Berta","Marva","Imelda","Marietta","Marci","Leonor","Arline","Sasha","Madelyn","Janna","Juliette","Deena","Aurelia","Josefa","Augusta","Liliana","Young","Christian","Lessie","Amalia","Savannah","Anastasia","Vilma","Natalia","Rosella","Lynnette","Corina","Alfreda","Leanna","Carey","Amparo","Coleen","Tamra","Aisha","Wilda","Karyn","Cherry","Queen","Maura","Mai","Evangelina","Rosanna","Hallie","Erna","Enid","Mariana","Lacy","Juliet","Jacklyn","Freida","Madeleine","Mara","Hester","Cathryn","Lelia","Casandra","Bridgett","Angelita","Jannie","Dionne","Annmarie","Katina","Beryl","Phoebe","Millicent","Katheryn","Diann","Carissa","Maryellen","Liz","Lauri","Helga","Gilda","Adrian","Rhea","Marquita","Hollie","Tisha","Tamera","Angelique","Francesca","Britney","Kaitlin","Lolita","Florine","Rowena","Reyna","Twila","Fanny","Janell","Ines","Concetta","Bertie","Alba","Brigitte","Alyson","Vonda","Pansy","Elba","Noelle","Letitia","Kitty","Deann","Brandie","Louella","Leta","Felecia","Sharlene","Lesa","Beverley","Robert","Isabella","Herminia","Terra","Celina"};
//     if (!getANameDB.count(p))
//     {
//         string name = "";
//         runSRand();

//         if (typeOfName == ALGORITHM_GENERATED)
//         {
//             string consonants = "bcdfghjklmnpqrstvxwz";
//             string vocals = "aeiouy";


//             while (AlgoGenMaxSyllables-- > 0)
//                 name += consonants.substr(rand() % consonants.size(), 1) + "" + vocals.substr(rand() % vocals.size(), 1);
//         }
//         else
//         {   
            

//             if (rand() % 1 == 1)
//                 name = male_names_list[rand() % male_names_list.size()];
//             else
//                 name = female_names_list[rand() % female_names_list.size()];

//             name += " "+surnames_list[rand() % surnames_list.size()];


//         }

//         getANameDB[p] = name;
//     }

//     return getANameDB[p];
// }

string Utils::getNestedExceptionText(exception &e, string prefix, int level)
{

    string ret;
    for (int c = 0; c < level; c++)
        ret += " ";
    //ret += "-> ";
    ret += prefix + e.what();

    try{
        rethrow_if_nested(e);
    }
    catch(exception &e2)
    {
        ret += "\n" + getNestedExceptionText(e2, prefix, level+1);
    }

    return ret;
}

string Utils::ltrim(string s) {
    s.erase(s.begin(), std::find_if(s.begin(), s.end(), [](unsigned char ch) {
        return !std::isspace(ch);
    }));

    return s;
}

// trim from end (in place)
string Utils::rtrim(string s) {
    s.erase(std::find_if(s.rbegin(), s.rend(), [](unsigned char ch) {
        return !std::isspace(ch);
    }).base(), s.end());

    return s;
}

// trim from both ends (in place)
string Utils::trim(std::string s) {
    return rtrim(ltrim(s));
}

string Utils::escapeString(string source)
{
    string ret = source;
    ret = Utils::stringReplace(ret, {
        {"\\", "\\\\"},
        {"\"", "\\\""},
        {"\n", "\\n"},
        {"\r", "\\r"},
        {"\t", "\\t"}
    });

    return ret;
}

string Utils::unescapeString(string source)
{
    string ret = source;
    ret = Utils::stringReplace(ret, {
        {"\\n", "\n"},
        {"\\r", "\r"},
        {"\\t", "\t"},
        {"\\\"", "\""},
        {"\\\\", "\\"}
    });

    return ret;
}