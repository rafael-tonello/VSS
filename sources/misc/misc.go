package misc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

func GetOnly(source string, validChars string) string {
	result := ""
	for _, c := range source {
		if strings.ContainsRune(validChars, c) {
			result += string(c)
		}
	}
	return result
}

func SeparateKeyAndValue(keyValuePair string, possibleCharSeps string) (string, string) {

	for _, char := range possibleCharSeps {
		if strings.Contains(keyValuePair, string(char)) {
			after, before, _ := strings.Cut(keyValuePair, string(char))
			return after, before
		}
	}
	return keyValuePair, ""
}

// #region terminal utils {
func executeCommandAndGetOutput(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	outputBytes, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(outputBytes)), nil
}

func getTerminalSize_tput() (width, height int, err error) {
	cmd := "tput"
	argsWidth := []string{"cols"}
	argsHeight := []string{"lines"}

	outWidth, err := executeCommandAndGetOutput(cmd, argsWidth...)
	if err != nil {
		return 100, 24, err
	}
	outHeight, err := executeCommandAndGetOutput(cmd, argsHeight...)
	if err != nil {
		return 100, 24, err
	}

	var w, h int
	_, err = fmt.Sscanf(outWidth, "%d", &w)
	if err != nil {
		return 100, 24, err
	}
	_, err = fmt.Sscanf(outHeight, "%d", &h)
	if err != nil {
		return 100, 24, err
	}

	return w, h, nil
}

func getTerminalSize() (width, height int, err error) {
	w, h, err := term.GetSize(0) // 0 = stdin file descriptor
	if err != nil {
		//try tput as fallback
		return getTerminalSize_tput()
	}

	return w, h, nil
}

func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// creates a 'line' that occupies the full width of the terminal
// if central text is provided, it will be centered in the line
// charToUse should be the character to be used, but you can provide more than one character (it could be truncated if the terminal width is not multiple of the charToUse length)
func CreateTermSeparator(terminalSize int, charToUse string, centralText string) string {
	width := terminalSize
	line := ""
	if centralText == "" {
		for len(line) < width {
			line += charToUse
		}
	} else {
		padding := (width - len(centralText)) / 2
		for len(line) < padding {
			line += charToUse
		}
		line += centralText
		for len(line) < width {
			line += charToUse
		}
	}
	//trunc if needed
	if len(line) > width {
		line = line[:width]
	}

	return line
}

func CreateTerminalSeparatorForCurrentTerminal(charToUse string, centralText string) string {
	width, _, err := getTerminalSize()
	if err != nil {
		width = 100
	}
	return CreateTermSeparator(width, charToUse, centralText)
}

func PrintTerminalSeparator(charToUse string, centralText string) {
	sep := CreateTerminalSeparatorForCurrentTerminal(charToUse, centralText)
	os.Stdout.WriteString(sep + "\n")
}

func PrintAtCenterOfTerminal(text string) {
	width, _, err := getTerminalSize()
	if err != nil {
		width = 100
	}

	for {
		if len(text) >= width {
			os.Stdout.WriteString(text + "\n")
			text = text[width:]
		} else {
			padding := (width - len(text)) / 2
			fmt.Println(strings.Repeat(" ", padding) + text)
			break
		}
	}
}

//#endregion }
