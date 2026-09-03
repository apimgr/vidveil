// SPDX-License-Identifier: MIT
// Package httputil provides HTTP to text conversion for terminal output
// See AI.md PART 14
package httputil

import (
	"golang.org/x/net/html"
	"strings"
)

// HTML2TextConverter converts rendered HTML to beautiful terminal text
// This is a custom Go function per AI.md - NOT a library wrapper
func HTML2TextConverter(htmlContent string, width int) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return stripTags(htmlContent)
	}

	var buf strings.Builder
	convertNode(&buf, doc, width, 0)
	return buf.String()
}

// convertNode recursively converts HTML nodes to formatted text
func convertNode(buf *strings.Builder, n *html.Node, width, indent int) {
	switch n.Type {
	case html.ElementNode:
		switch n.Data {
		case "h1":
			text := getTextContent(n)
			line := strings.Repeat("═", width)
			buf.WriteString(line + "\n")
			buf.WriteString(centerText(strings.ToUpper(text), width) + "\n")
			buf.WriteString(line + "\n\n")
		case "h2":
			text := getTextContent(n)
			buf.WriteString("─── " + text + " ───\n\n")
		case "h3":
			text := getTextContent(n)
			buf.WriteString("► " + text + "\n\n")
		case "p":
			text := getTextContent(n)
			buf.WriteString(wordWrap(text, width-indent) + "\n\n")
		case "ul":
			convertList(buf, n, width, indent, false)
		case "ol":
			convertList(buf, n, width, indent, true)
		case "a":
			text := getTextContent(n)
			href := getAttr(n, "href")
			if href != "" {
				buf.WriteString(text + " [" + href + "]")
			} else {
				buf.WriteString(text)
			}
		case "strong", "b":
			buf.WriteString("*" + getTextContent(n) + "*")
		case "em", "i":
			buf.WriteString("_" + getTextContent(n) + "_")
		case "code":
			buf.WriteString("`" + getTextContent(n) + "`")
		case "pre":
			text := getTextContent(n)
			for _, line := range strings.Split(text, "\n") {
				buf.WriteString("    " + line + "\n")
			}
			buf.WriteString("\n")
		case "table":
			convertTable(buf, n, width)
		case "hr":
			buf.WriteString(strings.Repeat("─", width) + "\n\n")
		case "blockquote":
			text := getTextContent(n)
			for _, line := range strings.Split(wordWrap(text, width-4), "\n") {
				buf.WriteString("│ " + line + "\n")
			}
			buf.WriteString("\n")
		case "br":
			buf.WriteString("\n")
		case "form", "input", "button", "select", "textarea", "script", "style", "noscript":
			// Skip non-interactive elements per AI.md PART 14
			return
		default:
			// Recurse into children
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				convertNode(buf, c, width, indent)
			}
		}
	case html.TextNode:
		text := strings.TrimSpace(n.Data)
		if text != "" {
			buf.WriteString(text)
		}
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertNode(buf, c, width, indent)
		}
	}
}

// convertList converts ul/ol to text with bullets/numbers
func convertList(buf *strings.Builder, n *html.Node, width, indent int, ordered bool) {
	num := 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "li" {
			text := getTextContent(c)
			prefix := "  • "
			if ordered {
				prefix = "  " + string(rune('0'+num)) + ". "
				num++
			}
			wrapped := wordWrap(text, width-indent-len(prefix))
			lines := strings.Split(wrapped, "\n")
			for i, line := range lines {
				if i == 0 {
					buf.WriteString(prefix + line + "\n")
				} else {
					buf.WriteString(strings.Repeat(" ", len(prefix)) + line + "\n")
				}
			}
		}
	}
	buf.WriteString("\n")
}

// convertTable converts HTML table to ASCII table
func convertTable(buf *strings.Builder, n *html.Node, width int) {
	var rows [][]string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "tr" || c.Data == "thead" || c.Data == "tbody") {
			row := extractTableRow(c)
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
	}

	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	cols := len(rows[0])
	colWidths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if i < cols && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	// Draw table
	for i, row := range rows {
		// Top border for first row
		if i == 0 {
			buf.WriteString("┌")
			for j, w := range colWidths {
				buf.WriteString(strings.Repeat("─", w+2))
				if j < len(colWidths)-1 {
					buf.WriteString("┬")
				}
			}
			buf.WriteString("┐\n")
		}

		// Row content
		buf.WriteString("│")
		for j, cell := range row {
			buf.WriteString(" " + cell + strings.Repeat(" ", colWidths[j]-len(cell)+1) + "│")
		}
		buf.WriteString("\n")

		// Separator after header
		if i == 0 && len(rows) > 1 {
			buf.WriteString("├")
			for j, w := range colWidths {
				buf.WriteString(strings.Repeat("─", w+2))
				if j < len(colWidths)-1 {
					buf.WriteString("┼")
				}
			}
			buf.WriteString("┤\n")
		}
	}

	// Bottom border
	buf.WriteString("└")
	for j, w := range colWidths {
		buf.WriteString(strings.Repeat("─", w+2))
		if j < len(colWidths)-1 {
			buf.WriteString("┴")
		}
	}
	buf.WriteString("┘\n\n")
}

// extractTableRow extracts text from table row
func extractTableRow(n *html.Node) []string {
	var cells []string
	var extractCells func(*html.Node)
	extractCells = func(node *html.Node) {
		if node.Type == html.ElementNode && (node.Data == "td" || node.Data == "th") {
			cells = append(cells, getTextContent(node))
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extractCells(c)
		}
	}
	extractCells(n)
	return cells
}

// getTextContent recursively gets text content from node
func getTextContent(n *html.Node) string {
	var buf strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(buf.String())
}

// getAttr gets attribute value from node
func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// centerText centers text in given width
func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}

// wordWrap wraps text to given width
func wordWrap(text string, width int) string {
	if width <= 0 {
		width = 80
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}
		}
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

// stripTags is a basic fallback for malformed HTML
func stripTags(html string) string {
	return strings.TrimSpace(html)
}
