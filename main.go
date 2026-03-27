//important: this project is a rewrite of the project inside cpp_version folder

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"rtonello/vss/sources/controller"
	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/services/apis/vstp"
	"rtonello/vss/sources/services/storage"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"rtonello/vss/sources/misc/logger"
	logwriters "rtonello/vss/sources/misc/logger/writers"

	"rtonello/vss/sources/misc/confs"
	"rtonello/vss/sources/misc/confs/sources"
	httpapi "rtonello/vss/sources/services/apis/httpapi"
)

const INFO_VERSION = "2.0.0+Capella"

func main() {

	if displayHelpOrVersion() {
		return
	}

	//#region configs and logger {
	fmt.Print("Loading configurations...")
	configs := initConfigurations()
	fmt.Println("done.")

	fmt.Print("Initializing logger...")
	logManager := initLogger(configs)
	fmt.Println("done.")
	//#endregion }

	if detectInvalidArguments(configs, logManager) {
		return
	}

	//setupStdoutAndStderrInterception(logManager)

	//#region Initial banner and runtime informations {
	printBanner(logManager, configs)
	//#endregion }

	//#region remain services {
	mainLog := logManager.GetNamedLogger("Main")

	mainLog.KeepNextLineOpened()
	mainLog.Info("Starting storage ...")
	theStorage := initStorage(configs, logManager)
	mainLog.Info("...Storage started.")

	mainLog.Info("Starting controller ...")
	theController := controller.NewController(logManager, configs, theStorage, INFO_VERSION)
	mainLog.Info("...Controller started.")

	mainLog.Info("Starting VSTP API ...")
	vstpPort := configs.GetConfig("vstpApiPort").Value().GetInt()
	vstpApi, err := vstp.NewVSTP(vstpPort, theController, logManager)
	if err != nil {
		mainLog.Error("Main", "Failed to start VSTP API: "+err.Error())
		// exit early on failure
		return
	}
	_ = vstpApi
	mainLog.Info("...VSTP API started.")

	mainLog.Info("Starting HTTP API ...")
	httpApiPort := configs.GetConfig("httpApiPort").Value()
	httpApi, err := httpapi.New(httpApiPort.GetInt(), theController)
	if err != nil {
		mainLog.Error("Main", "Failed to start HTTP API: "+err.Error())
		// exit early on failure
		return
	} else {
		mainLog.Info("...HTTP API started.")
	}
	_ = httpApi
	//#endregion }

	//#endregion }

	if configs.GetConfig("allowRawDbAccess").Value().GetBool() {
		//print a yellow message to the terminal

		//give some time to previous log messages to be printed
		time.Sleep(500 * time.Millisecond)

		fmt.Print("\n\n\033[33m")
		misc.PrintTerminalSeparator("-", " [ RAW DATABASE ACCESS NOTICE ] ")
		os.Stdout.WriteString("\033[0m")
		os.Stdout.WriteString("\033[33m Warning: Raw database access is enabled.\033[0m\n")
		os.Stdout.WriteString("\033[33m Warning:This may pose security risks if not properly managed.\033[0m\n")
		os.Stdout.WriteString("\033[33m Warning: You should only use it for database maintenance and if you know the internal structures of VSS.\033[0m\n")
		os.Stdout.WriteString("\033[33m Warning:Clients should not connect to the database when this option is enabled.\033[0m\n")
		fmt.Print("\033[33m")
		misc.PrintTerminalSeparator("-", " [ RAW DATABASE ACCESS NOTICE ] ")
		os.Stdout.WriteString("\033[0m\n\n")
	}

	//handle system signals to allow graceful shutdown
	sigs := make(chan os.Signal, 1)
	setupSignalHandler(sigs)

	sig := <-sigs
	mainLog.Info("Received the signal '" + sig.String() + "', shutting down...")

	//finalize services in reverse order
	//vstp.Finalize()
	//httpapi.Finalize()
	theStorage.Finalize()

	mainLog.Info("Vss gracefully shut down. Bye!")

	//wait one second to logger flush all messages
	time.Sleep(1 * time.Second)
	os.Exit(0)
}

func printHelpLine(line string, terminalSize int) {
	//find the last sequence of two spaces ('  ')
	index := strings.LastIndex(line, "  ")
	//if not found, just print the line
	if index == -1 || len(line) < terminalSize {
		fmt.Println(line)
	} else {
		index += 2
		//the current line is printed until it reaches the terminal size, after this, new lines
		//will be printed using a 'identation' of 'index' + 1 spaces. This shift of 1 space let the
		//text visually connected to the first line.
		//The next lines also respect the terminalSize

		for {
			if len(line) < terminalSize {
				fmt.Println(line)
				break
			}

			toPrint := line[:terminalSize]
			//look in the 20 previus charses to find the last space, to avoid breaking words. If not found, just break at terminalSize
			lastSpace := strings.LastIndex(toPrint, " ")
			if lastSpace != -1 && lastSpace > terminalSize-20 {
				toPrint = line[:lastSpace+1]
			}

			//remove the 'toPrint' from line
			line = line[len(toPrint):]
			if len(line) == 0 {
				fmt.Println(toPrint)
				break
			}
			fmt.Println(toPrint)

			//add identation to remaining text
			identation := strings.Repeat(" ", index+1)
			line = identation + line
		}
	}

}

func getTerminalSize() int {
	const defaultTerminalSize = 80

	// Try common terminal streams first.
	fds := []int{int(os.Stdout.Fd()), int(os.Stderr.Fd()), int(os.Stdin.Fd())}
	for _, fd := range fds {
		if fd < 0 {
			continue
		}

		width, _, err := term.GetSize(fd)
		if err == nil && width > 0 {
			return width
		}
	}

	// Fallback for non-interactive contexts where COLUMNS is exported.
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if parsed, err := strconv.Atoi(cols); err == nil && parsed > 0 {
			return parsed
		}
	}

	return defaultTerminalSize
}

func displayHelpOrVersion() bool {
	//get terminal size
	terminalSize := getTerminalSize()

	for _, arg := range os.Args {
		switch arg {
		case "--help", "-h", "/?", "-?":
			printHelpLine("VarServerSHU - The variable server for SHU-based systems", terminalSize)
			printHelpLine("Version: "+INFO_VERSION, terminalSize)
			printHelpLine("", terminalSize)

			misc.PrintWithColor("Usage: vss [options]\n", misc.TerminalColorGreen)
			printHelpLine("Options:", terminalSize)
			printHelpLine("  -h, --help, /?, -?, help          Show this help message and exit", terminalSize)
			printHelpLine("", terminalSize)
			printHelpLine("  --version                         Show version information and exit", terminalSize)
			printHelpLine("  --http-api-port <port>            Set the HTTP API port (default: 5024 or value in conf file)", terminalSize)
			printHelpLine("  --http-api-https-port <port>      Set the HTTPS API port (default: 5025 or value in conf file)", terminalSize)
			printHelpLine("  --http-data-folder                Set the HTTP data directory (default: /var/vss/data/http_data or value in conf file)", terminalSize)
			printHelpLine("  --http-api-cert-file              Set the HTTPS certificate file (default: ./ssl/cert/vssCert.pem or value in conf file)", terminalSize)
			printHelpLine("  --http-api-key-file               Set the HTTPS key file (default: ./ssl/cert/vssKey.pem or value in conf file)", terminalSize)
			printHelpLine("  --http-api-returns-full-paths     If true, HTTP API will return entire variables paths in the JSON results (default: false or value in conf file)", terminalSize)
			printHelpLine("  --http-data-directory <path>      Set the HTTP data directory (default: /var/vss/data/http_data or value in conf file)", terminalSize)
			printHelpLine("  --ram-cache-db-dump-interval-ms   Set the interval, in milliseconds, to RamCacheDB service check for changes in the memory and dump data to disk (default: 60000 or value in conf file)", terminalSize)
			printHelpLine("  --vstp-api-port <port>            Set the VSTP API port (default: 5032 or value in conf file)", terminalSize)
			printHelpLine("  --db-driver <driver_name>         Set the database driver (default: ramcacheddb or value in conf file). Available drivers: ramcacheddb, ramcacheddbpkv", terminalSize)
			printHelpLine("    driver names:", terminalSize)
			printHelpLine("      ramcacheddb                     An in-memory database with periodic dumps to disk. Uses a custom tree structure to store variables and their hierarchy. Data is stored in a single file on disk. This is the default driver.", terminalSize)
			printHelpLine("      ramcacheddbpkv                  Similar to ramcacheddb but uses a prefix tree (key-value store) for storage. This may provide better performance for certain workloads and allows more efficient storage of hierarchical keys. Data is stored in a single file on disk.", terminalSize)
			printHelpLine("  --db-path <path>                  Set the database path (default: /var/vss/data/database or value in conf file). Db driver dices if it wll be a file or a directory based on the presence of an extension in the provided path.", terminalSize)
			printHelpLine("  --max-log-file-size <size_in_bytes>", terminalSize)
			printHelpLine("                                    Set the maximum log file size in bytes before rotation (default: 52428800 or value in conf file)", terminalSize)
			printHelpLine("  --max-time-waiting-for-clients <seconds>", terminalSize)
			printHelpLine("                                    Set the maximum time in seconds to consider a client disconnected (default: 43200 or value in conf file)", terminalSize)
			printHelpLine("  --stdout-log-level <level>        Set the log level for console output (default: info or value in conf file)", terminalSize)
			printHelpLine("    levels:", terminalSize)
			printHelpLine("      trace                           Very detailed logs, used for debugging", terminalSize)
			printHelpLine("      debug                           Detailed logs, used for debugging", terminalSize)
			printHelpLine("      info                            General information logs", terminalSize)
			printHelpLine("      warning                         Warnings about potential issues", terminalSize)
			printHelpLine("      error                           Errors that occurred", terminalSize)
			printHelpLine("      critical                        Critical errors that may cause shutdown", terminalSize)
			printHelpLine("  --file-log-level <level>          Set the log level for file output (default: info or value in conf file)", terminalSize)
			printHelpLine("  --max-key-length <length>         Set the maximum key length in characters (default: 255 or value in conf file)", terminalSize)
			printHelpLine("  --max-key-word-length <length>    Set the maximum key word length in characters (default: 64 or value in conf file)", terminalSize)
			printHelpLine("  --max-value-size <size_in_bytes>  Set the maximum value size in bytes (default: 1048576 or value in conf file)", terminalSize)
			printHelpLine("  --allow-raw-db-access <true|false>", terminalSize)
			printHelpLine("                                    If true, allows raw database access without 'vars.' prefix (default: false or value in conf file)", terminalSize)
			printHelpLine("", terminalSize)

			misc.PrintWithColor("Environment Variables:\n", misc.TerminalColorGreen)
			printHelpLine("  VSS_HTTP_API_PORT                 Same as --http-api-port", terminalSize)
			printHelpLine("  VSS_HTTP_API_HTTPS_PORT           Same as --http-api-https-port", terminalSize)
			printHelpLine("  VSS_VSTP_API_PORT                 Same as --vstp-api-port", terminalSize)
			printHelpLine("  VSS_HTTP_DATA_FOLDER              Same as --http-data-folder", terminalSize)
			printHelpLine("  VSS_HTTP_API_CERT_FILE            Same as --http-api-cert-file", terminalSize)
			printHelpLine("  VSS_HTTP_API_KEY_FILE             Same as --http-api-key-file", terminalSize)
			printHelpLine("  VSS_HTTP_API_RETURN_FULL_PATHS    Same as --http-api-return-full-paths", terminalSize)
			printHelpLine("  VSS_RAM_CACHE_DB_DUMP_INTERVAL_MS", terminalSize)
			printHelpLine("                                    Same as --ram-cache-db-dump-interval-ms", terminalSize)
			printHelpLine("  VSS_VSTP_API_PORT                 Same as --vstp-api-port", terminalSize)
			printHelpLine("  VSS_DB_DRIVER                     Same as --db-driver", terminalSize)
			printHelpLine("  VSS_DB_PATH                       Same as --db-path", terminalSize)
			printHelpLine("  VSS_HTTP_DATA_DIRECTORY           Same as --http-data-directory", terminalSize)
			printHelpLine("  VSS_MAX_LOG_FILE_SIZE             Same as --max-log-file-size", terminalSize)
			printHelpLine("  VSS_MAX_TIME_WAITING_CLIENTS      Same as --max-time-waiting-clients", terminalSize)
			printHelpLine("  VSS_STDOUT_LOG_LEVEL              Same as --stdout-log-level", terminalSize)
			printHelpLine("  VSS_FILE_LOG_LEVEL                Same as --file-log-level", terminalSize)
			printHelpLine("  VSS_MAX_KEY_LENGTH                Same as --max-key-length", terminalSize)
			printHelpLine("  VSS_MAX_KEY_WORD_LENGTH           Same as --max-key-word-length", terminalSize)
			printHelpLine("  VSS_MAX_VALUE_SIZE                Same as --max-value-size", terminalSize)
			printHelpLine("  VSS_ALLOW_RAW_DB_ACCESS           Same as --allow-raw-db-access", terminalSize)
			printHelpLine("", terminalSize)
			misc.PrintWithColor("Command line arguments are more important than environment variables, which are more important than configuration file values.\n", misc.TerminalColorYellow)
			misc.PrintWithColor("For more information, visit the documentation.\n", misc.TerminalColorYellow)
			return true
		case "--version", "-v", "version":
			fmt.Println("VSS Version: " + INFO_VERSION)
			return true
		}
	}
	return false
}

func detectInvalidArguments(confs confs.IConfs, logManager logger.ILogger) bool {
	ret := false
	for i, arg := range os.Args {
		if i == 0 {
			continue
		}

		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			arg = parts[0]
		} else if strings.Contains(arg, ":") {
			parts := strings.SplitN(arg, ":", 2)
			arg = parts[0]
		}

		if _, found := confs.FindConfByParamName(arg); !found {
			logManager.Error("", "Unknown argument: "+arg)
			ret = true
		}
	}
	// Add more validation logic as needed
	return ret
}

func setupSignalHandler(sigs chan os.Signal) {
	// Register for common termination signals. The main goroutine is waiting
	// on the provided channel, so just forward OS signals into it.
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	// Optionally ignore SIGPIPE to avoid the process being killed when writing
	// to closed sockets on some platforms.
	signal.Ignore(syscall.SIGPIPE)
}

func printBanner(logger logger.ILogger, configs confs.IConfs) {

	text := "" + "The VSS has been started\n"
	text += "" + "+-- Version: " + INFO_VERSION + "\n"
	text += "" + "+-- Portable mode: "
	if runningInPortableMode() {
		text += "Yes" + "\n"
	} else {
		text += "No" + "\n"
	}

	text += "" + "|   +-- conf file: " + findConfigurationFile() + "\n"

	text += "" + "|   +-- log file: " + determineLogFile() + "\n"

	tmp := configs.GetConfig("DbPath").Value()
	text += "" + "|   +-- database folder: " + tmp.GetString() + "\n"
	text += "" + "+-- Services" + "\n"

	tmp = configs.GetConfig("vstpApiPort").Value()
	text += "" + "|   +-- VSTP port: TCP/" + tmp.GetString() + "\n"

	tmp = configs.GetConfig("httpApiPort").Value()
	tmp2 := configs.GetConfig("httpApiHttpsPort").Value()
	text += "" + "|   +-- HTTP port: TCP/" + tmp.GetString() + "(http) + TCP/" + tmp2.GetString() + "(https)" + "\n"

	tmp = configs.GetConfig("serverDiscoveryPort").Value()
	text += "" + "|   +-- Server discovery port: UDP/" + tmp.GetString() + "\n"
	text += "" + "+-- All configurations" + "\n"

	for key, item := range configs.AllConfigs() {
		tmp = item.NotMappedValue()
		text += "" + "    +-- " + key + ": " + tmp.GetString() + "\n"
	}

	logger.Info("", text)
}

func initConfigurations() confs.IConfs {
	theConfs := confs.NewConfs(
		//add configuration sources here
		[]confs.IConfsSource{
			//command line arguments
			sources.NewCommandLineSource(os.Args[1:]),

			//environment variables
			sources.NewEnvSource(),

			//configuration file
			sources.NewFileSource(findConfigurationFile()),
		},
	)

	theConfs.AddPlaceHolders(map[string]string{
		"%PROJECT_DIR%":         getApplicationDirectory(false),
		"%APP_DIR%":             getApplicationDirectory(false),
		"%SUGGESTED_DATA_PATH%": suggestDataDirectory(),
	})

	//#region maxLogFileSize {
	if conf, err := theConfs.CreateConfig("maxLogFileSize",
		confs.WithPossibleNames([]string{"max-log-file-size", "--max-log-file-size", "VSS_MAX_LOG_FILE_SIZE"}),
		confs.WithDefaultValue(misc.NewDynamicVar("52428800")), //50 MiB
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			//should be a positive integer
			value := conf.Value()
			_, err := value.GetInt64e()
			if err != nil {
				return fmt.Errorf("received '%s', must be a positive integer: %w", value.GetString(), err)
			}
			return nil
		}),
	); err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (52428800).")
	}

	//#endregion }

	//#region fileLogLevel {
	if conf, err := theConfs.CreateConfig("fileLogLevel",
		confs.WithPossibleNames([]string{"file-log-level", "--file-log-level", "VSS_FILE_LOG_LEVEL"}),
		confs.WithDefaultValue(misc.NewDynamicVar("info2")),
		confs.WithValueMap(map[misc.DynamicVar]misc.DynamicVar{
			misc.NewDynamicVar("trace"):    misc.NewDynamicVar(logger.LEVEL_TRACE),
			misc.NewDynamicVar("debug2"):   misc.NewDynamicVar(logger.LEVEL_DEBUG2),
			misc.NewDynamicVar("debug"):    misc.NewDynamicVar(logger.LEVEL_DEBUG),
			misc.NewDynamicVar("info2"):    misc.NewDynamicVar(logger.LEVEL_INFO2),
			misc.NewDynamicVar("info"):     misc.NewDynamicVar(logger.LEVEL_INFO),
			misc.NewDynamicVar("warning"):  misc.NewDynamicVar(logger.LEVEL_WARNING),
			misc.NewDynamicVar("error"):    misc.NewDynamicVar(logger.LEVEL_ERROR),
			misc.NewDynamicVar("critical"): misc.NewDynamicVar(logger.LEVEL_CRITICAL),
		}),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			valueStr := strings.ToLower(value.GetString())
			if !(strings.Contains("tracedebuginfoinfo2warningerrorcritical", valueStr)) {
				return fmt.Errorf("received '%s', must be a valid log level (trace, debug, info, info, warning, error or critical)", valueStr)
			}
			return nil
		}),
	); err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), " System will continue with default value (info).")
	}
	//#endregion }

	//#region stdoutLogLevel {
	if conf, err := theConfs.CreateConfig("stdoutLogLevel",
		confs.WithPossibleNames([]string{"stdout-log-level", "--stdout-log-level", "VSS_STDOUT_LOG_LEVEL"}),
		confs.WithDefaultValue(misc.NewDynamicVar("info")),
		confs.WithValueMap(map[misc.DynamicVar]misc.DynamicVar{
			misc.NewDynamicVar("trace"):    misc.NewDynamicVar(logger.LEVEL_TRACE),
			misc.NewDynamicVar("debug2"):   misc.NewDynamicVar(logger.LEVEL_DEBUG2),
			misc.NewDynamicVar("debug"):    misc.NewDynamicVar(logger.LEVEL_DEBUG),
			misc.NewDynamicVar("info2"):    misc.NewDynamicVar(logger.LEVEL_INFO2),
			misc.NewDynamicVar("info"):     misc.NewDynamicVar(logger.LEVEL_INFO),
			misc.NewDynamicVar("warning"):  misc.NewDynamicVar(logger.LEVEL_WARNING),
			misc.NewDynamicVar("error"):    misc.NewDynamicVar(logger.LEVEL_ERROR),
			misc.NewDynamicVar("critical"): misc.NewDynamicVar(logger.LEVEL_CRITICAL),
		}),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			valueStr := strings.ToLower(value.GetString())
			if !(strings.Contains("tracedebuginfoinfo2warningerrorcritical", valueStr)) {
				return fmt.Errorf("received '%s', must be a valid log level (trace, debug, info, info, warning, error or critical)", valueStr)
			}
			return nil
		}),
	); err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), " System will continue with default value (info).")
	}

	//#endregion }

	//#region serverDiscoveryPort {
	theConfs.CreateConfig("serverDiscoveryPort",
		confs.WithPossibleNames([]string{"server-discovery-port", "--server-discovery-port", "VSS_SERVER_DISCOVERY_PORT"}),
		confs.WithDefaultValue(misc.NewDynamicVar("5022")),
	)
	//#endregion }

	//#region maxTimeWaitingClient_seconds {
	theConfs.CreateConfig("maxTimeWaitingClient_seconds",
		confs.WithPossibleNames([]string{"max-time-waiting-client-seconds", "--max-time-waiting-for-clients", "VSS_MAX_TIME_WAITING_CLIENTS"}),
		confs.WithDefaultValue(misc.NewDynamicVar("43200")), //12 hours
	)
	//#endregion }

	//#region DbDriver {
	theConfs.CreateConfig("DbDriver",
		confs.WithPossibleNames([]string{"db-driver", "--db-driver", "VSS_DB_DRIVER"}),
		confs.WithDefaultValue(misc.NewDynamicVar("ramcacheddb")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			valueStr := strings.ToLower(value.GetString())
			if !(valueStr == "ramcacheddb" || valueStr == "ramcacheddbpkv") {
				return fmt.Errorf("received '%s', must be a valid database driver (ramcacheddb or ramcacheddbpkv)", valueStr)
			}
			return nil
		}),
	)
	//#endregion }

	//#region DbPath {
	theConfs.CreateConfig("DbPath",
		confs.WithPossibleNames([]string{"db-path", "--db-path", "VSS_DB_PATH"}),
		confs.WithDefaultValue(misc.NewDynamicVar("%SUGGESTED_DATA_PATH%/database")),
	)
	//#endregion }

	//#region httpDataDir {
	theConfs.CreateConfig("httpDataDir",
		confs.WithPossibleNames([]string{"http-data-directory", "--http-data-directory", "--http-data-dir", "VSS_HTTP_DATA_DIRECTORY"}),
		confs.WithDefaultValue(misc.NewDynamicVar("%SUGGESTED_DATA_PATH%/http_data")),
	)
	//#endregion }

	//#region httpApiPort {
	conf, err := theConfs.CreateConfig("httpApiPort",
		confs.WithPossibleNames([]string{"http-api-port", "--http-api-port", "VSS_HTTP_API_PORT"}),
		confs.WithDefaultValue(misc.NewDynamicVar("5024")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			port, err := value.GetInte()
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("received '%s', must be a valid TCP port number (1-65535)", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (5024).")
	}
	//#endregion }

	//#region httpApiHttpsPort {
	theConfs.CreateConfig("httpApiHttpsPort",
		confs.WithPossibleNames([]string{"http-api-https-port", "--http-api-https-port", "VSS_HTTP_API_HTTPS_PORT"}),
		confs.WithDefaultValue(misc.NewDynamicVar("5025")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			port, err := value.GetInte()
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("received '%s', must be a valid TCP port number (1-65535)", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (5025).")
	}
	//#endregion }

	//#region httpApiCertFile {
	theConfs.CreateConfig("httpApiCertFile",
		confs.WithPossibleNames([]string{"http-api-cert-file", "--http-api-cert-file", "VSS_HTTP_API_CERT_FILE"}),
		confs.WithDefaultValue(misc.NewDynamicVar("%APP_DIR%/ssl/cert/vssCert.pem")),
	)
	//#endregion }

	//#region httpApiKeyFile {
	theConfs.CreateConfig("httpApiKeyFile",
		confs.WithPossibleNames([]string{"http-api-key-file", "--http-api-key-file", "VSS_HTTP_API_KEY_FILE"}),
		confs.WithDefaultValue(misc.NewDynamicVar("%APP_DIR%/ssl/cert/vssKey.pem")),
	)
	//#endregion }

	//#region httpApiReturnsFullPaths {
	theConfs.CreateConfig("httpApiReturnsFullPaths",
		confs.WithPossibleNames([]string{"http-api-returns-full-paths", "--http-api-returns-full-paths", "VSS_HTTP_API_RETURN_FULL_PATHS"}),
		confs.WithDefaultValue(misc.NewDynamicVar("false")),
	)
	//#endregion }

	//#region vstpApiPort {
	theConfs.CreateConfig("vstpApiPort",
		confs.WithPossibleNames([]string{"vstp-api-port", "--vstp-api-port", "VSS_VSTP_API_PORT"}),
		confs.WithDefaultValue(misc.NewDynamicVar("5032")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			port, err := value.GetInte()
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("received '%s', must be a valid TCP port number (1-65535)", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (5032).")
	}
	//#endregion }

	//#region RamCacheDbDumpIntervalMs {
	conf, err = theConfs.CreateConfig("RamCacheDbDumpIntervalMs",
		confs.WithPossibleNames([]string{"ram-cache-db-dump-interval-ms", "--ram-cache-db-dump-interval-ms", "VSS_RAMCACHEDB_DUMP_INTERVAL_MS"}),
		confs.WithDefaultValue(misc.NewDynamicVar("60000")), //60 seconds
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			interval, err := value.GetInte()
			if err != nil || interval < 0 {
				return fmt.Errorf("received '%s', must be a valid positive integer", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (60000).")
	}

	//#endregion }

	//#region maxKeyLength {
	conf, err = theConfs.CreateConfig("maxKeyLength",
		confs.WithPossibleNames([]string{"max-key-length", "--max-key-length", "VSS_MAX_KEY_LENGTH"}),
		confs.WithDefaultValue(misc.NewDynamicVar("255")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			length, err := value.GetInte()
			if err != nil || length < 1 {
				return fmt.Errorf("received '%s', must be a valid positive integer", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (255).")
	}
	//#endregion }

	//#region maxKeyWordLength {
	//not obrigatory, 0 means no limit (but the whole key will be validated by 'maxKeyLength')
	conf, err = theConfs.CreateConfig("maxKeyWordLength",
		confs.WithPossibleNames([]string{"max-key-word-length", "--max-key-word-length", "VSS_MAX_KEY_WORD_LENGTH"}),
		confs.WithDefaultValue(misc.NewDynamicVar("64")),
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			length, err := value.GetInte()
			if err != nil || length < 1 {
				return fmt.Errorf("received '%s', must be a valid positive integer", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (64).")
	}
	//#endregion }

	//#region maxValueSize {
	//not obrigatory, 0 means no limit
	conf, err = theConfs.CreateConfig("maxValueSize",
		confs.WithPossibleNames([]string{"max-value-size", "--max-value-size", "VSS_MAX_VALUE_SIZE"}),
		confs.WithDefaultValue(misc.NewDynamicVar("1048576")), //1 MiB
		confs.WithValidationFunc(func(conf confs.IConfItem) error {
			value := conf.NotMappedValue()
			length, err := value.GetInte()
			if err != nil || length < 1 {
				return fmt.Errorf("received '%s', must be a valid positive integer", value.GetString())
			}
			return nil
		}),
	)
	if err != nil {
		//print to stderr
		usedName, sourceInfo := conf.GetUsedNameAnsSourceInfo()
		fmt.Fprintf(os.Stderr, "Invalid '%s' (%s): %s. %s", usedName, sourceInfo, err.Error(), "System will continue with default value (1048576).")
	}
	//#endregion }

	//#region allowRawDbAccess {
	theConfs.CreateConfig("allowRawDbAccess",
		confs.WithPossibleNames([]string{"allow-raw-db-access", "--allow-raw-db-access", "VSS_ALLOW_RAW_DB_ACCESS"}),
		confs.WithDefaultValue(misc.NewDynamicVar("false")),
	)

	// warning message is printed in 'main'
	//#endregion }
	return theConfs
}

func initLogger(configs confs.IConfs) logger.ILogger {
	fileLogLevel := configs.GetConfig("fileLogLevel").Value()
	consoleLogLevel := configs.GetConfig("stdoutLogLevel").Value()

	maxLogSize := configs.GetConfig("maxLogFileSize").Value()
	fileWriter, err := logwriters.NewFileWriter(
		determineLogFile(),
		fileLogLevel.GetInt(),
		true,
		maxLogSize.GetInt64(),
	)
	if err != nil {
		panic("Failed to initialize file writer: " + err.Error())
	}

	logManager := logger.NewLogger([]logger.ILogWriter{
		logwriters.NewConsoleWriter(consoleLogLevel.GetInt(), true, true, true, true),
		fileWriter,
	}, false, 0)

	return logManager
}

func setupStdoutAndStderrInterception(logManager logger.ILogger) {
	//intercept stdout and stderr to log them into the logger
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		logManager.Error("Main", "Failed to create pipe for stdout interception: "+err.Error())
		return
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		logManager.Error("Main", "Failed to create pipe for stderr interception: "+err.Error())
		return
	}

	//redirect stdout and stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	//start goroutine to read from stdout pipe
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutReader.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				logManager.Info("STDOUT", string(buf[:n]))
			}
		}
	}()

	//start goroutine to read from stderr pipe
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrReader.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				logManager.Error("STDERR", string(buf[:n]))
			}
		}
	}()

	fmt.Println("Stdout and Stderr interception setup completed.")
}

func initStorage(configs confs.IConfs, logger logger.ILogger) storage.IStorage {

	DbDriverConfig := configs.GetConfig("DbDriver").Value()
	DbPathConfig := configs.GetConfig("DbPath").Value()

	var theStorage storage.IStorage = nil
	var err error = nil

	if DbDriverConfig.GetString() == "ramcacheddbpkv" {
		logger.Info("    using a RamCacheDB Driver (data are load/stored in a txt file)")
		dbDumpIntervalConfig := configs.GetConfig("RamCacheDbDumpIntervalMs").Value()
		_ = os.MkdirAll(DbPathConfig.GetString(), 0755)
		theStorage = storage.NewRamCacheDB(logger, DbPathConfig.GetString(), dbDumpIntervalConfig.GetInt())
	} else {
		logger.Info("    using a RamCacheDBPKV Driver (data are load/stored in a prefixtree file)")
		dbDumpIntervalConfig := configs.GetConfig("RamCacheDbDumpIntervalMs").Value()
		theStorage, err = storage.NewRamCachedDBPkv(logger, DbPathConfig.GetString(), dbDumpIntervalConfig.GetInt())
		if err != nil {
			logger.Error("Main", "Failed to initialize storage: "+err.Error())
			panic("Failed to initialize storage: " + err.Error())
		}
	}

	return theStorage
}

func getApplicationDirectory(shortenIfPossible bool) string {
	executablePath, _ := os.Executable()
	if shortenIfPossible {
		//get current working directory
		cwd, err := os.Getwd()
		if err == nil {
			relPath, err := filepath.Rel(cwd, executablePath)
			if err == nil && !strings.HasPrefix(relPath, "..") {
				ret := filepath.Dir(relPath)
				return ret
			}
		}
	}

	return filepath.Dir(executablePath)
}

func runningInPortableMode() bool {
	executablePath, _ := os.Executable()

	if strings.HasPrefix(executablePath, "/usr") || strings.HasPrefix(executablePath, "/bin") {
		return false
	}

	return true
}

func findConfigurationFile() string {
	if runningInPortableMode() {
		return filepath.Join(getApplicationDirectory(false), "config", "vss.conf")
	} else {
		return "/etc/vss/vss.conf"
	}
}

func determineLogFile() string {
	if runningInPortableMode() {
		return filepath.Join(getApplicationDirectory(false), "vss.log")
	} else {
		return "/var/log/vss.log"
	}
}

func suggestDataDirectory() string {
	if runningInPortableMode() {
		return getApplicationDirectory(false) + "/data"
	} else {
		return "/var/vss/data"
	}
}
