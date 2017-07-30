package golog

import (
	"github.com/kataras/pio"
)

/// TODO: LevelMetadata should contains
// the text, the colorful text, the name
// and the actual level which will be the key of the map.
//
// var Levels map[Level]string {
// 	DisableLevel,
// 	ErrorLevel,
// 	WarnLevel,
// 	InfoLevel,
// 	DebugLevel,
// }

// Level is a number which defines the log level.
type Level uint32

// The available log levels.
const (
	// DisableLevel will disable printer
	DisableLevel Level = iota
	// ErrorLevel will print only errors
	ErrorLevel
	// WarnLevel will print errors and warnings
	WarnLevel
	// InfoLevel will print errors, warnings and infos
	InfoLevel
	// DebugLevel will print on any level, errors, warnings, infos and debug messages
	DebugLevel
)

func fromLevelName(levelName string) Level {
	switch levelName {
	case "error":
		return ErrorLevel
	case "warning":
		fallthrough
	case "warn":
		return WarnLevel
	case "info":
		return InfoLevel
	case "debug":
		return DebugLevel
	default:
		return DisableLevel
	}
}

var (

	// Author's note:
	// Here I choose to apply the below pattern of modifying the raw text and the colorful text
	// for performance reasons, we could just create the GetTextForLevel to generate
	// the colors on-the-fly or add a sync.Once but both of these would reduce our performance.

	errorText = "[ERRO]"
	// errorTextWithColor is the color that will be printed
	// when Error/Errorf functions are being used, if `Printer#IsTerminal` is true.
	//
	// Defaults to a red color.
	errorTextWithColor = pio.Red(errorText)
	// ErrorText can modify the prefix that will be prepended
	// to the output message log when `Error/Errorf` functions are being used.
	//
	// If "newRawText" is empty then it will just return the current prefix string value.
	// If "newColorfulText" is empty then it will update the text color version using
	// the default values by using the new raw text.
	//
	// Defaults to "[ERRO]" and pio.Red("[ERRO]").
	ErrorText = func(newRawText string, newColorfulText string) (oldRawText string) {
		oldRawText = errorText
		if newRawText != "" {
			errorText = newRawText
			errorTextWithColor = pio.Red(newRawText)
		}
		if newColorfulText != "" {
			errorTextWithColor = newColorfulText
		}
		return
	}

	warnText = "[WARN]"
	// warnTextWithColor is the color that will be printed
	// when Warn/Warnf functions are being used, if `Printer#IsTerminal` is true.
	//
	// Defaults to a purplish color.
	warnTextWithColor = pio.Purple(warnText)
	// WarnText can modify the prefix that will be prepended
	// to the output message log when `Warn/Warnf` functions are being used.
	//
	// If "newRawText" is empty then it will just return the current prefix string value.
	// If "newColorfulText" is empty then it will update the text color version using
	// the default values by using the new raw text.
	//
	// Defaults to "[WARN]" and pio.Purple("[WARN]").
	WarnText = func(newRawText string, newColorfulText string) (oldRawText string) {
		oldRawText = warnText
		if newRawText != "" {
			warnText = newRawText
			warnTextWithColor = pio.Purple(newRawText)
		}
		if newColorfulText != "" {
			warnTextWithColor = newColorfulText
		}
		return
	}

	infoText = "[INFO]"
	// infoTextWithColor is the color that will be printed
	// when Info/Infof functions are being used, if `Printer#IsTerminal` is true.
	//
	// Defaults to a mix of light green and blue color.
	infoTextWithColor = pio.LightGreen(infoText)
	// InfoText can modify the prefix that will be prepended
	// to the output message log when `Info/Infof` functions are being used.
	//
	// If "newRawText" is empty then it will just return the current prefix string value.
	// If "newColorfulText" is empty then it will update the text color version using
	// the default values by using the new raw text.
	//
	// Defaults to "[INFO]" and pio.LightGreen("[INFO]").
	InfoText = func(newRawText string, newColorfulText string) (oldRawText string) {
		oldRawText = infoText
		if newRawText != "" {
			infoText = newRawText
			infoTextWithColor = pio.LightGreen(newRawText)
		}
		if newColorfulText != "" {
			infoTextWithColor = newColorfulText
		}
		return
	}

	debugText = "[DBUG]"
	// debugTextWithColor is the color that will be printed
	// when Debug/Debugf functions are beingused, if `Printer#IsTerminal` is true.
	//
	// Defaults to a yellow color.
	debugTextWithColor = pio.Yellow(debugText)
	// DebugText can modify the prefix that will be prepended
	// to the output message log when `Info/Infof` functions are being used.
	//
	// If "newRawText" is empty then it will just return the current prefix string value.
	// If "newColorfulText" is empty then it will update the text color version using
	// the default values by using the new raw text.
	//
	// Defaults to "[DBUG]" and pio.Yellow("[DBUG]").
	DebugText = func(newRawText string, newColorfulText string) (oldRawText string) {
		oldRawText = debugText
		if newRawText != "" {
			debugText = newRawText
			debugTextWithColor = pio.Yellow(newRawText)
		}
		if newColorfulText != "" {
			debugTextWithColor = newColorfulText
		}
		return
	}

	// GetTextForLevel is the function which
	// has the "final" responsibility to generate the text (colorful or not)
	// that is prepended to the leveled log message
	// when `Error/Errorf, Warn/Warnf, Info/Infof or Debug/Debugf`
	// functions are being called.
	//
	// It can be used to override the default behavior, at the start-up state.
	GetTextForLevel = func(level Level, enableColor bool) string {
		switch level {
		case ErrorLevel:
			if !enableColor {
				return errorText
			}
			return errorTextWithColor

		case WarnLevel:
			if !enableColor {
				return warnText
			}
			return warnTextWithColor

		case InfoLevel:
			if !enableColor {
				return infoText
			}
			return infoTextWithColor

		case DebugLevel:
			if !enableColor {
				return debugText
			}
			return debugTextWithColor

		default:
			return ""
		}
	}
)
