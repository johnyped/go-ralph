package pi

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/johnyped/go-ralph/internal/config"
)

// rendererScript is the Python renderer run inline via python3 -u -c.
// It reads pi's JSONL stream from stdin, writes the session ID sidecar,
// prints live tool activity, and exits 1 if any tool returned an error.
// %s is substituted with the quoted sidecar file path before use.
const rendererScript = `
import sys, json
had_error = False
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        e = json.loads(line)
        t = e.get('type', '')
        if t == 'session':
            sid = e.get('id', '')
            if sid:
                open(%s, 'w').write(sid)
        elif t == 'tool_execution_start':
            print('  ⚙', e.get('toolName', ''), flush=True)
            args = e.get('args', {})
            if args:
                s = json.dumps(args, separators=(',', ':'))
                print('    └─', s[:200] + ('...' if len(s) > 200 else ''), flush=True)
        elif t == 'tool_execution_end':
            if e.get('isError'):
                had_error = True
            parts = e.get('result', {}).get('content', [])
            text = ''.join(p.get('text', '') for p in parts if p.get('type') == 'text').strip()
            if text:
                print('    →', text[:200] + ('...' if len(text) > 200 else ''), flush=True)
        elif t == 'agent_end':
            print('  ✓ done', flush=True)
        elif t == 'message_update':
            ae = e.get('assistantMessageEvent', {})
            atype = ae.get('type', '')
            if atype == 'thinking_start':
                print('💭 ', end='', flush=True)
            elif atype in ('text_delta', 'thinking_delta'):
                print(ae.get('delta', ''), end='', flush=True)
            elif atype in ('text_end', 'thinking_end'):
                print(flush=True)
    except:
        pass
sys.exit(1 if had_error else 0)
`

// PaneArgv returns the bash -c command for herdr agent start.
// Uses --mode json with $(cat promptFile) prompt injection.
// Tees raw JSONL to logFile, renders live tool activity in the pane.
// Writes session ID to <logFile>.session-id sidecar.
// read -r keeps the pane alive until ralph reads the RALPH_DONE: sentinel.
func PaneArgv(dir string, cfg *config.Config, slug, promptFile, logFile, model string) []string {
	jsonArgs := append([]string{cfg.PiPath}, buildArgs(dir, cfg, slug, model)...)
	sidecarFile := logFile + ".session-id"
	script := fmt.Sprintf(rendererScript, shellQuote(sidecarFile))
	cmd := fmt.Sprintf(
		`%s "$(cat %s)" | tee %s | python3 -u -c "%s"; _pi_rc=${PIPESTATUS[0]} _py_rc=${PIPESTATUS[2]}; echo RALPH_DONE:$((_pi_rc > 0 ? _pi_rc : _py_rc)); read -r`,
		shellJoin(jsonArgs),
		shellQuote(promptFile),
		shellQuote(logFile),
		shellEscapeForDoubleQuote(script),
	)
	return []string{"bash", "-c", cmd}
}

// SessionIDPath returns the path of the session-id sidecar for a given log file.
func SessionIDPath(logFile string) string {
	return logFile + ".session-id"
}

// LogFile returns the JSONL log path for an issue slug and attempt number.
// Attempt 1 → <slug>.jsonl; attempt N → <slug>-attempt-N.jsonl.
func LogFile(dir, slug string, attempt int) string {
	name := slug + ".jsonl"
	if attempt > 1 {
		name = fmt.Sprintf("%s-attempt-%d.jsonl", slug, attempt)
	}
	return filepath.Join(dir, ".ralph", "logs", name)
}

func buildArgs(dir string, cfg *config.Config, slug, model string) []string {
	args := []string{"--mode", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--name", fmt.Sprintf("%s: %s", cfg.PiSessionPrefix, slug))
	return args
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"'") {
			parts[i] = fmt.Sprintf("%q", a)
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "'\"'\"'"))
}

// shellEscapeForDoubleQuote escapes a string for embedding inside bash "...".
func shellEscapeForDoubleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "\\`")
	s = strings.ReplaceAll(s, `$`, `\$`)
	return s
}
