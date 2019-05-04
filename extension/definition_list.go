package extension

import (
	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type definitionListParser struct {
}

var defaultDefinitionListParser = &definitionListParser{}

// NewDefinitionListParser return a new parser.BlockParser that
// can parse PHP Markdown Extra Definition lists.
func NewDefinitionListParser() parser.BlockParser {
	return defaultDefinitionListParser
}

func (b *definitionListParser) Open(parent gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	if _, ok := parent.(*ast.DefinitionList); ok {
		return nil, parser.NoChildren
	}
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if line[pos] != ':' {
		return nil, parser.NoChildren
	}

	last := parent.LastChild()
	// need 1 or more spaces after ':'
	w, _ := util.IndentWidth(line[pos+1:], pos+1)
	if w < 1 {
		return nil, parser.NoChildren
	}
	if w >= 8 { // starts with indented code
		w = 5
	}
	w += pos + 1 /* 1 = ':' */

	para, lastIsParagraph := last.(*gast.Paragraph)
	var list *ast.DefinitionList
	var ok bool
	if lastIsParagraph {
		list, ok = last.PreviousSibling().(*ast.DefinitionList)
		if ok { // is not first item
			list.Offset = w
			list.TemporaryParagraph = para
		} else { // is first item
			list = ast.NewDefinitionList(w, para)
		}
	} else if list, ok = last.(*ast.DefinitionList); ok { // multiple description
		list.Offset = w
		list.TemporaryParagraph = nil
	} else {
		return nil, parser.NoChildren
	}

	return list, parser.HasChildren
}

func (b *definitionListParser) Continue(node gast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	list, _ := node.(*ast.DefinitionList)
	w, _ := util.IndentWidth(line, reader.LineOffset())
	if w < list.Offset {
		return parser.Close
	}
	pos, padding := util.IndentPosition(line, reader.LineOffset(), list.Offset)
	reader.AdvanceAndSetPadding(pos, padding)
	return parser.Continue | parser.HasChildren
}

func (b *definitionListParser) Close(node gast.Node, reader text.Reader, pc parser.Context) {
	// nothing to do
}

func (b *definitionListParser) CanInterruptParagraph() bool {
	return true
}

func (b *definitionListParser) CanAcceptIndentedLine() bool {
	return false
}

type definitionDescriptionParser struct {
}

var defaultDefinitionDescriptionParser = &definitionDescriptionParser{}

// NewDefinitionDescriptionParser return a new parser.BlockParser that
// can parse definition description starts with ':'.
func NewDefinitionDescriptionParser() parser.BlockParser {
	return defaultDefinitionDescriptionParser
}

func (b *definitionDescriptionParser) Open(parent gast.Node, reader text.Reader, pc parser.Context) (gast.Node, parser.State) {
	line, _ := reader.PeekLine()
	pos := pc.BlockOffset()
	if line[pos] != ':' {
		return nil, parser.NoChildren
	}
	list, _ := parent.(*ast.DefinitionList)
	para := list.TemporaryParagraph
	list.TemporaryParagraph = nil
	if para != nil {
		lines := para.Lines()
		l := lines.Len()
		for i := 0; i < l; i++ {
			term := ast.NewDefinitionTerm()
			segment := lines.At(i)
			term.Lines().Append(segment.TrimRightSpace(reader.Source()))
			list.AppendChild(list, term)
		}
		para.Parent().RemoveChild(para.Parent(), para)
	}
	cpos, padding := util.IndentPosition(line[pos+1:], pos+1, list.Offset-pos-1)
	reader.AdvanceAndSetPadding(cpos, padding)

	return ast.NewDefinitionDescription(), parser.HasChildren
}

func (b *definitionDescriptionParser) Continue(node gast.Node, reader text.Reader, pc parser.Context) parser.State {
	// definitionListParser detects end of the description.
	// so this method will never be called.
	return parser.Continue | parser.HasChildren
}

func (b *definitionDescriptionParser) Close(node gast.Node, reader text.Reader, pc parser.Context) {
	desc := node.(*ast.DefinitionDescription)
	desc.IsTight = !desc.HasBlankPreviousLines()
	if desc.IsTight {
		for gc := desc.FirstChild(); gc != nil; gc = gc.NextSibling() {
			paragraph, ok := gc.(*gast.Paragraph)
			if ok {
				textBlock := gast.NewTextBlock()
				textBlock.SetLines(paragraph.Lines())
				desc.ReplaceChild(desc, paragraph, textBlock)
			}
		}
	}
}

func (b *definitionDescriptionParser) CanInterruptParagraph() bool {
	return true
}

func (b *definitionDescriptionParser) CanAcceptIndentedLine() bool {
	return false
}

// DefinitionListHTMLRenderer is a renderer.NodeRenderer implementation that
// renders DefinitionList nodes.
type DefinitionListHTMLRenderer struct {
	html.Config
}

// NewDefinitionListHTMLRenderer returns a new DefinitionListHTMLRenderer.
func NewDefinitionListHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &DefinitionListHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *DefinitionListHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDefinitionList, r.renderDefinitionList)
	reg.Register(ast.KindDefinitionTerm, r.renderDefinitionTerm)
	reg.Register(ast.KindDefinitionDescription, r.renderDefinitionDescription)
}

func (r *DefinitionListHTMLRenderer) renderDefinitionList(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		w.WriteString("<dl>\n")
	} else {
		w.WriteString("</dl>\n")
	}
	return gast.WalkContinue, nil
}

func (r *DefinitionListHTMLRenderer) renderDefinitionTerm(w util.BufWriter, source []byte, n gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		w.WriteString("<dt>")
	} else {
		w.WriteString("</dt>\n")
	}
	return gast.WalkContinue, nil
}

func (r *DefinitionListHTMLRenderer) renderDefinitionDescription(w util.BufWriter, source []byte, node gast.Node, entering bool) (gast.WalkStatus, error) {
	if entering {
		n := node.(*ast.DefinitionDescription)
		if n.IsTight {
			w.WriteString("<dd>")
		} else {
			w.WriteString("<dd>\n")
		}
	} else {
		w.WriteString("</dd>\n")
	}
	return gast.WalkContinue, nil
}

type definitionList struct {
}

// DefinitionList is an extension that allow you to use PHP Markdown Extra Definition lists.
var DefinitionList = &definitionList{}

func (e *definitionList) Extend(m goldmark.Markdown) {
	m.Parser().AddOption(parser.WithBlockParsers(
		util.Prioritized(NewDefinitionListParser(), 101),
		util.Prioritized(NewDefinitionDescriptionParser(), 102),
	))
	m.Renderer().AddOption(renderer.WithNodeRenderers(
		util.Prioritized(NewDefinitionListHTMLRenderer(), 500),
	))
}