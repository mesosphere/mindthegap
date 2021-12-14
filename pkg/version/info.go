package version

import (
	"bytes"
	"fmt"
	"html/template"
	"runtime"
	"strings"
)

// Build information. Populated at build-time.
//nolint:gochecknoglobals // Version globals are set at build time.
var (
	version      string
	major        string
	minor        string
	patch        string
	revision     string
	branch       string
	commitDate   string
	gitTreeState string
	goVersion    = runtime.Version()
)

// Print returns version information.
func Print(program string) string {
	m := map[string]string{
		"program":      program,
		"version":      version,
		"major":        major,
		"minor":        minor,
		"patch":        patch,
		"revision":     revision,
		"branch":       branch,
		"commitDate":   commitDate,
		"gitTreeState": gitTreeState,
		"goVersion":    goVersion,
		"platform":     runtime.GOOS + "/" + runtime.GOARCH,
	}
	t := template.Must(template.New("version").Parse(`
	{{.program}}, version {{.version}} (branch: {{.branch}}, revision: {{.revision}}{{with .gitTreeState}}, gitTreeState: {{.}}{{end}})
		build date:       {{.commitDate}}
		go version:       {{.goVersion}}
		platform:         {{.platform}}
	`))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}
	return strings.TrimSpace(buf.String())
}

// Info returns version, branch, revision, and git tree state information.
func Info() string {
	return fmt.Sprintf("(version=%s, branch=%s, revision=%s, gitTreeState=%s)", version, branch, revision, gitTreeState)
}

// BuildContext returns goVersion, and commitDate information.
func BuildContext() string {
	return fmt.Sprintf("(go=%s, date=%s)", goVersion, commitDate)
}
