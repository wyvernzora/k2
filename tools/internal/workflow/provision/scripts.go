package provision

import (
	"embed"

	"github.com/wyvernzora/k2/tools/internal/shellscript"
)

//go:embed scripts/*.sh.tmpl
var scriptTemplates embed.FS

var scripts = shellscript.New(scriptTemplates, "scripts/*.sh.tmpl")

func renderScript(name string, data any) string {
	return scripts.Render(name, data)
}
