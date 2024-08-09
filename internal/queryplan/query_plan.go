package queryplan

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/coord/llmtools"
	yaml "gopkg.in/yaml.v3"
)

type QueryPlan struct {
	Language      string          `yaml:"language" json:"language"`
	SearchQueries []SearchQueries `yaml:"search_queries" json:"search_queries"`
	Instruction   string          `yaml:"instruction" json:"instruction"`
}

type SearchQueries struct {
	Query       string `yaml:"query" json:"query"`
	Description string `yaml:"description" json:"description"`
}

const prompt = `You are a search query generator. Your role is to analyze user queries and generate specific search queries that can be used in search engines. Follow these instructions carefully:

0. Current time is {{CURRENT_TIME}}.

1. You will be given a user query. Here it is:
<user_query>
{{USER_QUERY}}
</user_query>

2. First, detect the language of the input query. Use a two-letter language code (e.g., "ko" for Korean, "en" for English).

3. Analyze the query and think step-by-step about how to generate appropriate search queries. Summarize your reasoning process in a <reasoning> block. Use this reasoning to understand the user's intent. If the input is a simple noun or object name, assume the user is looking for a definition/explanation/example/review/etc. and construct your search query accordingly.

4. Generate search queries following these rules:
   - Focus on generating search queries in a sequential search process.
   - For each search query, include a description of the information to be extracted from the search.

5. Provide your output in YAML format, structured as follows:
   - language: (two-letter language code)
   - search_queries: (list of search queries)
     - query: (the search query)
     - description: (description of information to extract)
   - instruction: (specify the user's intent and the action to be taken after the search for further processing)

6. Generate search keywords only in English, but if the input question language is not English and related to local information like opening hours, local event, local places..., generate search keywords in both the detected language and English.

7. Here's an example of how your output should be structured:

<reasoning>
(Your step-by-step reasoning process goes here)
</reasoning>

` + "```yaml\n" + `
language: (language code)
search_queries:
- query: "(first search query)"
  description: "(description of information to extract)"
- query: "(second search query)"
  description: "(description of information to extract)"
instruction: |-
  (Instruction for further processing)
` + "```" + `

----
Start your response with <reasoning>`

var ErrFailedToGenerateQueryPlan = errors.New("Failed to generate query plan")

func GenerateQueryPlan(ctx context.Context, m llm.Model, query string) (*QueryPlan, error) {
	prompt := strings.ReplaceAll(prompt, "{{CURRENT_TIME}}", time.Now().Format(time.RFC1123Z))
	prompt = strings.ReplaceAll(prompt, "{{USER_QUERY}}", query)

	stream := m.GenerateStream(ctx, &llm.ChatContext{}, llm.TextContent(llm.RoleUser, prompt))
	err := stream.Wait()
	if err != nil {
		return nil, err
	}

	text := llmtools.TextFromContents(stream.Content)

	text = strings.TrimSpace(text)
	_, text, ok := strings.Cut(text, "```yaml\n")
	if !ok {
		return nil, ErrFailedToGenerateQueryPlan
	}
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var queryPlan QueryPlan
	err = yaml.Unmarshal([]byte(text), &queryPlan)
	if err != nil {
		return nil, err
	}

	return &queryPlan, nil
}
