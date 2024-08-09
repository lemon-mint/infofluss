package main

import (
	"embed"
	"io/fs"
	"net/http"
	"sync"

	"github.com/lemon-mint/coord"
	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/coord/provider"
)

type Server struct {
	mux *http.ServeMux

	clients map[string]provider.LLMClient
	models  map[string]llm.Model
	config  *Config

	sessions      map[string]*Session
	sessionsMutex sync.Mutex
}

//go:embed frontend/dist/*
var frontend embed.FS

type svelteFS struct {
	fs.FS
}

func (g *svelteFS) Open(name string) (fs.File, error) {
	f, err := g.FS.Open(name)
	if err != nil {
		return g.FS.Open("index.html")
	}
	return f, nil
}

func NewServer(c *Config) (*Server, error) {
	var err error

	s := &Server{
		mux:      http.NewServeMux(),
		clients:  make(map[string]provider.LLMClient),
		models:   make(map[string]llm.Model),
		sessions: make(map[string]*Session),
		config:   c,
	}

	for _, m := range c.Providers {
		c, err := Connect(m)
		if err != nil {
			return nil, err
		}
		s.clients[m.Name] = c
	}

	query_planner_client := s.clients[c.ModelConfigs.QueryPlanner.Provider]
	if query_planner_client == nil {
		return nil, coord.ErrNoSuchProvider
	}
	s.models["query_planner"], err = GetModel(query_planner_client, c.ModelConfigs.QueryPlanner.Model, c.ModelConfigs.QueryPlanner.Parameters)

	response_generator_client := s.clients[c.ModelConfigs.ResponseGenerator.Provider]
	if response_generator_client == nil {
		return nil, coord.ErrNoSuchProvider
	}
	s.models["response_generator"], err = GetModel(response_generator_client, c.ModelConfigs.ResponseGenerator.Model, c.ModelConfigs.ResponseGenerator.Parameters)

	chat_client := s.clients[c.ModelConfigs.Chat.Provider]
	if chat_client == nil {
		return nil, coord.ErrNoSuchProvider
	}
	s.models["chat"], err = GetModel(chat_client, c.ModelConfigs.Chat.Model, c.ModelConfigs.Chat.Parameters)

	search_reranker_client := s.clients[c.ModelConfigs.SearchReranker.Provider]
	if search_reranker_client == nil {
		return nil, coord.ErrNoSuchProvider
	}
	s.models["search_reranker"], err = GetModel(search_reranker_client, c.ModelConfigs.SearchReranker.Model, c.ModelConfigs.SearchReranker.Parameters)

	static, err := fs.Sub(frontend, "frontend/dist")
	if err != nil {
		panic(err)
	}

	s.mux.Handle("/", http.FileServer(http.FS(&svelteFS{static})))
	s.mux.HandleFunc("/api/v1/internal/search", s.searchAPI)
	s.mux.HandleFunc("/api/v1/internal/stream/{sessID}", s.sessionSSE)

	return s, nil
}

func (g *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mux.ServeHTTP(w, r)
}
