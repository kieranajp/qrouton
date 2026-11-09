package desktop

import "github.com/kieranajp/qrouton/internal/core"

type Ollama struct{ ollama *core.Ollama }

func (o *Ollama) Check() core.OllamaStatus { return o.ollama.Check() }

func (o *Ollama) Pull() { o.ollama.Pull() }

func (o *Ollama) CancelPull() { o.ollama.CancelPull() }
