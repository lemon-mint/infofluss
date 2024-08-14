package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/infofluss/internal/chat"
	"github.com/lemon-mint/infofluss/internal/crawl"
	"github.com/lemon-mint/infofluss/internal/htmldistill"
	"github.com/lemon-mint/infofluss/internal/queryplan"
	"github.com/lemon-mint/infofluss/internal/reranker"
	"github.com/lemon-mint/infofluss/internal/search"
	"github.com/rs/zerolog/log"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func (g *Server) searchAPI(w http.ResponseWriter, r *http.Request) {
	type Query struct {
		Query string `json:"query"`
	}

	var q Query
	err := json.NewDecoder(r.Body).Decode(&q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if q.Query == "" {
		http.Error(w, "{\"error\":\"empty query\"}", http.StatusBadRequest)
		return
	}

	session := g.NewSession(q.Query)
	go g.searchWorker(session)

	type SessionCreated struct {
		ID string `json:"id"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(SessionCreated{session.ID})
}

func (g *Server) searchWorker(s *Session) {
	var close_session = true
	defer func() {
		if close_session {
			g.CloseSession(s.ID)
		}
	}()

	ctx := context.Background()

	plan, err := queryplan.GenerateQueryPlan(ctx, g.models["query_planner"], s.Query)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate query plan")
		s.Stream <- &Message{
			Type:  MessageTypeError,
			Error: "failed to generate query plan",
		}
		return
	}

	log.Info().Str("query", s.Query).Interface("plan", plan).Msg("Generated query plan")
	s.QueryPlan = plan
	s.Results = make([][]search.SearchResult, len(plan.SearchQueries))
	s.RerankedResults = make([][]search.SearchResult, len(plan.SearchQueries))

	s.Stream <- &Message{
		Type:      MessageTypeQueryPlan,
		QueryPlan: plan,
	}

	wg := &sync.WaitGroup{}
	for index, query := range plan.SearchQueries {
		wg.Add(1)
		go func(index int, query queryplan.SearchQueries) {
			defer wg.Done()
			endpoint := g.config.SearchEndpoints[rand.IntN(len(g.config.SearchEndpoints))]
			log.Info().Str("endpoint", endpoint).Str("query", query.Query).Msg("Searching")
			results, err := search.SearchSearXNG(httpClient, endpoint, query.Query, g.config.SearchEngines)
			if err != nil {
				log.Error().Err(err).Msg("Failed to search")
				s.Stream <- &Message{
					Type:    MessageTypeSearchDone,
					Success: false,
					Index:   index,
				}
				return
			}

			s.Results[index] = results
			var rerankInput []string = make([]string, len(results))
			for i, result := range results {
				type InputFormat struct {
					URL     string `json:"url"`
					Title   string `json:"title"`
					Snippet string `json:"snippet"`
				}
				data, _ := json.Marshal(InputFormat{
					URL:     result.URL,
					Title:   result.Title,
					Snippet: result.Content,
				})
				rerankInput[i] = string(data)
			}

			reranked, err := reranker.RerankDocuments(ctx, g.models["search_reranker"], rerankInput, query.Query+"\n\n"+query.Description)
			if err != nil {
				log.Error().Err(err).Msg("Failed to rerank documents")
				s.Stream <- &Message{
					Type:    MessageTypeSearchDone,
					Success: false,
					Index:   index,
				}
				return
			}

			var rerankedResults []search.SearchResult = make([]search.SearchResult, len(reranked))
			for i, rerankedIndex := range reranked {
				rerankedResults[i] = s.Results[index][rerankedIndex]
			}
			s.RerankedResults[index] = rerankedResults

			s.Stream <- &Message{
				Type:    MessageTypeSearchDone,
				Success: true,
				Index:   index,
			}
		}(index, query)
	}
	wg.Wait()
	log.Info().Interface("results", s.RerankedResults).Msg("Search results")

	var deduplicate map[string]struct{} = make(map[string]struct{})
	for _, results := range s.RerankedResults {
		for _, result := range results {
			deduplicate[result.URL] = struct{}{}
		}
	}

	wg = &sync.WaitGroup{}
	var crawlMu sync.Mutex
	for url := range deduplicate {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			contents, err := g.CrawlPage(url)
			if err != nil {
				log.Error().Err(err).Str("url", url).Msg("Failed to crawl page")
				return
			}

			s.Stream <- &Message{
				Type: MessageTypeCrawlDone,
				URL:  url,
			}
			crawlMu.Lock()
			s.CrawledPages[url] = contents
			crawlMu.Unlock()
		}(url)
	}
	wg.Wait()

	var documents []chat.Document = make([]chat.Document, 0, len(s.CrawledPages))
	var source map[string]string = make(map[string]string, len(s.CrawledPages))
	var i int = 0
	for url, crawledPage := range s.CrawledPages {
		i++
		documents = append(documents, chat.Document{
			Source:   url,
			Contents: crawledPage,
		})
		source[strconv.Itoa(i)] = url
	}

	s.Stream <- &Message{
		Type:   MessageTypeSetSource,
		Source: source,
	}

	t := time.Now()
	response := chat.Generate(ctx, g.models["response_generator"], s.Query, s.QueryPlan, documents)

	var t_first_token time.Duration
	for part := range response.Stream {
		if t_first_token == 0 {
			t_first_token = time.Since(t)
		}

		if part.Type() == llm.SegmentTypeText {
			s.Stream <- &Message{
				Type: MessageTypeGenerateStream,
				Text: string(part.(llm.Text)),
			}
		}
	}
	var t_total time.Duration = time.Since(t)

	if response.Err != nil {
		log.Error().Err(response.Err).Msg("Failed to generate response")
		s.Stream <- &Message{
			Type: MessageTypeGenerateStream,
			Text: string("\n\n\n\n\nError: failed to generate response, please try again later\n\n\n\n\n"),
		}
		s.Stream <- &Message{
			Type:  MessageTypeError,
			Error: "failed to generate response",
		}
		return
	}

	if response.UsageData != nil {
		inputTokensPerSecond := float64(response.UsageData.InputTokens) * 1000.0 / float64(t_first_token.Milliseconds())
		outputTokensPerSecond := float64(response.UsageData.OutputTokens) * 1000.0 / float64((t_total - t_first_token).Milliseconds())

		log.Info().
			Int("input_tokens", response.UsageData.InputTokens).
			Int("output_tokens", response.UsageData.OutputTokens).
			Float64("input_tokens_per_second", inputTokensPerSecond).
			Float64("output_tokens_per_second", outputTokensPerSecond).
			Msg("Generated response")
	}

	s.Stream <- &Message{
		Type: MessageTypeGenerateStreamDone,
	}
}

func (g *Server) CrawlPage(url string) ([]llm.Segment, error) {
	var rawhtml string
	var err error

	if g.config.CrawlerConfigs.Mode == "cdp" {
		rawhtml, err = crawl.ScrapeCDP(url)
		if err != nil {
			return nil, err
		}
	} else if g.config.CrawlerConfigs.Mode == "cdp_images" {
		images, err := crawl.ScrapeCDPImages(url)
		if err != nil {
			return nil, err
		}
		var parts []llm.Segment
		for _, image := range images {
			parts = append(parts, &image)
		}
		return parts, nil
	} else if g.config.CrawlerConfigs.Mode == "http" {
		rawhtml, err = crawl.ScrapeHTTP(httpClient, url)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unknown crawler mode: %s", g.config.CrawlerConfigs.Mode)
	}

	cleaned, err := htmldistill.Clean(rawhtml)
	if err != nil {
		return nil, err
	}

	if !utf8.ValidString(cleaned) {
		return nil, fmt.Errorf("utf8 validation failed")
	}

	return []llm.Segment{llm.Text(cleaned)}, nil
}
