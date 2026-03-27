package logwriters

import (
	"fmt"
	"os"
	"strings"
	"time"

	"rtonello/vss/sources/misc/logger"
	"rtonello/vss/sources/misc/logger/loggerutils"
)

type ConsoleWriter struct {
	colors          bool
	displayLinePref bool
	displayDateTime bool
	logLevel        int
	printMillis     bool
	lastLineOpened  bool
	currIdentPrefix string
}

func NewConsoleWriter(logLevel int, useColors, displayLinePrefix, displayDateTime, printMillis bool) *ConsoleWriter {
	return &ConsoleWriter{colors: useColors, displayLinePref: displayLinePrefix, displayDateTime: displayDateTime, logLevel: logLevel, printMillis: printMillis, lastLineOpened: false}
}

func (c *ConsoleWriter) Write(msg string, level int, name string, dateTime time.Time, timezoneoffset int) {
	if level < c.logLevel {
		return
	}

	// if the message ends with '\n', we consider that the line is closed

	var out string
	if c.displayLinePref && !c.lastLineOpened {
		lvl := loggerutils.LevelToString(level, "INFO")
		prefix := loggerutils.GenerateLineBeginning(lvl, name, c.displayDateTime, dateTime, timezoneoffset, c.printMillis)
		//create a string with 'len(prefix)' spaces for identing
		c.currIdentPrefix = strings.Repeat(" ", len(prefix))

		out = prefix + msg
	} else {
		out = msg
	}

	out = loggerutils.Ident(out, c.currIdentPrefix)

	//handle colors
	if c.colors {
		switch level {
		case logger.LEVEL_INFO2:
			out = "\033[0;36m" + out + "\033[0m"
		case logger.LEVEL_WARNING:
			out = "\033[0;33m" + out + "\033[0m"
		case logger.LEVEL_ERROR:
			out = "\033[0;31m" + out + "\033[0m"
		case logger.LEVEL_CRITICAL:
			//white and red background
			out = "\033[1;37;41m" + out + "\033[0m"
		case logger.LEVEL_DEBUG:
			out = "\033[0;35m" + out + "\033[0m"
		case logger.LEVEL_DEBUG2, logger.LEVEL_TRACE:
			out = "\033[0;90m" + out + "\033[0m"
		}
	}

	if level == logger.LEVEL_ERROR || level == logger.LEVEL_CRITICAL {
		fmt.Fprint(os.Stderr, out)
	} else {
		fmt.Print(out)
	}

	c.lastLineOpened = !strings.HasSuffix(msg, "\n")
}
