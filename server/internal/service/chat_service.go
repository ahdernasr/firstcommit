package service

import "context"

// ChatService provides conversational follow‑ups on top of an existing guide
// using the same RAG loop (retrieve context → local LLM → cache).
type ChatService interface {
	// Ask returns an answer string for the user's follow‑up question.
	Ask(ctx context.Context, contextID, question string) (string, error)
}

// chatService is the concrete implementation that delegates context retrieval
// to GuideService and then runs the RAG pipeline (placeholder for now).
type chatService struct {
	guideSvc GuideService
}

// NewChatService wires dependencies and returns ChatService.
func NewChatService(guideSvc GuideService) ChatService {
	return &chatService{guideSvc: guideSvc}
}

// Ask fetches the original guide/context and passes it—together with the
// user's question—into the RAG model to generate a follow‑up answer.
// The actual RAG call is left as a TODO so you can plug in your local model.
func (s *chatService) Ask(ctx context.Context, contextID, question string) (string, error) {
	if question == "" {
		return "", nil
	}

	// 1. Retrieve existing guide/context chunks.
	_, err := s.guideSvc.GetGuide(ctx, contextID)
	if err != nil {
		return "", err
	}

	// 2. TODO: Embed `question`, retrieve top‑k vectors, run local LLM.
	// Placeholder until the RAG pipeline is wired in:
	answer := "This is a placeholder answer. RAG integration pending."

	return answer, nil
}
