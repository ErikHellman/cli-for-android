// Package output handles all terminal and JSON rendering for acli.
// Call Init() once from the root command's PersistentPreRunE, then use
// the package-level Success / Info / Warn / Error / Table functions.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/android-cli/acli/pkg/aclerr"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ── styles ────────────────────────────────────────────────────────────────

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	blue   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	code   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	bold   = lipgloss.NewStyle().Bold(true)

	errorPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("9")).
			Padding(0, 1)

	warnPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("3")).
			Padding(0, 1)

	successPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("2")).
			Padding(0, 1)
)

// ── renderer ─────────────────────────────────────────────────────────────

// Renderer holds per-session render settings.
type Renderer struct {
	JSON    bool
	Verbose bool
	noColor bool
	isTTY   bool
}

// Default is the package-level renderer, initialized by Init().
var Default = &Renderer{isTTY: true}

// Init initializes the package-level renderer. Call once from
// root command's PersistentPreRunE.
func Init(jsonMode, verbose, noColor bool) {
	Default = &Renderer{
		JSON:    jsonMode,
		Verbose: verbose,
		noColor: noColor,
		isTTY:   isTerminal(os.Stdout),
	}
	if noColor || !isTerminal(os.Stdout) {
		lipgloss.SetColorProfile(0) // disable colors
	}
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// ── output functions (use package-level Default renderer) ─────────────────

// Success prints a green success message.
func Success(format string, args ...any) { Default.Success(format, args...) }

// Info prints a blue informational message.
func Info(format string, args ...any) { Default.Info(format, args...) }

// Warn prints a yellow warning message.
func Warn(format string, args ...any) { Default.Warn(format, args...) }

// Error renders an AcliError to stderr.
func Error(err error) { Default.Error(err) }

// Println prints a plain line.
func Println(format string, args ...any) { Default.Println(format, args...) }

// JSON emits an arbitrary value as JSON to stdout.
func JSON(v any) { Default.emitJSON(v) }

// ── Renderer methods ──────────────────────────────────────────────────────

func (r *Renderer) Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.JSON {
		r.emitJSON(map[string]string{"status": "ok", "message": msg})
		return
	}
	fmt.Println(green.Render("✓ ") + msg)
}

func (r *Renderer) Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.JSON {
		r.emitJSON(map[string]string{"status": "info", "message": msg})
		return
	}
	fmt.Println(blue.Render("→ ") + msg)
}

func (r *Renderer) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if r.JSON {
		r.emitJSON(map[string]string{"status": "warn", "message": msg})
		return
	}
	body := yellow.Render("Warning") + "\n\n" + msg
	fmt.Fprintln(os.Stderr, warnPanel.Render(body))
}

func (r *Renderer) Println(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}

// Error renders an error. If the error is an *aclerr.AcliError it is rendered
// with full context; otherwise it is wrapped as a generic error.
func (r *Renderer) Error(err error) {
	if err == nil {
		return
	}
	var ae *aclerr.AcliError
	if !aclerr.As(err, &ae) {
		ae = &aclerr.AcliError{
			Code:       aclerr.ErrUnknown,
			Message:    err.Error(),
			Underlying: err,
		}
	}

	if r.JSON {
		out := map[string]any{
			"error": map[string]any{
				"code":    string(ae.Code),
				"message": ae.Message,
				"detail":  ae.Detail,
				"fix":     ae.FixCmds,
				"docs":    ae.DocsURL,
			},
		}
		if r.Verbose && ae.Underlying != nil {
			out["error"].(map[string]any)["underlying"] = ae.Underlying.Error()
		}
		enc := json.NewEncoder(os.Stderr)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}

	// Build human-readable panel content
	var sb strings.Builder
	sb.WriteString(red.Render("Error: " + string(ae.Code)))
	sb.WriteString("\n\n")
	sb.WriteString(bold.Render(ae.Message))

	if ae.Detail != "" {
		sb.WriteString("\n\n")
		sb.WriteString(ae.Detail)
	}

	if len(ae.FixCmds) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(dim.Render("Try:"))
		for _, cmd := range ae.FixCmds {
			sb.WriteString("\n  ")
			sb.WriteString(code.Render(cmd))
		}
	}

	if ae.DocsURL != "" {
		sb.WriteString("\n\n")
		sb.WriteString(dim.Render("Docs: " + ae.DocsURL))
	}

	if r.Verbose && ae.Underlying != nil {
		sb.WriteString("\n\n")
		sb.WriteString(dim.Render("Underlying: " + ae.Underlying.Error()))
	}

	fmt.Fprintln(os.Stderr, errorPanel.Render(sb.String()))
}

// Table renders a table with a header row and data rows.
func (r *Renderer) Table(headers []string, rows [][]string) {
	if r.JSON {
		var result []map[string]string
		for _, row := range rows {
			m := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					m[strings.ToLower(strings.ReplaceAll(h, " ", "_"))] = row[i]
				}
			}
			result = append(result, m)
		}
		r.emitJSON(result)
		return
	}

	if len(rows) == 0 {
		fmt.Println(dim.Render("(no results)"))
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Header
	var hdr strings.Builder
	for i, h := range headers {
		if i > 0 {
			hdr.WriteString("  ")
		}
		hdr.WriteString(bold.Render(padRight(h, widths[i])))
	}
	fmt.Println(hdr.String())

	// Separator
	var sep strings.Builder
	for i, w := range widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(dim.Render(strings.Repeat("─", w)))
	}
	fmt.Println(sep.String())

	// Rows
	for _, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(padRight(cell, widths[i]))
		}
		fmt.Println(line.String())
	}
}

// Table is a package-level convenience.
func Table(headers []string, rows [][]string) { Default.Table(headers, rows) }

// CheckList renders a doctor-style checklist.
func CheckList(items []CheckItem) { Default.CheckList(items) }

// CheckItem is one line in a doctor checklist.
type CheckItem struct {
	Label   string
	OK      bool
	Detail  string
	FixCmds []string
}

func (r *Renderer) CheckList(items []CheckItem) {
	if r.JSON {
		type jsonItem struct {
			Label   string   `json:"label"`
			OK      bool     `json:"ok"`
			Detail  string   `json:"detail,omitempty"`
			FixCmds []string `json:"fix,omitempty"`
		}
		var out []jsonItem
		for _, it := range items {
			out = append(out, jsonItem{Label: it.Label, OK: it.OK, Detail: it.Detail, FixCmds: it.FixCmds})
		}
		r.emitJSON(map[string]any{"checks": out})
		return
	}

	for _, it := range items {
		if it.OK {
			fmt.Printf("%s  %s\n", green.Render("✓"), it.Label)
			if it.Detail != "" {
				fmt.Printf("   %s\n", dim.Render(it.Detail))
			}
		} else {
			fmt.Printf("%s  %s\n", red.Render("✗"), bold.Render(it.Label))
			if it.Detail != "" {
				fmt.Printf("   %s\n", it.Detail)
			}
			for _, cmd := range it.FixCmds {
				fmt.Printf("   %s %s\n", dim.Render("Run:"), code.Render(cmd))
			}
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func (r *Renderer) emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
