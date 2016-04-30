// Copyright 2015 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helpers implements general utility functions that work with
// and on content.  The helper functions defined here lay down the
// foundation of how Hugo works with files and filepaths, and perform
// string operations on content.
package helpers

import (
	"bytes"
	"html/template"
	"os/exec"
	"unicode"
	"unicode/utf8"

	"github.com/miekg/mmark"
	"github.com/mitchellh/mapstructure"
	"github.com/russross/blackfriday"
	"github.com/spf13/cast"
	bp "github.com/spf13/hugo/bufferpool"
	jww "github.com/spf13/jwalterweatherman"
	"github.com/spf13/viper"

	"strings"
	"sync"
)

// SummaryLength is the length of the summary that Hugo extracts from a content.
var SummaryLength = 70

// SummaryDivider denotes where content summarization should end. The default is "<!--more-->".
var SummaryDivider = []byte("<!--more-->")

var summaryDividerAndNewLines = []byte("<!--more-->\n\n")

// Blackfriday holds configuration values for Blackfriday rendering.
type Blackfriday struct {
	Smartypants                      bool
	AngledQuotes                     bool
	Fractions                        bool
	HrefTargetBlank                  bool
	SmartDashes                      bool
	LatexDashes                      bool
	TaskLists                        bool
	PlainIDAnchors                   bool
	SourceRelativeLinksEval          bool
	SourceRelativeLinksProjectFolder string
	Extensions                       []string
	ExtensionsMask                   []string
}

// NewBlackfriday creates a new Blackfriday filled with site config or some sane defaults.
func NewBlackfriday(c ConfigProvider) *Blackfriday {
	combinedParam := map[string]interface{}{
		"smartypants":                      true,
		"angledQuotes":                     false,
		"fractions":                        true,
		"hrefTargetBlank":                  false,
		"smartDashes":                      true,
		"latexDashes":                      true,
		"plainIDAnchors":                   true,
		"taskLists":                        true,
		"sourceRelativeLinks":              false,
		"sourceRelativeLinksProjectFolder": "/docs/content",
	}

	siteParam := c.GetStringMap("blackfriday")
	if siteParam != nil {
		siteConfig := cast.ToStringMap(siteParam)

		for key, value := range siteConfig {
			combinedParam[key] = value
		}
	}

	combinedConfig := &Blackfriday{}
	if err := mapstructure.Decode(combinedParam, combinedConfig); err != nil {
		jww.FATAL.Printf("Failed to get site rendering config\n%s", err.Error())
	}

	return combinedConfig
}

var blackfridayExtensionMap = map[string]int{
	"noIntraEmphasis":        blackfriday.EXTENSION_NO_INTRA_EMPHASIS,
	"tables":                 blackfriday.EXTENSION_TABLES,
	"fencedCode":             blackfriday.EXTENSION_FENCED_CODE,
	"autolink":               blackfriday.EXTENSION_AUTOLINK,
	"strikethrough":          blackfriday.EXTENSION_STRIKETHROUGH,
	"laxHtmlBlocks":          blackfriday.EXTENSION_LAX_HTML_BLOCKS,
	"spaceHeaders":           blackfriday.EXTENSION_SPACE_HEADERS,
	"hardLineBreak":          blackfriday.EXTENSION_HARD_LINE_BREAK,
	"tabSizeEight":           blackfriday.EXTENSION_TAB_SIZE_EIGHT,
	"footnotes":              blackfriday.EXTENSION_FOOTNOTES,
	"noEmptyLineBeforeBlock": blackfriday.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK,
	"headerIds":              blackfriday.EXTENSION_HEADER_IDS,
	"titleblock":             blackfriday.EXTENSION_TITLEBLOCK,
	"autoHeaderIds":          blackfriday.EXTENSION_AUTO_HEADER_IDS,
	"backslashLineBreak":     blackfriday.EXTENSION_BACKSLASH_LINE_BREAK,
	"definitionLists":        blackfriday.EXTENSION_DEFINITION_LISTS,
}

// Mmark holds configuration values for Mmark rendering.
type Mmark struct {
	HrefTargetBlank bool
	PlainIDAnchors  bool
	Extensions      []string
	ExtensionsMask  []string
}

// NewMmark creates a new Mmark filled with site config or some sane defaults.
func NewMmark() *Mmark {
	combinedParam := map[string]interface{}{
		"hrefTargetBlank": false,
		"plainIDAnchors":  true,
	}

	siteParam := viper.GetStringMap("mmark")
	if siteParam != nil {
		siteConfig := cast.ToStringMap(siteParam)

		for key, value := range siteConfig {
			combinedParam[key] = value
		}
	}

	combinedConfig := &Mmark{}
	if err := mapstructure.Decode(combinedParam, combinedConfig); err != nil {
		jww.FATAL.Printf("Failed to get site rendering config\n%s", err.Error())
	}

	return combinedConfig
}

var mmarkExtensionMap = map[string]int{
	"abbreviations":          mmark.EXTENSION_ABBREVIATIONS,
	"autoHeaderIds":          mmark.EXTENSION_AUTO_HEADER_IDS,
	"autolink":               mmark.EXTENSION_AUTOLINK,
	"citation":               mmark.EXTENSION_CITATION,
	"exampleLists":           mmark.EXTENSION_EXAMPLE_LISTS,
	"fencedCode":             mmark.EXTENSION_FENCED_CODE,
	"footnotes":              mmark.EXTENSION_FOOTNOTES,
	"hardLineBreak":          mmark.EXTENSION_HARD_LINE_BREAK,
	"headerIds":              mmark.EXTENSION_HEADER_IDS,
	"include":                mmark.EXTENSION_INCLUDE,
	"inlineAttr":             mmark.EXTENSION_INLINE_ATTR,
	"laxHtmlBlocks":          mmark.EXTENSION_LAX_HTML_BLOCKS,
	"math":                   mmark.EXTENSION_MATH,
	"matter":                 mmark.EXTENSION_MATTER,
	"noEmptyLineBeforeBlock": mmark.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK,
	"parts":                  mmark.EXTENSION_PARTS,
	"quotes":                 mmark.EXTENSION_QUOTES,
	"shortRef":               mmark.EXTENSION_SHORT_REF,
	"spaceHeaders":           mmark.EXTENSION_SPACE_HEADERS,
	"tables":                 mmark.EXTENSION_TABLES,
	"titleblockToml":         mmark.EXTENSION_TITLEBLOCK_TOML,
	"uniqueHeaderIds":        mmark.EXTENSION_UNIQUE_HEADER_IDS,
	"backslashLineBreak":     mmark.EXTENSION_BACKSLASH_LINE_BREAK,
	"rfc7328":                mmark.EXTENSION_RFC7328,
}

var stripHTMLReplacer = strings.NewReplacer("\n", " ", "</p>", "\n", "<br>", "\n", "<br />", "\n")

// StripHTML accepts a string, strips out all HTML tags and returns it.
func StripHTML(s string) string {

	// Shortcut strings with no tags in them
	if !strings.ContainsAny(s, "<>") {
		return s
	}
	s = stripHTMLReplacer.Replace(s)

	// Walk through the string removing all tags
	b := bp.GetBuffer()
	defer bp.PutBuffer(b)
	var inTag, isSpace, wasSpace bool
	for _, r := range s {
		if !inTag {
			isSpace = false
		}

		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case unicode.IsSpace(r):
			isSpace = true
			fallthrough
		default:
			if !inTag && (!isSpace || (isSpace && !wasSpace)) {
				b.WriteRune(r)
			}
		}

		wasSpace = isSpace

	}
	return b.String()
}

// stripEmptyNav strips out empty <nav> tags from content.
func stripEmptyNav(in []byte) []byte {
	return bytes.Replace(in, []byte("<nav>\n</nav>\n\n"), []byte(``), -1)
}

// BytesToHTML converts bytes to type template.HTML.
func BytesToHTML(b []byte) template.HTML {
	return template.HTML(string(b))
}

// getHTMLRenderer creates a new Blackfriday HTML Renderer with the given configuration.
func getHTMLRenderer(defaultFlags int, ctx *RenderingContext) blackfriday.Renderer {
	renderParameters := blackfriday.HtmlRendererParameters{
		FootnoteAnchorPrefix:       viper.GetString("FootnoteAnchorPrefix"),
		FootnoteReturnLinkContents: viper.GetString("FootnoteReturnLinkContents"),
	}

	b := len(ctx.DocumentID) != 0

	if b && !ctx.getConfig().PlainIDAnchors {
		renderParameters.FootnoteAnchorPrefix = ctx.DocumentID + ":" + renderParameters.FootnoteAnchorPrefix
		renderParameters.HeaderIDSuffix = ":" + ctx.DocumentID
	}

	htmlFlags := defaultFlags
	htmlFlags |= blackfriday.HTML_USE_XHTML
	htmlFlags |= blackfriday.HTML_FOOTNOTE_RETURN_LINKS

	if ctx.getConfig().Smartypants {
		htmlFlags |= blackfriday.HTML_USE_SMARTYPANTS
	}

	if ctx.getConfig().AngledQuotes {
		htmlFlags |= blackfriday.HTML_SMARTYPANTS_ANGLED_QUOTES
	}

	if ctx.getConfig().Fractions {
		htmlFlags |= blackfriday.HTML_SMARTYPANTS_FRACTIONS
	}

	if ctx.getConfig().HrefTargetBlank {
		htmlFlags |= blackfriday.HTML_HREF_TARGET_BLANK
	}

	if ctx.getConfig().SmartDashes {
		htmlFlags |= blackfriday.HTML_SMARTYPANTS_DASHES
	}

	if ctx.getConfig().LatexDashes {
		htmlFlags |= blackfriday.HTML_SMARTYPANTS_LATEX_DASHES
	}

	return &HugoHTMLRenderer{
		RenderingContext: ctx,
		Renderer:         blackfriday.HtmlRendererWithParameters(htmlFlags, "", "", renderParameters),
	}
}

func getMarkdownExtensions(ctx *RenderingContext) int {
	// Default Blackfriday common extensions
	commonExtensions := 0 |
		blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS |
		blackfriday.EXTENSION_HEADER_IDS |
		blackfriday.EXTENSION_BACKSLASH_LINE_BREAK |
		blackfriday.EXTENSION_DEFINITION_LISTS

	// Extra Blackfriday extensions that Hugo enables by default
	flags := commonExtensions |
		blackfriday.EXTENSION_AUTO_HEADER_IDS |
		blackfriday.EXTENSION_FOOTNOTES

	for _, extension := range ctx.getConfig().Extensions {
		if flag, ok := blackfridayExtensionMap[extension]; ok {
			flags |= flag
		}
	}
	for _, extension := range ctx.getConfig().ExtensionsMask {
		if flag, ok := blackfridayExtensionMap[extension]; ok {
			flags &= ^flag
		}
	}
	return flags
}

func markdownRender(ctx *RenderingContext) []byte {
	if ctx.RenderTOC {
		return blackfriday.Markdown(ctx.Content,
			getHTMLRenderer(blackfriday.HTML_TOC, ctx),
			getMarkdownExtensions(ctx))
	}
	return blackfriday.Markdown(ctx.Content, getHTMLRenderer(0, ctx),
		getMarkdownExtensions(ctx))
}

// getMmarkHTMLRenderer creates a new mmark HTML Renderer with the given configuration.
func getMmarkHTMLRenderer(defaultFlags int, ctx *RenderingContext) mmark.Renderer {
	renderParameters := mmark.HtmlRendererParameters{
		FootnoteAnchorPrefix:       viper.GetString("FootnoteAnchorPrefix"),
		FootnoteReturnLinkContents: viper.GetString("FootnoteReturnLinkContents"),
	}

	b := len(ctx.DocumentID) != 0

	if b && !ctx.getMmarkConfig().PlainIDAnchors {
		renderParameters.FootnoteAnchorPrefix = ctx.DocumentID + ":" + renderParameters.FootnoteAnchorPrefix
	}

	htmlFlags := defaultFlags
	htmlFlags |= mmark.HTML_FOOTNOTE_RETURN_LINKS

	return &HugoMmarkHTMLRenderer{
		mmark.HtmlRendererWithParameters(htmlFlags, "", "", renderParameters),
	}
}

func getMmarkExtensions(ctx *RenderingContext) int {
	// Default mmark common extensions
	commonExtensions := 0 |
		mmark.EXTENSION_TABLES |
		mmark.EXTENSION_FENCED_CODE |
		mmark.EXTENSION_AUTOLINK |
		mmark.EXTENSION_SPACE_HEADERS |
		mmark.EXTENSION_HEADER_IDS |
		mmark.EXTENSION_ABBREVIATIONS |
		mmark.EXTENSION_NO_EMPTY_LINE_BEFORE_BLOCK | // CommonMark
		mmark.EXTENSION_BACKSLASH_LINE_BREAK // CommonMark

	// Default mmark XML extensions
	commonXmlExtensions := commonExtensions |
		mmark.EXTENSION_UNIQUE_HEADER_IDS |
		mmark.EXTENSION_AUTO_HEADER_IDS |
		mmark.EXTENSION_INLINE_ATTR |
		mmark.EXTENSION_QUOTES |
		mmark.EXTENSION_MATTER |
		mmark.EXTENSION_CITATION |
		mmark.EXTENSION_EXAMPLE_LISTS |
		mmark.EXTENSION_SHORT_REF

	// Extra mmark extension(s) that Hugo enables by default
	flags := commonXmlExtensions |
		mmark.EXTENSION_FOOTNOTES

	for _, extension := range ctx.getMmarkConfig().Extensions {
		if flag, ok := mmarkExtensionMap[extension]; ok {
			flags |= flag
		}
	}
	for _, extension := range ctx.getMmarkConfig().ExtensionsMask {
		if flag, ok := mmarkExtensionMap[extension]; ok {
			flags &= ^flag
		}
	}
	return flags
}

func mmarkRender(ctx *RenderingContext) []byte {
	return mmark.Parse(ctx.Content, getMmarkHTMLRenderer(0, ctx),
		getMmarkExtensions(ctx)).Bytes()
}

// ExtractTOC extracts Table of Contents from content.
func ExtractTOC(content []byte) (newcontent []byte, toc []byte) {
	origContent := make([]byte, len(content))
	copy(origContent, content)
	first := []byte(`<nav>
<ul>`)

	last := []byte(`</ul>
</nav>`)

	replacement := []byte(`<nav id="TableOfContents">
<ul>`)

	startOfTOC := bytes.Index(content, first)

	peekEnd := len(content)
	if peekEnd > 70+startOfTOC {
		peekEnd = 70 + startOfTOC
	}

	if startOfTOC < 0 {
		return stripEmptyNav(content), toc
	}
	// Need to peek ahead to see if this nav element is actually the right one.
	correctNav := bytes.Index(content[startOfTOC:peekEnd], []byte(`<li><a href="#`))
	if correctNav < 0 { // no match found
		return content, toc
	}
	lengthOfTOC := bytes.Index(content[startOfTOC:], last) + len(last)
	endOfTOC := startOfTOC + lengthOfTOC

	newcontent = append(content[:startOfTOC], content[endOfTOC:]...)
	toc = append(replacement, origContent[startOfTOC+len(first):endOfTOC]...)
	return
}

// RenderingContext holds contextual information, like content and configuration,
// for a given content rendering.
type RenderingContext struct {
	Content        []byte
	PageFmt        string
	DocumentID     string
	Config         *Blackfriday
	MmarkConfig    *Mmark // TODO(anthonyfok) Refactor
	RenderTOC      bool
	FileResolver   FileResolverFunc
	LinkResolver   LinkResolverFunc
	ConfigProvider ConfigProvider
	configInit     sync.Once
}

func newViperProvidedRenderingContext() *RenderingContext {
	return &RenderingContext{ConfigProvider: viper.GetViper()}
}

func (c *RenderingContext) getConfig() *Blackfriday {
	c.configInit.Do(func() {
		if c.Config == nil {
			c.Config = NewBlackfriday(c.ConfigProvider)
		}
	})
	return c.Config
}

// TODO(anthonyfok) Refactor
func (c *RenderingContext) getMmarkConfig() *Mmark {
	c.configInit.Do(func() {
		if c.MmarkConfig == nil {
			c.MmarkConfig = NewMmark()
		}
	})
	return c.MmarkConfig
}

// RenderBytes renders a []byte.
func RenderBytes(ctx *RenderingContext) []byte {
	switch ctx.PageFmt {
	default:
		return markdownRender(ctx)
	case "markdown":
		return markdownRender(ctx)
	case "asciidoc":
		return getAsciidocContent(ctx.Content)
	case "mmark":
		return mmarkRender(ctx)
	case "rst":
		return getRstContent(ctx.Content)
	}
}

// TotalWords counts instance of one or more consecutive white space
// characters, as defined by unicode.IsSpace, in s.
// This is a cheaper way of word counting than the obvious len(strings.Fields(s)).
func TotalWords(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		wasInWord := inWord
		inWord = !unicode.IsSpace(r)
		if inWord && !wasInWord {
			n++
		}
	}
	return n
}

// Old implementation only kept for benchmark comparison.
// TODO(bep) remove
func totalWordsOld(s string) int {
	return len(strings.Fields(s))
}

// WordCount takes content and returns a map of words and count of each word.
func WordCount(s string) map[string]int {
	m := make(map[string]int)
	for _, f := range strings.Fields(s) {
		m[f]++
	}

	return m
}

// TruncateWordsByRune truncates words by runes.
func TruncateWordsByRune(words []string, max int) (string, bool) {
	count := 0
	for index, word := range words {
		if count >= max {
			return strings.Join(words[:index], " "), true
		}
		runeCount := utf8.RuneCountInString(word)
		if len(word) == runeCount {
			count++
		} else if count+runeCount < max {
			count += runeCount
		} else {
			for ri := range word {
				if count >= max {
					truncatedWords := append(words[:index], word[:ri])
					return strings.Join(truncatedWords, " "), true
				}
				count++
			}
		}
	}

	return strings.Join(words, " "), false
}

// TruncateWordsToWholeSentence takes content and truncates to whole sentence
// limited by max number of words. It also returns whether it is truncated.
func TruncateWordsToWholeSentence(s string, max int) (string, bool) {

	var (
		wordCount     = 0
		lastWordIndex = -1
	)

	for i, r := range s {
		if unicode.IsSpace(r) {
			wordCount++
			lastWordIndex = i

			if wordCount >= max {
				break
			}

		}
	}

	if lastWordIndex == -1 {
		return s, false
	}

	endIndex := -1

	for j, r := range s[lastWordIndex:] {
		if isEndOfSentence(r) {
			endIndex = j + lastWordIndex + utf8.RuneLen(r)
			break
		}
	}

	if endIndex == -1 {
		return s, false
	}

	return strings.TrimSpace(s[:endIndex]), endIndex < len(s)
}

func isEndOfSentence(r rune) bool {
	return r == '.' || r == '?' || r == '!' || r == '"' || r == '\n'
}

// Kept only for benchmark.
func truncateWordsToWholeSentenceOld(content string, max int) (string, bool) {
	words := strings.Fields(content)

	if max >= len(words) {
		return strings.Join(words, " "), false
	}

	for counter, word := range words[max:] {
		if strings.HasSuffix(word, ".") ||
			strings.HasSuffix(word, "?") ||
			strings.HasSuffix(word, ".\"") ||
			strings.HasSuffix(word, "!") {
			upper := max + counter + 1
			return strings.Join(words[:upper], " "), (upper < len(words))
		}
	}

	return strings.Join(words[:max], " "), true
}

func getAsciidocExecPath() string {
	path, err := exec.LookPath("asciidoctor")
	if err != nil {
		path, err = exec.LookPath("asciidoc")
		if err != nil {
			return ""
		}
	}
	return path
}

// HasAsciidoc returns whether Asciidoctor or Asciidoc is installed on this computer.
func HasAsciidoc() bool {
	return getAsciidocExecPath() != ""
}

// getAsciidocContent calls asciidoctor or asciidoc as an external helper
// to convert AsciiDoc content to HTML.
func getAsciidocContent(content []byte) []byte {
	cleanContent := bytes.Replace(content, SummaryDivider, []byte(""), 1)

	path := getAsciidocExecPath()
	if path == "" {
		jww.ERROR.Println("asciidoctor / asciidoc not found in $PATH: Please install.\n",
			"                 Leaving AsciiDoc content unrendered.")
		return content
	}

	jww.INFO.Println("Rendering with", path, "...")
	cmd := exec.Command(path, "--no-header-footer", "--safe", "-")
	cmd.Stdin = bytes.NewReader(cleanContent)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		jww.ERROR.Println(err)
	}

	return out.Bytes()
}

// HasRst returns whether rst2html is installed on this computer.
func HasRst() bool {
	return getRstExecPath() != ""
}

func getRstExecPath() string {
	path, err := exec.LookPath("rst2html")
	if err != nil {
		path, err = exec.LookPath("rst2html.py")
		if err != nil {
			return ""
		}
	}
	return path
}

// getRstContent calls the Python script rst2html as an external helper
// to convert reStructuredText content to HTML.
func getRstContent(content []byte) []byte {
	cleanContent := bytes.Replace(content, SummaryDivider, []byte(""), 1)

	path := getRstExecPath()

	if path == "" {
		jww.ERROR.Println("rst2html / rst2html.py not found in $PATH: Please install.\n",
			"                 Leaving reStructuredText content unrendered.")
		return content

	}

	cmd := exec.Command(path, "--leave-comments")
	cmd.Stdin = bytes.NewReader(cleanContent)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		jww.ERROR.Println(err)
	}

	result := out.Bytes()

	// TODO(bep) check if rst2html has a body only option.
	bodyStart := bytes.Index(result, []byte("<body>\n"))
	bodyEnd := bytes.Index(result, []byte("\n</body>"))

	return result[bodyStart+7 : bodyEnd]
}
