package golog

import (
	"encoding/json/jsontext"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/kataras/golog/printer"
)

// Level is a number which defines the log level.
type Level uint32

// MarshalJSON implements the json marshaler for Level.
func (l Level) MarshalJSON() ([]byte, error) {
	if level, ok := lookupLevel(l); ok {
		return []byte(strconv.Quote(level.Name)), nil
	}

	return nil, fmt.Errorf("unknown level %v", uint32(l))
}

// MarshalJSONTo implements the encoding/json/v2 MarshalerTo interface for Level.
//
// It exists alongside MarshalJSON rather than replacing it. Both the v1 and the
// v2 json packages prefer this method when a type has both, and writing the
// name straight into the encoder saves the quoted []byte that MarshalJSON has
// to allocate for every log line. MarshalJSON stays for callers that invoke it
// directly and for anything still reflecting on the v1 Marshaler interface.
func (l Level) MarshalJSONTo(enc *jsontext.Encoder) error {
	if level, ok := lookupLevel(l); ok {
		return enc.WriteToken(jsontext.String(level.Name))
	}

	return fmt.Errorf("unknown level %v", uint32(l))
}

// String implements the fmt.Stringer interface for level.
// Returns the level's name.
func (l Level) String() string {
	if level, ok := lookupLevel(l); ok {
		return level.Name
	}

	return ""
}

// The available built'n log levels, users can add or modify a level via `Levels` field.
const (
	// DisableLevel will disable the printer.
	DisableLevel Level = iota
	// FatalLevel will `os.Exit(1)` no matter the level of the logger.
	// If the logger's level is fatal, error, warn, info or debug
	// then it will print the log message too.
	FatalLevel
	// ErrorLevel will print only errors.
	ErrorLevel
	// WarnLevel will print errors and warnings.
	WarnLevel
	// InfoLevel will print errors, warnings and infos.
	InfoLevel
	// DebugLevel will print on any level, fatals, errors, warnings, infos and debug logs.
	DebugLevel
)

// Levels contains the levels and their
// mapped (pointer of, in order to be able to be modified) metadata.
//
// Callers may add or replace entries, and may change an entry's fields. The
// safe way to do either while the program is already logging is `RegisterLevel`
// and `LevelMetadata.SetText`. Writing to the map or to a metadata field
// directly also works, but only before the first log line, or followed by
// `ReloadLevels`: logging never reads this map on its own hot path, it reads a
// snapshot taken from it, so a direct write after that point is not seen until
// the next snapshot.
var Levels = map[Level]*LevelMetadata{
	DisableLevel: {
		Name:             "disable",
		AlternativeNames: []string{"disabled", "none", "off"},
		Title:            "",
	},
	FatalLevel: {
		Name:      "fatal",
		Title:     "[FTAL]",
		ColorCode: printer.Red,
		Style:     []printer.RichOption{printer.Background},
	},
	ErrorLevel: {
		Name:      "error",
		Title:     "[ERRO]",
		ColorCode: printer.Red,
	},
	WarnLevel: {
		Name:             "warn",
		AlternativeNames: []string{"warning"},
		Title:            "[WARN]",
		ColorCode:        printer.Magenta,
	},
	InfoLevel: {
		Name:      "info",
		Title:     "[INFO]",
		ColorCode: printer.Cyan,
	},
	DebugLevel: {
		Name:      "debug",
		Title:     "[DBUG]",
		ColorCode: printer.Yellow,
	},
}

// levelTable is an immutable copy of Levels. Every lookup on the logging path
// goes through one, so a caller writing to Levels can neither race with a log
// line nor throw the process with a concurrent map access.
type levelTable struct {
	byLevel map[Level]*LevelMetadata
	// ordered holds the levels ascending, so ParseLevel resolves a name shared
	// by two levels the same way every time instead of by map iteration order.
	ordered []Level
}

var (
	// levelsMu guards writes to Levels made through this package and the
	// rebuild of the snapshot. Direct writes by callers are not covered, which
	// is why the Levels doc asks for them to happen before logging starts.
	levelsMu sync.Mutex
	levels   atomic.Pointer[levelTable]
)

// RegisterLevel adds "level" to `Levels` with the given metadata, or replaces
// the metadata it already has, and makes the change visible to logging at
// once. It is the runtime-safe form of `Levels[level] = meta`.
//
//	const SuccessLevel golog.Level = 6
//	golog.RegisterLevel(SuccessLevel, &golog.LevelMetadata{
//		Name:      "success",
//		Title:     "[SUCC]",
//		ColorCode: printer.Green,
//	})
func RegisterLevel(level Level, meta *LevelMetadata) {
	levelsMu.Lock()
	Levels[level] = meta
	reloadLevelsLocked()
	levelsMu.Unlock()
}

// ReloadLevels makes changes written directly to `Levels`, or to the fields of
// one of its entries, visible to logging. `RegisterLevel` and
// `LevelMetadata.SetText` call it on the caller's behalf.
func ReloadLevels() {
	reloadLevels()
}

func reloadLevels() *levelTable {
	levelsMu.Lock()
	defer levelsMu.Unlock()
	return reloadLevelsLocked()
}

// reloadLevelsLocked rebuilds the snapshot from Levels. The caller holds levelsMu.
func reloadLevelsLocked() *levelTable {
	t := &levelTable{
		byLevel: make(map[Level]*LevelMetadata, len(Levels)),
		ordered: make([]Level, 0, len(Levels)),
	}

	for level, meta := range Levels {
		if meta == nil {
			continue
		}
		// A copy, so that a later SetText on the caller's pointer cannot
		// change a snapshot that is being read.
		c := *meta
		c.AlternativeNames = slices.Clone(meta.AlternativeNames)
		c.Style = slices.Clone(meta.Style)
		t.byLevel[level] = &c
		t.ordered = append(t.ordered, level)
	}
	slices.Sort(t.ordered)

	levels.Store(t)
	return t
}

// currentLevels returns the snapshot, building it on first use.
func currentLevels() *levelTable {
	if t := levels.Load(); t != nil {
		return t
	}
	return reloadLevels()
}

// lookupLevel returns the metadata of "level" as logging sees it.
//
// A miss rebuilds the snapshot once and looks again, so a level added to
// `Levels` directly since the last rebuild (the init-time pattern of the
// examples) is picked up on first use. An entry that is genuinely unknown pays
// that rebuild on every lookup, which is the cost of logging at an
// unregistered level.
func lookupLevel(level Level) (*LevelMetadata, bool) {
	if meta, ok := currentLevels().byLevel[level]; ok {
		return meta, true
	}

	meta, ok := reloadLevels().byLevel[level]
	return meta, ok
}

// ParseLevel returns a `golog.Level` from a string level.
// Note that all existing log levels (name, prefix and color) can be customized
// and new one can be added by the package-level `golog.Levels` map variable.
//
// An unknown name yields `DisableLevel`.
func ParseLevel(levelName string) Level {
	levelName = strings.ToLower(levelName)

	if level, ok := currentLevels().parse(levelName); ok {
		return level
	}

	// Same reasoning as lookupLevel: the name may belong to a level written
	// into Levels directly since the last rebuild.
	if level, ok := reloadLevels().parse(levelName); ok {
		return level
	}

	return DisableLevel
}

func (t *levelTable) parse(lowerName string) (Level, bool) {
	for _, level := range t.ordered {
		meta := t.byLevel[level]
		if strings.ToLower(meta.Name) == lowerName {
			return level, true
		}

		for _, altName := range meta.AlternativeNames {
			if strings.ToLower(altName) == lowerName {
				return level, true
			}
		}
	}

	return DisableLevel, false
}

// LevelMetadata describes the information
// behind a log Level, each level has its own unique metadata.
type LevelMetadata struct {
	// The Name of the Level
	// that named (lowercased) will be used
	// to convert a string level on `SetLevel`
	// to the correct Level type.
	Name string
	// AlternativeNames are the names that can be referred to this specific log level.
	// i.e Name = "warn"
	// AlternativeNames = []string{"warning"}, it's an optional field,
	// therefore we keep Name as a simple string and created this new field.
	AlternativeNames []string
	// Tha Title is the prefix of the log level.
	// See `ColorCode` and `Style` too.
	// Both `ColorCode` and `Style` should be respected across writers.
	Title string
	// ColorCode a color for the `Title`.
	ColorCode int
	// Style one or more rich options for the `Title`.
	Style []printer.RichOption
}

// Text returns the text that should be
// prepended to the log message when a specific
// log level is being written.
//
// With "enableColor" set the ANSI escapes are always emitted; it is the
// caller's statement that the destination renders them, typically the answer
// of `printer.SupportsColor(w)` for that destination. It does not consult
// os.Stdout.
func (m *LevelMetadata) Text(enableColor bool) string {
	if enableColor {
		return printer.RichText(m.Title, m.ColorCode, m.Style...)
	}
	return m.Title
}

// SetText can modify the prefix that will be prepended
// to the output message log when `Error/Errorf` functions are being used.
//
// The change is visible to logging at once; see `Levels`.
func (m *LevelMetadata) SetText(title string, colorCode int, style ...printer.RichOption) {
	m.Title = title
	m.ColorCode = colorCode
	m.Style = style
	ReloadLevels()
}

var (
	// ErrorText can modify the prefix that will be prepended
	// to the output message log when `Error/Errorf` functions are being used.
	//
	// Defaults to "[ERRO]" in red.
	//
	// Deprecated: Use `Levels[ErrorLevel].SetText(title string, colorCode int, style ...printer.RichOption)` instead.
	ErrorText = Levels[ErrorLevel].SetText

	// WarnText can modify the prefix that will be prepended
	// to the output message log when `Warn/Warnf` functions are being used.
	//
	// Defaults to "[WARN]" in magenta.
	//
	// Deprecated: Use `Levels[WarnLevel].SetText(title string, colorCode int, style ...printer.RichOption)` instead.
	WarnText = Levels[WarnLevel].SetText

	// InfoText can modify the prefix that will be prepended
	// to the output message log when `Info/Infof` functions are being used.
	//
	// Defaults to "[INFO]" in cyan.
	//
	// Deprecated: Use `Levels[InfoLevel].SetText(title string, colorCode int, style ...printer.RichOption)` instead.
	InfoText = Levels[InfoLevel].SetText

	// DebugText can modify the prefix that will be prepended
	// to the output message log when `Debug/Debugf` functions are being used.
	//
	// Defaults to "[DBUG]" in yellow.
	//
	// Deprecated: Use `Levels[DebugLevel].SetText(title string, colorCode int, style ...printer.RichOption)` instead.
	DebugText = Levels[DebugLevel].SetText

	// GetTextForLevel is the function which
	// has the "final" responsibility to generate the text (colorful or not)
	// that is prepended to the leveled log message
	// when `Error/Errorf, Warn/Warnf, Info/Infof or Debug/Debugf`
	// functions are being called.
	//
	// It can be used to override the default behavior, at the start-up state.
	GetTextForLevel = func(level Level, enableColor bool) string {
		if meta, ok := lookupLevel(level); ok {
			return meta.Text(enableColor)
		}
		return ""
	}

	// GetNameForLevel is the function which
	// has the "final" responsibility to generagte the name of the level
	// that is prepended to the leveled log message
	GetNameForLevel = func(level Level) string {
		if meta, ok := lookupLevel(level); ok {
			return meta.Name
		}
		return ""
	}
)
