package formatter

import (
    "encoding/json"
    "fmt"
    "io"
    "strings"
    "text/tabwriter"

    "gopkg.in/yaml.v3"
)

// OutputFormat represents the output format
type OutputFormat string

const (
    FormatTable OutputFormat = "table"
    FormatJSON  OutputFormat = "json"
    FormatYAML  OutputFormat = "yaml"
)

// Formatter formats output
type Formatter struct {
    format OutputFormat
    writer  io.Writer
}

// NewFormatter creates a new formatter
func NewFormatter(format string, writer io.Writer) *Formatter {
    f := OutputFormat(format)
    if f != FormatJSON && f != FormatYAML {
        f = FormatTable
    }
    return &Formatter{
        format: f,
        writer:  writer,
    }
}

// Output outputs data in the specified format
func (f *Formatter) Output(data interface{}) error {
    switch f.format {
    case FormatJSON:
        return f.outputJSON(data)
    case FormatYAML:
        return f.outputYAML(data)
    default:
        return f.outputTable(data)
    }
}

// outputJSON outputs as JSON
func (f *Formatter) outputJSON(data interface{}) error {
    encoder := json.NewEncoder(f.writer)
    encoder.SetIndent("", "  ")
    return encoder.Encode(data)
}

// outputYAML outputs as YAML
func (f *Formatter) outputYAML(data interface{}) error {
    encoder := yaml.NewEncoder(f.writer)
    return encoder.Encode(data)
}

// outputTable outputs as table
func (f *Formatter) outputTable(data interface{}) error {
    // Simple table output
    w := tabwriter.NewWriter(f.writer, 0, 0, 2, ' ', 0)
    defer w.Flush()

    // This is a simplified version
    fmt.Fprintf(w, "%v\n", data)
    return nil
}

// ParseFormat parses output format from string
func ParseFormat(s string) OutputFormat {
    switch strings.ToLower(s) {
    case "json":
        return FormatJSON
    case "yaml":
        return FormatYAML
    default:
        return FormatTable
    }
}
