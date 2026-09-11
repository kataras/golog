package golog

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"io"
	"sync"
)

// Formatter is responsible to print a log to the logger's writer.
//
// Format may call any getter on `log.Logger`, but it must not log through that
// same logger. When "dest" is the logger's Printer, each Write to it is atomic
// across every logger sharing that destination, so a Formatter that builds the
// whole line and writes it once cannot have its output interleaved with
// another line; a Formatter that writes in several pieces can. When "dest" is
// a per-level writer set with `SetLevelOutput`, the logger holds a write lock
// around Format that it shares with its clones.
type Formatter interface {
	// The name of the formatter.
	String() string
	// Set any options and return a clone,
	// generic. See `Logger.SetFormat`.
	Options(opts ...any) Formatter
	// Writes the "log" to "dest" logger.
	Format(dest io.Writer, log *Log) bool
}

// JSONFormatter is a Formatter type for JSON logs.
type JSONFormatter struct {
	Indent string

	// opts is derived from Indent once, by Options.
	opts json.Options
}

// jsonEncoder pairs an encoder with the buffer it encodes into.
//
// jsontext special-cases a *bytes.Buffer destination and encodes straight into
// its spare capacity, so pairing the two costs nothing once the pair has been
// through the pool.
type jsonEncoder struct {
	buf bytes.Buffer
	enc *jsontext.Encoder
}

// encode marshals "log" into the buffer from scratch.
func (e *jsonEncoder) encode(log *Log, opts json.Options) error {
	e.buf.Reset()
	e.enc.Reset(&e.buf, opts)
	return json.MarshalEncode(e.enc, log)
}

// jsonEncoders is shared by every JSONFormatter, level and writer.
//
// The per-level map of encoders this replaced existed because an
// encoding/json.Encoder is bound for life to the writer it was built with. That
// binding was also a bug: the cached encoder kept writing to whichever writer
// the level had when it was first used, so a SetLevelOutput after the first log
// at that level was silently ignored. jsontext.Encoder.Reset rebinds, so one
// pool serves everything.
var jsonEncoders = sync.Pool{New: func() any {
	e := new(jsonEncoder)
	e.enc = jsontext.NewEncoder(&e.buf)
	return e
}}

// maxPooledJSONBuffer is the largest scratch buffer worth keeping alive between
// log lines. One 10MB message should not permanently cost 10MB of idle memory.
const maxPooledJSONBuffer = 64 << 10

// String returns the name of the Formatter.
// In this case it returns "json".
// It's used to map the formatter names with their implementations.
func (f *JSONFormatter) String() string {
	return "json"
}

// Options sets the options for the JSON Formatter (currently only indent).
func (f *JSONFormatter) Options(opts ...any) Formatter {
	formatter := &JSONFormatter{
		Indent: "  ",
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if indent, ok := opt.(string); ok {
			formatter.Indent = indent
			break
		}
	}

	formatter.opts = jsonOptions(formatter.Indent)
	return formatter
}

// jsonOptions builds the encoding/json/v2 options this formatter encodes with.
//
// The base is DefaultOptionsV1, which is not laziness: a logger serializes
// whatever a caller put in Fields, and v2 changed the rendering of several
// ordinary Go values. Measured against the v1 encoder this replaced, taking v2's
// defaults would have meant a time.Duration failing to marshal at all ("no
// default representation") and taking the whole log line down with it, a nil
// slice or map rendering as [] and {} instead of null, a [N]byte becoming a
// base64 string instead of an array, and -- worst, because it is silent and it
// is somebody else's struct -- `omitempty` on a caller's zero int or false bool
// no longer omitting. None of that is a change golog gets to make on a caller's
// behalf.
//
// EscapeForHTML is the one deliberate exception. v1 wrote '<', '>' and '&' as
// <, > and & on the assumption the output might be embedded in a
// web page; a log file is not a web page, so those are now written literally.
// EscapeForJS is left on, so U+2028 and U+2029 are still escaped.
func jsonOptions(indent string) json.Options {
	opts := []json.Options{
		jsonv1.DefaultOptionsV1(),
		jsontext.EscapeForHTML(false),
	}

	// An empty indent means compact output, the same as the v1
	// Encoder.SetIndent("", "") this replaced. Passing WithIndent("") instead
	// would turn on multiline output with nothing to indent by.
	if indent != "" {
		opts = append(opts, jsontext.WithIndent(indent))
	}

	return json.JoinOptions(opts...)
}

// compactJSONOptions serves a JSONFormatter built directly, as a zero value,
// rather than through Options.
var compactJSONOptions = sync.OnceValue(func() json.Options { return jsonOptions("") })

func (f *JSONFormatter) options() json.Options {
	if f.opts != nil {
		return f.opts
	}
	if f.Indent == "" {
		return compactJSONOptions()
	}
	return jsonOptions(f.Indent)
}

// Format prints the logs in JSON format.
//
// Usage:
// logger.SetFormat("json") or
// logger.SetLevelFormat("info", "json")
//
// A value in `log.Fields` the encoder cannot represent (a NaN, a channel, a
// function) does not turn the line into plain text: the fields are replaced by
// a single "golog_error" field carrying the encoder's message, so a JSON stream
// stays a JSON stream. Only an entry that fails even then, such as one at a
// level missing from `Levels`, is handed back to the plain formatter.
func (f *JSONFormatter) Format(dest io.Writer, log *Log) bool {
	opts := f.options()

	e := jsonEncoders.Get().(*jsonEncoder)

	if err := e.encode(log, opts); err != nil {
		fields := log.Fields
		log.Fields = Fields{"golog_error": err.Error()}
		err = e.encode(log, opts)
		log.Fields = fields

		if err != nil {
			// Not returned to the pool: the encoder stopped part-way through
			// a value and nothing about the rest of its state is worth
			// carrying forward. Errors are rare enough that the lost buffer
			// never shows up.
			return false
		}
	}

	// One Write per line. Handing dest to the encoder directly instead would
	// let jsontext flush a half-written line whenever its scratch buffer passes
	// three quarters full -- two writes per line in practice, which is two
	// chances for a concurrent logger sharing the writer to cut in, and a
	// truncated fragment on the output if the marshal then fails.
	_, err := dest.Write(e.buf.Bytes())

	if e.buf.Cap() <= maxPooledJSONBuffer {
		jsonEncoders.Put(e)
	}

	return err == nil
}
