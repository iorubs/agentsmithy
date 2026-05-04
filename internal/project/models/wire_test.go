package models

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	llm, err := newOpenAI(config.ModelEntry{
		Model:     "gpt-4o-mini",
		BaseURL:   srv.URL,
		APIKeyEnv: "TEST_OPENAI_KEY",
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
