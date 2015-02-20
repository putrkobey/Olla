package domain

import "strings"

// OllamaDefaultTag is the tag Ollama assumes when a model name carries none:
// "ollama pull foo" lists as "foo:latest" and clients still ask for "foo".
const OllamaDefaultTag = "latest"

// OllamaDefaultTagName returns name with Ollama's default tag appended and true
// when name carries no tag, false otherwise.
//
// A colon only starts a tag when no path separator follows it: the colon in
// "registry.example.com:5000/foo" belongs to the host, so that name is untagged.
