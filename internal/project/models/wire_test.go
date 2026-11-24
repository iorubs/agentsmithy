package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iorubs/agentsmithy/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

// TestOpenAI_GenerateContent_Roundtrip stands up an httptest server
// imitating the OpenAI Chat Completions endpoint and verifies the
// provider builds the expected request body and decodes the reply.
func TestOpenAI_GenerateContent_Roundtrip(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "sk-test")

	var gotReq oaiRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(oaiResponse{
			Choices: []oaiChoice{{
				Message:      oaiMessage{Role: "assistant", Content: "hello back"},
				FinishReason: "stop",
			}},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	temperature, maxTokens := 0.25, 64
	llm, err := newOpenAI(config.ModelEntry{
		Model:       "gpt-4o-mini",
		BaseURL:     srv.URL,
		APIKeyEnv:   "TEST_OPENAI_KEY",
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatalf("newOpenAI: %v", err)
	}

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hi"}}},
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: "be terse"}},
			},
		},
	}

	var gotResp *adkmodel.LLMResponse
	for resp, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
		gotResp = resp
	}

	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q; want Bearer sk-test", gotAuth)
	}
	if gotReq.Model != "gpt-4o-mini" {
		t.Errorf("request.Model = %q; want gpt-4o-mini", gotReq.Model)
	}
	if len(gotReq.Messages) != 2 || gotReq.Messages[0].Role != "system" || gotReq.Messages[1].Role != "user" {
		t.Errorf("unexpected message shape: %+v", gotReq.Messages)
	}
	if gotReq.Temperature == nil || *gotReq.Temperature != temperature {
		t.Errorf("request.Temperature = %v; want %v", gotReq.Temperature, temperature)
	}
	if gotReq.MaxTokens == nil || *gotReq.MaxTokens != maxTokens {
		t.Errorf("request.MaxTokens = %v; want %v", gotReq.MaxTokens, maxTokens)
	}
	if gotResp == nil || gotResp.Content == nil || len(gotResp.Content.Parts) != 1 {
		t.Fatalf("unexpected response: %+v", gotResp)
	}
	if got := gotResp.Content.Parts[0].Text; got != "hello back" {
		t.Errorf("response text = %q; want hello back", got)
	}
}

// TestOpenAI_GenerateContent_HTTPError surfaces non-200 responses as
// an iter error rather than a bogus empty completion.
func TestOpenAI_GenerateContent_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	llm, err := newOpenAI(config.ModelEntry{Model: "gpt-4o-mini", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newOpenAI: %v", err)
	}

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: "hi"}}}},
	}
	for _, err := range llm.GenerateContent(context.Background(), req, false) {
		if err == nil {
			t.Fatal("expected error on 401, got nil")
		}
		return
	}
	t.Fatal("expected at least one yield")
}

// TestBorrowed_GenerateContent_Roundtrip exercises the full borrowed
// path against an in-process MCP client+server pair: agent → server
// session → sampling/createMessage → canned client handler.
func TestBorrowed_GenerateContent_Roundtrip(t *testing.T) {
	maxTokens := 64
	llm, err := newBorrowed(config.ModelEntry{Model: "claude-haiku", MaxTokens: &maxTokens})
	if err != nil {
		t.Fatalf("newBorrowed: %v", err)
	}

	clientT, serverT := mcp.NewInMemoryTransports()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Server-side session is established when the client connects.
	type sessRes struct {
		ss  *mcp.ServerSession
		err error
	}
	sessCh := make(chan sessRes, 1)
	go func() {
		ss, err := server.Connect(ctx, serverT, nil)
		sessCh <- sessRes{ss, err}
	}()

	var gotParams *mcp.CreateMessageParams
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, &mcp.ClientOptions{
		CreateMessageHandler: func(_ context.Context, req *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
			gotParams = req.Params
			return &mcp.CreateMessageResult{
				Model:   "claude-haiku-test",
				Role:    "assistant",
				Content: &mcp.TextContent{Text: "borrowed reply"},
			}, nil
		},
	})
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer cs.Close()

	srv := <-sessCh
	if srv.err != nil {
		t.Fatalf("server.Connect: %v", srv.err)
	}
	defer srv.ss.Close()

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hi"}}},
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: "be terse"}},
			},
		},
	}

	sessCtx := WithSession(ctx, srv.ss)
	var gotResp *adkmodel.LLMResponse
	for resp, err := range llm.GenerateContent(sessCtx, req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
		gotResp = resp
	}

	if gotParams == nil {
		t.Fatal("client handler never invoked")
	}
	if gotParams.MaxTokens != 64 {
		t.Errorf("MaxTokens = %d; want 64", gotParams.MaxTokens)
	}
	if gotParams.SystemPrompt != "be terse" {
		t.Errorf("SystemPrompt = %q; want be terse", gotParams.SystemPrompt)
	}
	if len(gotParams.Messages) != 1 || gotParams.Messages[0].Role != "user" {
		t.Errorf("unexpected sampling messages: %+v", gotParams.Messages)
	}
	if gotResp == nil || gotResp.Content == nil || len(gotResp.Content.Parts) != 1 {
		t.Fatalf("unexpected response: %+v", gotResp)
	}
	if got := gotResp.Content.Parts[0].Text; got != "borrowed reply" {
		t.Errorf("response text = %q; want borrowed reply", got)
	}
	if gotResp.ModelVersion != "claude-haiku-test" {
		t.Errorf("ModelVersion = %q; want claude-haiku-test", gotResp.ModelVersion)
	}
}

// TestBorrowed_GenerateContent_NoSession returns a clear error when
// the agent runs under a non-MCP transport (no session in ctx).
func TestBorrowed_GenerateContent_NoSession(t *testing.T) {
	maxTokens := 64
	llm, err := newBorrowed(config.ModelEntry{MaxTokens: &maxTokens})
	if err != nil {
		t.Fatalf("newBorrowed: %v", err)
	}

	for _, err := range llm.GenerateContent(context.Background(), &adkmodel.LLMRequest{}, false) {
		if err == nil {
			t.Fatal("expected error without MCP session")
		}
		return
	}
	t.Fatal("expected at least one yield")
}

// TestBorrowed_NewLLM_RequiresMaxTokens is an explicit guard for the
// host-pays-for-tokens contract.
func TestBorrowed_NewLLM_RequiresMaxTokens(t *testing.T) {
	if _, err := newBorrowed(config.ModelEntry{}); err == nil {
		t.Fatal("expected error when MaxTokens is unset")
	}
	bad := 0
	if _, err := newBorrowed(config.ModelEntry{MaxTokens: &bad}); err == nil {
		t.Fatal("expected error when MaxTokens is 0")
	}
}

// TestAnthropic_GenerateContent_Roundtrip stands up an httptest server
// imitating the Messages API and verifies the provider hoists the
// system prompt, sends tool schemas, and decodes a tool_use reply.
func TestAnthropic_GenerateContent_Roundtrip(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test")

	var gotReq antRequest
	var gotKey, gotVersion, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		gotVersion = r.Header.Get("Anthropic-Version")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(antResponse{
			Model: "claude-sonnet-4-5",
			Content: []antBlock{
				{Type: "text", Text: "looking it up"},
				{Type: "tool_use", ID: "tu_1", Name: "search", Input: map[string]any{"query": "go"}},
			},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	maxTokens := 512
	llm, err := newAnthropic(config.ModelEntry{
		Model:     "claude-sonnet-4-5",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
		MaxTokens: &maxTokens,
	})
	if err != nil {
		t.Fatalf("newAnthropic: %v", err)
	}

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: "find go docs"}}}},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: "be terse"}}},
			Tools: []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{{
				Name:                 "search",
				Description:          "search the docs",
				ParametersJsonSchema: map[string]any{"type": "OBJECT", "properties": map[string]any{"query": map[string]any{"type": "STRING"}}},
			}}}},
		},
	}

	var gotResp *adkmodel.LLMResponse
	for resp, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
		gotResp = resp
	}

	if gotPath != "/v1/messages" {
		t.Errorf("path = %q; want /v1/messages", gotPath)
	}
	if gotKey != "sk-ant-test" {
		t.Errorf("X-Api-Key = %q; want sk-ant-test", gotKey)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("Anthropic-Version = %q; want %q", gotVersion, anthropicVersion)
	}
	if gotReq.System != "be terse" {
		t.Errorf("request.System = %q; want be terse", gotReq.System)
	}
	if gotReq.MaxTokens != maxTokens {
		t.Errorf("request.MaxTokens = %d; want %d", gotReq.MaxTokens, maxTokens)
	}
	if len(gotReq.Messages) != 1 || gotReq.Messages[0].Role != "user" {
		t.Fatalf("unexpected message shape: %+v", gotReq.Messages)
	}
	if len(gotReq.Tools) != 1 || gotReq.Tools[0].Name != "search" {
		t.Fatalf("unexpected tools: %+v", gotReq.Tools)
	}
	if got := gotReq.Tools[0].InputSchema["type"]; got != "object" {
		t.Errorf("tool schema type = %v; want lowercase object", got)
	}
	if gotResp == nil || len(gotResp.Content.Parts) != 2 {
		t.Fatalf("unexpected response: %+v", gotResp)
	}
	fc := gotResp.Content.Parts[1].FunctionCall
	if fc == nil || fc.Name != "search" || fc.ID != "tu_1" || fc.Args["query"] != "go" {
		t.Errorf("unexpected function call: %+v", fc)
	}
}

// TestAnthropic_GenerateContent_ToolResult verifies a FunctionResponse
// part becomes a user-role tool_result message rather than an
// assistant turn.
func TestAnthropic_GenerateContent_ToolResult(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test")

	var gotReq antRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(antResponse{
			Content: []antBlock{{Type: "text", Text: "done"}},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	llm, err := newAnthropic(config.ModelEntry{
		Model:     "claude-sonnet-4-5",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
	})
	if err != nil {
		t.Fatalf("newAnthropic: %v", err)
	}

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "search"}}},
			{Role: "model", Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{ID: "tu_1", Name: "search", Args: map[string]any{"query": "go"}}}}},
			{Role: "user", Parts: []*genai.Part{{FunctionResponse: &genai.FunctionResponse{ID: "tu_1", Name: "search", Response: map[string]any{"result": "ok"}}}}},
		},
	}
	for _, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
	}

	if len(gotReq.Messages) != 3 {
		t.Fatalf("got %d messages; want 3: %+v", len(gotReq.Messages), gotReq.Messages)
	}
	if gotReq.Messages[1].Role != "assistant" || gotReq.Messages[1].Content[0].Type != "tool_use" {
		t.Errorf("unexpected assistant turn: %+v", gotReq.Messages[1])
	}
	result := gotReq.Messages[2]
	if result.Role != "user" || result.Content[0].Type != "tool_result" || result.Content[0].ToolUseID != "tu_1" {
		t.Errorf("unexpected tool result turn: %+v", result)
	}
	if payload := result.Content[0].Content; !strings.Contains(payload, `"result":"ok"`) {
		t.Errorf("tool_result payload = %q; want the marshalled response", payload)
	}
}

// TestAnthropic_GenerateContent_HTTPError surfaces non-200 responses as
// an iter error.
func TestAnthropic_GenerateContent_HTTPError(t *testing.T) {
	t.Setenv("TEST_ANTHROPIC_KEY", "sk-ant-test")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	llm, err := newAnthropic(config.ModelEntry{
		Model:     "claude-sonnet-4-5",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_ANTHROPIC_KEY",
	})
	if err != nil {
		t.Fatalf("newAnthropic: %v", err)
	}

	var gotErr error
	for _, err := range llm.GenerateContent(context.Background(), &adkmodel.LLMRequest{}, false) {
		gotErr = err
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "401") {
		t.Fatalf("err = %v; want 401", gotErr)
	}
}

// TestBedrock_GenerateContent_Roundtrip points the Converse client at
// an httptest server standing in for the Bedrock runtime endpoint and
// verifies the system block, tool config, and toolUse decoding.
func TestBedrock_GenerateContent_Roundtrip(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "")

	var gotBody map[string]any
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{
			"output": {"message": {"role": "assistant", "content": [
				{"text": "looking it up"},
				{"toolUse": {"toolUseId": "tu_1", "name": "search", "input": {"query": "go"}}}
			]}},
			"stopReason": "tool_use"
		}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	llm, err := newBedrock(config.ModelEntry{
		Model:   "anthropic.claude-3-5-sonnet-20241022-v2:0",
		BaseURL: srv.URL,
		Region:  "us-east-1",
	})
	if err != nil {
		t.Fatalf("newBedrock: %v", err)
	}

	req := &adkmodel.LLMRequest{
		Contents: []*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: "find go docs"}}}},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: "be terse"}}},
			Tools: []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{{
				Name:                 "search",
				Description:          "search the docs",
				ParametersJsonSchema: map[string]any{"type": "OBJECT", "properties": map[string]any{"query": map[string]any{"type": "STRING"}}},
			}}}},
		},
	}

	var gotResp *adkmodel.LLMResponse
	for resp, err := range llm.GenerateContent(context.Background(), req, false) {
		if err != nil {
			t.Fatalf("GenerateContent: %v", err)
		}
		gotResp = resp
	}

	if !strings.Contains(gotPath, "/converse") {
		t.Errorf("path = %q; want the converse operation", gotPath)
	}
	system, _ := gotBody["system"].([]any)
	if len(system) != 1 {
		t.Fatalf("system = %v; want one block", gotBody["system"])
	}
	if got := system[0].(map[string]any)["text"]; got != "be terse" {
		t.Errorf("system text = %v; want be terse", got)
	}
	toolConfig, _ := gotBody["toolConfig"].(map[string]any)
	tools, _ := toolConfig["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("toolConfig.tools = %v; want one tool", toolConfig)
	}
	spec := tools[0].(map[string]any)["toolSpec"].(map[string]any)
	if spec["name"] != "search" {
		t.Errorf("tool name = %v; want search", spec["name"])
	}
	schema := spec["inputSchema"].(map[string]any)["json"].(map[string]any)
	if schema["type"] != "object" {
		t.Errorf("tool schema type = %v; want lowercase object", schema["type"])
	}

	if gotResp == nil || len(gotResp.Content.Parts) != 2 {
		t.Fatalf("unexpected response: %+v", gotResp)
	}
	fc := gotResp.Content.Parts[1].FunctionCall
	if fc == nil || fc.Name != "search" || fc.ID != "tu_1" || fc.Args["query"] != "go" {
		t.Errorf("unexpected function call: %+v", fc)
	}
}

// TestBedrock_GenerateContent_HTTPError surfaces service errors as an
// iter error rather than an empty completion.
func TestBedrock_GenerateContent_HTTPError(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIATEST")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"nope"}`, http.StatusForbidden)
	}))
	defer srv.Close()

	llm, err := newBedrock(config.ModelEntry{Model: "m", BaseURL: srv.URL, Region: "us-east-1"})
	if err != nil {
		t.Fatalf("newBedrock: %v", err)
	}

	var gotErr error
	for _, err := range llm.GenerateContent(context.Background(), &adkmodel.LLMRequest{}, false) {
		gotErr = err
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "converse") {
		t.Fatalf("err = %v; want a converse error", gotErr)
	}
}
