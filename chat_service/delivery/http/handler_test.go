package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"2026_2_PinPals/chat_service/domain"
	"2026_2_PinPals/pkg/middleware"
)

type transportValidationService struct {
	createChatCalls    int
	listMessagesCalls  int
	createMessageCalls int
	hasAccessCalls     int
	lastUserID         int
	lastChatID         int
	lastLimit          int
	lastBeforeID       int
}

func (s *transportValidationService) CreateChat(context.Context, int, int) (domain.Chat, error) {
	s.createChatCalls++
	return domain.Chat{}, nil
}

func (s *transportValidationService) ListChats(context.Context, int) ([]domain.Chat, error) {
	return nil, nil
}

func (s *transportValidationService) HasAccess(context.Context, int, int) (bool, error) {
	s.hasAccessCalls++
	return true, nil
}

func (s *transportValidationService) ListMessages(_ context.Context, userID, chatID, limit, beforeID int) ([]domain.MessageView, error) {
	s.listMessagesCalls++
	s.lastUserID = userID
	s.lastChatID = chatID
	s.lastLimit = limit
	s.lastBeforeID = beforeID
	return []domain.MessageView{}, nil
}

func (s *transportValidationService) CreateMessage(context.Context, int, int, string, *int) (domain.MessageView, error) {
	s.createMessageCalls++
	return domain.MessageView{}, nil
}

func newAuthenticatedRecorder(t *testing.T, request *http.Request, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	tokens := middleware.NewTokenManager([]byte(strings.Repeat("s", 32)), 3600)
	token, err := tokens.Issue(7)
	if err != nil {
		t.Fatalf("issue test access token: %v", err)
	}
	request.AddCookie(&http.Cookie{Name: middleware.AccessTokenCookieName, Value: token})

	recorder := httptest.NewRecorder()
	middleware.Access(tokens)(handler).ServeHTTP(recorder, request)
	return recorder
}

func TestCreateChatRejectsInvalidUserIDAtTransport(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "zero user id", body: `{"user_id":0}`},
		{name: "negative user id", body: `{"user_id":-1}`},
		{name: "wrong user id type", body: `{"user_id":"not-an-int"}`},
		{name: "unknown field", body: `{"user_id":12,"unexpected":true}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &transportValidationService{}
			handler := NewHandler(service, NewHub(), HandlerOptions{})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chats", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")

			recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.CreateChat))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if service.createChatCalls != 0 {
				t.Fatalf("CreateChat was called %d times for invalid transport input", service.createChatCalls)
			}
		})
	}
}

func TestListMessagesRejectsInvalidChatIDAtTransport(t *testing.T) {
	for _, pathValue := range []string{"", "abc", "0", "-1", "999999999999999999999999999"} {
		t.Run(pathValue, func(t *testing.T) {
			service := &transportValidationService{}
			handler := NewHandler(service, NewHub(), HandlerOptions{})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/"+pathValue+"/messages", nil)
			request.SetPathValue("chatID", pathValue)

			recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.ListMessages))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if service.listMessagesCalls != 0 {
				t.Fatalf("ListMessages was called %d times for invalid chat ID", service.listMessagesCalls)
			}
		})
	}
}

func TestListMessagesRejectsInvalidPaginationAtTransport(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{name: "zero limit", query: "limit=0"},
		{name: "negative limit", query: "limit=-1"},
		{name: "too large limit", query: "limit=101"},
		{name: "non numeric limit", query: "limit=abc"},
		{name: "empty limit", query: "limit="},
		{name: "duplicate limit", query: "limit=10&limit=20"},
		{name: "negative cursor", query: "before_id=-1"},
		{name: "non numeric cursor", query: "before_id=abc"},
		{name: "empty cursor", query: "before_id="},
		{name: "duplicate cursor", query: "before_id=10&before_id=20"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &transportValidationService{}
			handler := NewHandler(service, NewHub(), HandlerOptions{})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/12/messages?"+test.query, nil)
			request.SetPathValue("chatID", "12")

			recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.ListMessages))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if service.listMessagesCalls != 0 {
				t.Fatalf("ListMessages was called %d times for invalid pagination", service.listMessagesCalls)
			}
		})
	}
}

func TestListMessagesPassesValidatedParametersToService(t *testing.T) {
	service := &transportValidationService{}
	handler := NewHandler(service, NewHub(), HandlerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/12/messages?limit=25&before_id=123", nil)
	request.SetPathValue("chatID", "12")

	recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.ListMessages))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if service.listMessagesCalls != 1 {
		t.Fatalf("ListMessages calls = %d, want 1", service.listMessagesCalls)
	}
	if service.lastUserID != 7 || service.lastChatID != 12 || service.lastLimit != 25 || service.lastBeforeID != 123 {
		t.Fatalf("unexpected service parameters: user=%d chat=%d limit=%d before=%d", service.lastUserID, service.lastChatID, service.lastLimit, service.lastBeforeID)
	}
}

func TestSendMessageRejectsInvalidChatIDAtTransport(t *testing.T) {
	service := &transportValidationService{}
	handler := NewHandler(service, NewHub(), HandlerOptions{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chats/0/messages", strings.NewReader(`{"body":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.SetPathValue("chatID", "0")

	recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.SendMessage))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if service.createMessageCalls != 0 {
		t.Fatalf("CreateMessage was called %d times for invalid chat ID", service.createMessageCalls)
	}
}

func TestSendMessageRejectsInvalidPayloadAtTransport(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid related id", body: `{"body":"hello","related_to":0}`},
		{name: "blank message", body: `{"body":"   "}`},
		{name: "too long message", body: `{"body":"` + strings.Repeat("x", domain.MaxMessageLength+1) + `"}`},
		{name: "unsupported type", body: `{"type":"typing","body":"hello"}`},
		{name: "unknown field", body: `{"body":"hello","admin":true}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &transportValidationService{}
			handler := NewHandler(service, NewHub(), HandlerOptions{})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chats/12/messages", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.SetPathValue("chatID", "12")

			recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.SendMessage))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if service.createMessageCalls != 0 {
				t.Fatalf("CreateMessage was called %d times for invalid payload", service.createMessageCalls)
			}
		})
	}
}

type deniedAccessService struct {
	*transportValidationService
}

func (s *deniedAccessService) HasAccess(context.Context, int, int) (bool, error) {
	s.hasAccessCalls++
	return false, nil
}

func TestWebSocketRejectsChatWithoutAccessBeforeUpgrade(t *testing.T) {
	service := &deniedAccessService{transportValidationService: &transportValidationService{}}
	handler := NewHandler(service, NewHub(), HandlerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/12/ws", nil)
	request.SetPathValue("chatID", "12")
	recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.WebSocket))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
	}
	if service.hasAccessCalls != 1 {
		t.Fatalf("HasAccess calls = %d, want 1", service.hasAccessCalls)
	}
}

func TestWebSocketRejectsUnauthenticatedRequest(t *testing.T) {
	service := &transportValidationService{}
	handler := NewHandler(service, NewHub(), HandlerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/12/ws", nil)
	request.SetPathValue("chatID", "12")
	recorder := httptest.NewRecorder()
	handler.WebSocket(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	if service.hasAccessCalls != 0 {
		t.Fatalf("HasAccess was called %d times for an unauthenticated request", service.hasAccessCalls)
	}
}

func TestWebSocketRejectsInvalidChatIDBeforeAccessCheck(t *testing.T) {
	service := &transportValidationService{}
	handler := NewHandler(service, NewHub(), HandlerOptions{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/chats/nope/ws", nil)
	request.SetPathValue("chatID", "nope")

	recorder := newAuthenticatedRecorder(t, request, http.HandlerFunc(handler.WebSocket))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if service.hasAccessCalls != 0 {
		t.Fatalf("HasAccess was called %d times for invalid chat ID", service.hasAccessCalls)
	}
}
