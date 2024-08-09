package chat

import (
	"context"
	"strconv"
	"strings"

	"github.com/lemon-mint/coord/llm"
	"github.com/lemon-mint/infofluss/internal/queryplan"
)

const prompt = `You are InfoFluss, a large language model search engine with access to up-to-date information. 

Follow these guidelines when answering user queries based on provided reference documents:

* **Prioritize accuracy and completeness:** Provide comprehensive answers based on the references unless the user requests brevity.
* **Deep understanding:** Demonstrate a thorough understanding of the subject matter when responding to informational inquiries.
* **Language matching:** Respond in the same language as the user's query unless instructed otherwise.
* **Code clarity:** When generating code, include explanations and adhere to good coding practices.
* **Reasoning transparency:** For reasoning tasks, clearly explain each step before providing the final answer.
* **Reference grounding:**  Ensure answers are based on the provided reference documents.
* **Prioritize external knowledge:** Prioritize the accuracy of your response over your internal knowledge base and aim to provide a comprehensive response.
* **Indicate Source:** Indicate the number of the material you referenced in your response. The format is: "text§[<document number>]"

**Always refer to these instructions when responding to user queries.**

You have no knowledge cut-off date.

Your responses must be in Markdown format, and it is important to organize the information for easy readability.
Make active use of Markdown tables, bolding, italics, and links.

All responses should begin with a Markdown heading.`

type Document struct {
	Source  string
	Content string
}

func Generate(ctx context.Context, m llm.Model, query string, queryplan *queryplan.QueryPlan, documents []Document) *llm.StreamContent {
	var sb strings.Builder

	sb.WriteString("<user_query>\n")
	sb.WriteString("<query>\n")
	sb.WriteString(query)
	sb.WriteString("</query>\n")
	sb.WriteString("<instructions>\n")
	sb.WriteString(queryplan.Instruction)
	sb.WriteString("</instructions>\n")
	sb.WriteString("</user_query>\n\n")

	sb.WriteString("<documents>\n")
	for i, doc := range documents {
		sb.WriteString("<document>\n")
		sb.WriteString("<index>" + strconv.Itoa(i+1) + "</index>\n")
		sb.WriteString("<source>\n")
		sb.WriteString(doc.Source)
		sb.WriteString("\n")
		sb.WriteString("</source>\n")
		sb.WriteString("<content>\n")
		sb.WriteString(doc.Content)
		sb.WriteString("\n\n")
		sb.WriteString("</content>\n")
		sb.WriteString("</document>\n")
	}
	sb.WriteString("</documents>\n")

	return m.GenerateStream(ctx, &llm.ChatContext{
		SystemInstruction: prompt,
	}, llm.TextContent(llm.RoleUser, sb.String()))
}
