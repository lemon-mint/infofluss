package htmldistill

import (
	"strings"

	"golang.org/x/net/html"
)

var usefulattrs = map[string]bool{
	"src":         true,
	"href":        true,
	"alt":         true,
	"title":       true,
	"role":        true,
	"aria-label":  true,
	"aria-hidden": true,
	"aria-atomic": true,
	"name":        true,
	"type":        true,
	"value":       true,
	"content":     true,
	"property":    true,
}

func distillPipeline(n *html.Node) {
	if n == nil {
		return
	}

	unusefulE := false
	for i := range n.Attr {
		if !usefulattrs[n.Attr[i].Key] {
			unusefulE = true
			break
		}
	}

	if unusefulE {
		newAttr := make([]html.Attribute, 0, len(n.Attr))
		for i := range n.Attr {
			if usefulattrs[n.Attr[i].Key] {
				newAttr = append(newAttr, n.Attr[i])
			}
		}
		n.Attr = newAttr
	}

L:
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			switch c.Data {
			case "svg":
				c.FirstChild = nil
				c.LastChild = nil
				continue L
			case "script", "style", "link", "noscript", "template", "iframe":
				defer n.RemoveChild(c)
				continue L
			}
		}
		distillPipeline(c)
	}
}

func trimWhitespaceAndRemoveEmptyTags(n *html.Node) {
	if n == nil {
		return
	}

	// Trim whitespace in text nodes
	if n.Type == html.TextNode {
		n.Data = strings.TrimSpace(n.Data)
	}

	// Process children
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		trimWhitespaceAndRemoveEmptyTags(c)

		// Remove empty tags, excluding self-closing tags
		if c.Type == html.ElementNode && c.FirstChild == nil && !isSelfClosingTag(c.Data) {
			n.RemoveChild(c)
		}
	}
}

var selfClosingTags = map[string]bool{
	"area":  true,
	"base":  true,
	"br":    true,
	"col":   true,
	"embed": true,
	"hr":    true,
	"img":   true,
	"input": true,
	"link":  true,
	"wbr":   true,
	"meta":  true,
}

func isSelfClosingTag(tag string) bool {
	return selfClosingTags[tag]
}

func removeSelfClosingTagsWithoutAttr(n *html.Node) {
	if n == nil {
		return
	}

	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		removeSelfClosingTagsWithoutAttr(c)

		if c.Type == html.ElementNode && isSelfClosingTag(c.Data) && len(c.Attr) == 0 {
			n.RemoveChild(c)
		}
	}
}

// Clean takes an HTML string, parses it into an HTML document, runs the distillPipeline
// function on the document to remove unnecessary elements and attributes, and then
// renders the cleaned HTML back to a string.
//
// If there is an error parsing or rendering the HTML, an error is returned.
func Clean(s string) (string, error) {
	htmlnode, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", err
	}
	distillPipeline(htmlnode)
	trimWhitespaceAndRemoveEmptyTags(htmlnode)
	removeSelfClosingTagsWithoutAttr(htmlnode)

	var sb strings.Builder
	err = html.Render(&sb, htmlnode)
	if err != nil {
		return "", err
	}

	return sb.String(), nil
}

// ExtractText takes an HTML string and returns the extracted text content.
// It removes all HTML tags and returns only the text nodes.
func ExtractText(s string) (string, error) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	var extractTextRecursive func(*html.Node)
	extractTextRecursive = func(n *html.Node) {
		if n == nil {
			return
		}
		if n.Type == html.TextNode {
			buf.WriteString(strings.TrimSpace(n.Data))
			buf.WriteString("\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractTextRecursive(c)
		}
	}

	extractTextRecursive(doc)
	return strings.TrimSpace(buf.String()), nil
}
