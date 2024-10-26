package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexGames73/unioffice-free/color"
	"github.com/AlexGames73/unioffice-free/common"
	"github.com/AlexGames73/unioffice-free/document"
	"github.com/AlexGames73/unioffice-free/measurement"
	"github.com/AlexGames73/unioffice-free/schema/soo/wml"
)

func main() {

	// Check if directory is passed as argument
	if len(os.Args) < 2 {
		log.Fatal("Please provide a directory path")
	}

	if len(os.Args) < 3 {
		log.Fatal("Please provide a base assets path")
	}

	// Get the directory path from arguments
	dir := os.Args[1]

	// Get the base assets path from arguments
	assets_path := os.Args[2]

	doc := document.New()
	defer doc.Close()

	// Loop over all files in the directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil // Skip this file and continue
		}

		if info == nil {
			log.Printf("Nil info encountered for path %q\n", path)
			return nil // Skip if info is nil
		}

		// Check if it is a file (not a directory)
		if !info.IsDir() {
			log.Println("Processing file:", path)
			process_file(doc, path, assets_path)
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking the path %q: %v\n", dir, err)
	}

	doc.SaveToFile("output.docx")
}

func process_file(doc *document.Document, path string, assets_path string) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	// Tokenize the file
	tokens := Tokenize(string(data))
	tokenizer := NewTokenizer(tokens)

	var current_paragraph document.Paragraph = doc.AddParagraph()
	// Date is path without extension
	date := filepath.Base(path)
	current_paragraph.Properties().SetStyle("Heading1")
	current_paragraph.AddRun().AddText(date)
	current_paragraph = doc.AddParagraph()

	var current_hyperlink *document.HyperLink = nil
	for tokenizer.HasNext() {
		current := tokenizer.Pop()
		if current == nil {
			continue
		}
		// log.Println(current.String())

		if current.Type == Code {
			run := current_paragraph.AddRun()
			run.AddText(current.Value)
			run.Properties().SetFontFamily("Noto Sans Mono")
			run.Properties().Bold()
		} else if current.Type == Newline && tokenizer.Peek().Type == Text {
			current_paragraph = doc.AddParagraph()
		} else if current.Type == Heading && tokenizer.Peek().Type == Text {
			current_paragraph = doc.AddParagraph()
			current_paragraph.Properties().SetStyle("Heading2")
			run := current_paragraph.AddRun()
			run.AddText(tokenizer.Pop().Value)
			run.Properties().SetBold(true)
			current_paragraph = doc.AddParagraph()
		} else if current.Type == Bullet && tokenizer.Peek().Type == Text {
			run := current_paragraph.AddRun()
			run.AddText("- " + tokenizer.Pop().Value)
		} else if current.Type == ImageLinkStart && tokenizer.Peek().Type == Text {
			file_path := assets_path + "/" + tokenizer.Pop().Value
			img, err := common.ImageFromFile(file_path)
			if err != nil {
				log.Fatal(err)
			}
			// Load image from file
			ref, err := doc.AddImage(img)
			if err != nil {
				log.Fatal(err)
			}

			// Add image to paragraph
			anchored, err := current_paragraph.AddRun().AddDrawingAnchored(ref)
			if err != nil {
				log.Fatal(err)
			}

			scale := float64(img.Size.Y)/float64(img.Size.X)
			scaledHeight := measurement.Distance(scale * 5.0) * measurement.Inch
			anchored.SetSize(5 * measurement.Inch, scaledHeight)
			anchored.SetHAlignment(wml.WdST_AlignHCenter)
			anchored.SetOrigin(wml.WdST_RelFromHColumn, wml.WdST_RelFromVParagraph)
			anchored.SetTextWrapTopAndBottom()

			current_paragraph = doc.AddParagraph()

			tokenizer.Pop()
		} else if current.Type == LeftBracket && tokenizer.Peek().Type == Text && tokenizer.PeekI(1).Type == RightBracket && tokenizer.PeekI(2).Type == LeftParen {
			link := current_paragraph.AddHyperLink()
			current_hyperlink = &link
			name := tokenizer.Pop()
			run := link.AddRun()
			run.AddText(name.Value)
			run.Properties().SetUnderline(wml.ST_UnderlineSingle, color.Black)
			run.Properties().SetColor(color.Blue) // Blue
		} else if current.Type == LeftBracket {
			run := current_paragraph.AddRun()
			run.AddText(current.Value)
		} else if current.Type == LeftParen && tokenizer.Peek().Type == Text {
			if current_hyperlink == nil {
				current_paragraph.AddRun().AddText(tokenizer.Pop().Value)
				continue
			}
			target := tokenizer.Pop()
			current_hyperlink.SetTarget(target.Value)
			tokenizer.Pop()

			log.Println(target.String())
		} else if current.Type == Text {
			run := current_paragraph.AddRun()
			run.AddText(current.Value)
		} else if current.Type == Newline {
			current_paragraph = doc.AddParagraph()
		}
	}

	doc.AddParagraph().Properties().AddSection(wml.ST_SectionMarkNextPage)

	fmt.Println("File:", path)
}

type Tokenizer struct {
	tokens []Token
	index  int
}

// NewTokenizer creates a tokenizer instance with the given tokens
func NewTokenizer(tokens []Token) *Tokenizer {
	return &Tokenizer{tokens: tokens, index: 0}
}

// HasNext checks if there are more tokens to process
func (t *Tokenizer) HasNext() bool {
	return t.index < len(t.tokens)
}

func (t *Tokenizer) Println() {
	for _, token := range t.tokens {
		log.Println(token.String())
	}
}

func (token *Token) String() string {
	return fmt.Sprintf("[Type: %s, Value: %s]", token.Type, token.Value)
}

// Peek returns the next token without advancing the index
func (t *Tokenizer) Peek() Token {
	if t.HasNext() {
		return t.tokens[t.index]
	}
	return Token{ Type: "", Value: "" }
}

func (t *Tokenizer) PeekI(i int) Token {
	if len(t.tokens) > t.index+i {
		return t.tokens[t.index+i]
	}
	return Token{ Type: "", Value: "" }
}

// Pop returns the next token and advances the index
func (t *Tokenizer) Pop() *Token {
	if t.HasNext() {
		token := &t.tokens[t.index]
		t.index++
		return token
	}
	return nil
}

// Define the token types

const (
	Newline        = "newline"
	Bullet         = "bullet"
	Heading        = "heading"
	ImageLinkStart = "imagelinkstart"
	ImageLinkEnd   = "imagelinkend"
	LeftBracket    = "leftbracket"
	RightBracket   = "linknameend"
	LeftParen      = "linktargetstart"
	RightParen     = "linktargetend"

	Indent     = "indent"
	Unindent   = "unindent"
	LinkMiddle = "linkmiddle"

	Code = "code"
	Text = "text"
)

// Token structure
type Token struct {
	Type  string
	Value string
}

type Indentation struct {
	size    int
	current []rune
}

func (i *Indentation) Append(r rune) {
	i.current = append(i.current, r)
	i.size++
}

func (i *Indentation) Pop() rune {
	r := i.current[len(i.current)-1]
	i.current = i.current[:len(i.current)-1]
	i.size--
	return r
}

func (i *Indentation) IsEmpty() bool {
	return i.size == 0
}

func (i *Indentation) Peek() rune {
	if i.IsEmpty() {
		return 0
	}
	return i.current[len(i.current)-1]
}

func (i *Indentation) Peek2() rune {
	if len(i.current) < 2 || i.IsEmpty() {
		return 0
	}
	return i.current[len(i.current)-2]
}

type Tokens []Token

func (tokens *Tokens) Append(current string, t string, value string, styles Indentation) {
	text := current[:len(current)-len(value)]
	tokens.AppendText(text, styles)
	*tokens = append(*tokens, Token{Type: t, Value: value})
}

func (tokens *Tokens) AppendText(text string, styles Indentation) {
	if text != "" {
		if styles.Peek() == '`' {
			*tokens = append(*tokens, Token{Type: Code, Value: text})
			println(tokens.String())
		} else {
			*tokens = append(*tokens, Token{Type: Text, Value: text})
		}
	}
}

func (tokens *Tokens) String() string {
	var str string
	for _, token := range *tokens {
		str += token.String()
	}
	return str
}

// Tokenize function
func Tokenize(input string) []Token {
	var tokens Tokens
	input = strings.TrimRight(input, "\n")
	lines := strings.Split(input, "\n")

	styles := Indentation{size: 0, current: make([]rune, 0)}
	for _, line := range lines {
		current := ""
		indent := Indentation{size: 0, current: make([]rune, 0)}
		for _, character := range line {
			current += string(character)

			if indent.IsEmpty() && current == "-" {
				tokens.Append(current, Bullet, "-", styles)
				current = ""
			}

			if indent.IsEmpty() && current == "#" {
				tokens.Append(current, Heading, "#", styles)
				current = ""
			}

			if !strings.HasSuffix(current, "![") && !strings.HasSuffix(current, "![[") && strings.HasSuffix(current, "[") {
				tokens.Append(current, LeftBracket, "[", styles)
				indent.Append('[')
				current = ""
			}

			if indent.Peek() == '[' && strings.HasSuffix(current, "](") {
				text := current[:len(current)-2]
				tokens = append(tokens, Token{Type: Text, Value: text})
				tokens = append(tokens, Token{Type: RightBracket, Value: "]"})
				tokens = append(tokens, Token{Type: LeftParen, Value: "("})
				indent.Pop()
				indent.Append('(')
				current = ""
			}

			if indent.Peek() == '(' && strings.HasSuffix(current, ")") {
				tokens.Append(current, RightParen, ")", styles)
				indent.Pop()
				current = ""
			}

			if strings.HasSuffix(current, "`") {
				if styles.Peek() == '`' {
					text := current[:len(current)-1]
					tokens = append(tokens, Token{Type: Code, Value: text})
					styles.Pop()
					current = ""
				} else {
					text := current[:len(current)-1]
					tokens = append(tokens, Token{Type: Text, Value: text})
					styles.Append('`')
					current = ""
				}
			}

			if strings.HasSuffix(current, "![[") {
				tokens.Append(current, ImageLinkStart, "![[", styles)
				current = ""
			}

			if strings.HasSuffix(current, "]]") {
				tokens.Append(current, ImageLinkEnd, "]]", styles)
				current = ""
			}
		}
		tokens.AppendText(current, styles)
		tokens = append(tokens, Token{Type: Newline, Value: "newline"})
	}

	return tokens
}
