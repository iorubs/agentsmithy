// Package htmltext converts HTML into readable text for LLM
// consumption. Inline formatting (bold, italic, links, images) is
// stripped; semantic structure (headings, lists, code blocks) is
// preserved as Markdown. Tables are rendered as tab-separated rows.
//
// This is a vendored copy of mcpsmithy's HTML→text conversion used
// by the scrape source, kept in agentsmithy so the web_scrape skill
// has no dependency on mcpsmithy.
package htmltext

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	reExcessNewlines = regexp.MustCompile(`\n{3,}`)
	reStripTags      = regexp.MustCompile(`<[^>]+>`)

	noiseElements = map[atom.Atom]bool{
		atom.Script: true, atom.Style: true, atom.Noscript: true,
		atom.Nav: true, atom.Footer: true, atom.Header: true,
		atom.Svg: true, atom.Iframe: true,
	}
	headingMarker = map[atom.Atom]string{
		atom.H1: "# ", atom.H2: "## ", atom.H3: "### ",
		atom.H4: "#### ", atom.H5: "##### ", atom.H6: "###### ",
	}
)

// Convert returns a readable text/markdown rendering of rawHTML.
// On parse failure it falls back to a tag-strip + whitespace trim
// so the caller still gets something useful.
func Convert(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return strings.TrimSpace(reStripTags.ReplaceAllString(rawHTML, " "))
	}
	var b strings.Builder
	walkNode(&b, doc, 0)
	return strings.TrimSpace(reExcessNewlines.ReplaceAllString(b.String(), "\n\n"))
}

func walkNode(b *strings.Builder, n *html.Node, depth int) {
	if n.Type == html.TextNode {
		b.WriteString(collapseWhitespace(n.Data))
		return
	}
	if n.Type != html.ElementNode {
		walkChildren(b, n, depth)
		return
	}
	if noiseElements[n.DataAtom] {
		return
	}
	if marker, ok := headingMarker[n.DataAtom]; ok {
		b.WriteString("\n\n" + marker)
		walkChildren(b, n, depth)
		b.WriteString("\n\n")
		return
	}
	switch n.DataAtom {
	case atom.P:
		b.WriteString("\n\n")
		walkChildren(b, n, depth)
		b.WriteString("\n\n")
	case atom.Br:
		b.WriteString("\n")
	case atom.Hr:
		b.WriteString("\n\n---\n\n")
	case atom.Strong, atom.B, atom.Em, atom.I, atom.Span, atom.A:
		walkChildren(b, n, depth)
	case atom.Img:
		// no value for LLM text; skip
	case atom.Code:
		if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
			b.WriteString("```" + extractLang(getAttr(n, "class")) + "\n")
			writeRawText(b, n)
			b.WriteString("\n```")
		} else {
			b.WriteString("`")
			walkChildren(b, n, depth)
			b.WriteString("`")
		}
	case atom.Pre:
		b.WriteString("\n\n")
		if c := firstElementChild(n); c != nil && c.DataAtom == atom.Code {
			walkChildren(b, n, depth)
		} else {
			b.WriteString("```\n")
			writeRawText(b, n)
			b.WriteString("\n```")
		}
		b.WriteString("\n\n")
	case atom.Ul, atom.Ol:
		b.WriteString("\n")
		walkChildren(b, n, depth+1)
		b.WriteString("\n")
	case atom.Li:
		b.WriteString(strings.Repeat("  ", depth-1) + "- ")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Tr:
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Td, atom.Th:
		walkChildren(b, n, depth)
		b.WriteString("\t")
	case atom.Dt:
		b.WriteString("\n")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	case atom.Dd:
		b.WriteString("  ")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	default:
		b.WriteString("\n")
		walkChildren(b, n, depth)
		b.WriteString("\n")
	}
}

func walkChildren(b *strings.Builder, n *html.Node, depth int) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkNode(b, c, depth)
	}
}

func writeRawText(b *strings.Builder, n *html.Node) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		writeRawText(b, c)
	}
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractLang(class string) string {
	for part := range strings.FieldsSeq(class) {
		if after, ok := strings.CutPrefix(part, "language-"); ok {
			return after
		}
		if after, ok := strings.CutPrefix(part, "lang-"); ok {
			return after
		}
	}
	return ""
}

func firstElementChild(n *html.Node) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return c
		}
	}
	return nil
}

// collapseWhitespace reduces runs of whitespace to a single space,
// trimming leading whitespace before any content has been written.
func collapseWhitespace(s string) string {
	var b strings.Builder
	lastWasSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !lastWasSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			lastWasSpace = true
			continue
		}
		b.WriteRune(r)
		lastWasSpace = false
	}
	return b.String()
}
