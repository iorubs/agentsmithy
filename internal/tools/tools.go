// Package tools builds ADK toolsets from agentsmithy config tool
// references. Each entry in `tools.mcp` or `tools.a2a` lowers to a
// toolset that the pipeline attaches to autonomous agents that
// reference the tool by name.
package tools

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2aclient"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/agent/remoteagent"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/agenttool"
	"google.golang.org/adk/tool/mcptoolset"
)

// Resolved is the lowered form of one config tool entry.
type Resolved struct {
	Name string
	// One of Tools or Toolset is populated. mcp entries produce a
	// toolset (lazy connection, expands at LLM time); a2a entries
	// produce a single tool wrapping a remote agent.
	Tools   []adktool.Tool
	Toolset adktool.Toolset
}

// MCP builds a toolset that connects to an MCP server over
// Streamable HTTP. urlOrAddr accepts either a full URL
// (`http://host:port/`) or a bare host:port, in which case the
// scheme defaults to http:// and the path defaults to `/`.
func MCP(name, urlOrAddr string) (Resolved, error) {
	endpoint := normaliseMCPEndpoint(urlOrAddr)
	ts, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.StreamableClientTransport{Endpoint: endpoint},
	})
	if err != nil {
		return Resolved{}, fmt.Errorf("mcp %q: %w", name, err)
	}
	return Resolved{Name: name, Toolset: ts}, nil
}

// A2A builds a tool wrapping a remote A2A agent.
func A2A(name, url string) (Resolved, error) {
	card := &a2a.AgentCard{
		Name:               name,
		URL:                url,
		Version:            "1.0.0",
		ProtocolVersion:    "0.2.0",
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		AdditionalInterfaces: []a2a.AgentInterface{
			{Transport: a2a.TransportProtocolJSONRPC, URL: url},
		},
		Capabilities: a2a.AgentCapabilities{Streaming: true},
	}
	remote, err := remoteagent.NewA2A(remoteagent.A2AConfig{
		Name:      name,
		AgentCard: card,
		ClientFactory: a2aclient.NewFactory(
			a2aclient.WithJSONRPCTransport(&http.Client{Timeout: 5 * time.Minute}),
		),
	})
	if err != nil {
		return Resolved{}, fmt.Errorf("a2a %q: %w", name, err)
	}
	return Resolved{
		Name:  name,
		Tools: []adktool.Tool{agenttool.New(remote, nil)},
	}, nil
}

// normaliseMCPEndpoint accepts either a full URL or a bare host:port
// and returns a usable Streamable HTTP endpoint. Bare addresses
// default to http:// and `/`.
func normaliseMCPEndpoint(s string) string {
	if len(s) >= 7 && (s[:7] == "http://" || (len(s) >= 8 && s[:8] == "https://")) {
		return s
	}
	if s != "" && s[0] == ':' {
		s = "127.0.0.1" + s
	}
	return "http://" + s + "/"
}
