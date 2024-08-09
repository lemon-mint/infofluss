package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/lemon-mint/infofluss/internal/queryplan"
	"github.com/lemon-mint/infofluss/internal/search"
)

type Session struct {
	ID string

	Query           string
	QueryPlan       *queryplan.QueryPlan
	Results         [][]search.SearchResult
	RerankedResults [][]search.SearchResult
	CrawledPages    map[string]string

	Error error

	Stream chan *Message
}

type MessageType int16

const (
	MessageTypeUnknown            MessageType = 0
	MessageTypeHeartbeat          MessageType = 1
	MessageTypeError              MessageType = 2
	MessageTypeQueryPlan          MessageType = 3
	MessageTypeSearchDone         MessageType = 4
	MessageTypeGenerateStream     MessageType = 5
	MessageTypeGenerateStreamDone MessageType = 6
	MessageTypeCrawlDone          MessageType = 7
	MessageTypeSetSource          MessageType = 8
	MessageTypeDisconnect         MessageType = 9
)

type Message struct {
	Type      MessageType          `json:"type"`
	QueryPlan *queryplan.QueryPlan `json:"query_plan,omitempty"`

	Success bool   `json:"success,omitempty"`
	Index   int    `json:"index,omitempty"` // MessageTypeSearchDone
	Text    string `json:"text,omitempty"`  // MessageTypeGenerateStream
	URL     string `json:"url,omitempty"`   // MessageTypeCrawlDone
	Error   string `json:"error,omitempty"` // MessageTypeGenerateStreamDone

	Source map[string]string `json:"source,omitempty"` // MessageTypeSetSource
}

func (g *Server) GetSession(id string) *Session {
	g.sessionsMutex.Lock()
	defer g.sessionsMutex.Unlock()
	return g.sessions[id]
}

func (g *Server) CloseSession(id string) {
	g.sessionsMutex.Lock()
	defer g.sessionsMutex.Unlock()
	s, ok := g.sessions[id]
	if ok {
		func() {
			defer recover() // ignore double close panic
			close(s.Stream)
		}()
	}
	delete(g.sessions, id)
}

func newSessionID() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (g *Server) NewSession(query string) *Session {
	g.sessionsMutex.Lock()
	defer g.sessionsMutex.Unlock()
	s := &Session{
		ID:           newSessionID(),
		Query:        query,
		Stream:       make(chan *Message, 128),
		CrawledPages: map[string]string{},
	}
	g.sessions[s.ID] = s
	return s
}

func (g *Server) sessionSSE(w http.ResponseWriter, r *http.Request) {
	sessID := r.PathValue("sessID")
	if sessID == "" {
		http.Error(w, "Bad Request 0", http.StatusBadRequest)
		return
	}

	session := g.GetSession(sessID)
	if session == nil {
		http.Error(w, "Not Found 0", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Internal Server Error 1", http.StatusInternalServerError)
		return
	}
	defer flusher.Flush()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	heartbeat_ticker := time.NewTicker(5 * time.Second)
	defer heartbeat_ticker.Stop()

	io.WriteString(w, "id: "+sessID+"\n\n")
	flusher.Flush()

	defer func() {
		msg := &Message{
			Type: MessageTypeDisconnect,
		}
		json_msg, err := json.Marshal(msg)
		if err != nil {
			return
		}
		io.WriteString(w, "data: "+string(json_msg)+"\n\n")
		flusher.Flush()
	}()

	for {
		select {
		case <-heartbeat_ticker.C:
			msg := &Message{
				Type: MessageTypeHeartbeat,
			}

			json_msg, err := json.Marshal(msg)
			if err != nil {
				return
			}

			io.WriteString(w, "data: "+string(json_msg)+"\n\n")
			flusher.Flush()
		case msg, ok := <-session.Stream:
			if msg == nil || !ok {
				return
			}

			json_msg, err := json.Marshal(msg)
			if err != nil {
				return
			}

			io.WriteString(w, "data: "+string(json_msg)+"\n\n")
			flusher.Flush()
		}
	}
}
