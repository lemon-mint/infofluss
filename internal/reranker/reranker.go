package reranker

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/coord/llmtools"
)

const prompt = `You are a search re-ranker. Given a user's query and a description of what information should be extracted, rank the candidate webpages from most to least relevant. 

Each candidate webpage will include its URL, title, and snippet. Analyze these and return a JSON object containing an array of webpage indices, ordered from most to least relevant to the query, in the following format:

<reranking_result>[14, 3, 2, 5, ...]</reranking_result>

Consider these criteria when determining webpage relevance:

1. **Authoritativeness:** Prioritize websites known for credibility and trustworthiness in the given domain (e.g. Official documents, wikipedia, official websites, etc).
2. **Recency:**  When applicable, favor websites offering up-to-date information.
3. **Snippet Relevance:**  Base your decision on how closely the snippet summarizes relevant information.
4. **Keyword Relevance:**  Consider if the webpage's title or snippet contains keywords from the query.
5. **Avoiding Duplicates:**  Avoid selecting the same webpage multiple times.
6. **Contextual Relevance:**  Consider the overall context of the query and the relevance of the webpage to the query.
7. **Avoid PDF Files:**  Avoid selecting PDF files.

Select up to 4 websites, even if they overlap thematically, as long as they provide distinct information.`

var ErrRerankFailed = errors.New("rerank failed")
var re = regexp.MustCompile(`<reranking_result>((.|\n)*?)</reranking_result>`)

func RerankDocuments(ctx context.Context, m llm.Model, documents []string, query string) ([]int, error) {
	var sb strings.Builder

	if len(documents) == 0 {
		return nil, ErrRerankFailed
	}

	sb.WriteString("<user_query>\n")
	sb.WriteString(query)
	sb.WriteString("</user_query>\n\n")

	sb.WriteString("<candidates>\n")
	for i, doc := range documents {
		sb.WriteString("<candidate>\n")
		sb.WriteString("<index>" + strconv.Itoa(i) + "</index>\n")
		sb.WriteString("<content>\n")
		sb.WriteString(doc)
		sb.WriteString("\n\n")
		sb.WriteString("</content>\n")
		sb.WriteString("</candidate>\n")
	}
	sb.WriteString("</candidates>\n")

	stream := m.GenerateStream(ctx,
		&llm.ChatContext{
			Contents: []*llm.Content{
				llm.TextContent(llm.RoleUser, prompt),
				llm.TextContent(llm.RoleModel, "Sure, Please provide the candidate webpages for me to rank."),
			},
		},
		llm.TextContent(llm.RoleModel, sb.String()),
	)

	err := stream.Wait()
	if err != nil {
		return nil, err
	}

	text := llmtools.TextFromContents(stream.Content)
	var result []int

	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil, ErrRerankFailed
	}

	jsonStr := strings.TrimSpace(matches[1])
	err = json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, ErrRerankFailed
	}

	if len(result) == 0 {
		return nil, ErrRerankFailed
	}

	for _, idx := range result {
		if idx < 0 || idx >= len(documents) {
			return nil, ErrRerankFailed
		}
	}

	return result, nil
}
