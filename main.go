package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AlexGames73/unioffice-free/common"
	"github.com/AlexGames73/unioffice-free/document"
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
		log.Println(current.String())
		if current.Type == Text {
			run := current_paragraph.AddRun()
			run.AddText(current.Value)
		} else if current.Type == Newline {
			current_paragraph = doc.AddParagraph()
		}

		if !tokenizer.HasNext() {
			continue;
		}

		if current.Type == Newline && tokenizer.Peek().Type == Text {
			current_paragraph = doc.AddParagraph()
		} else if current.Type == Bullet && tokenizer.Peek().Type == Text {
			run := current_paragraph.AddRun()
			run.AddText("- " + tokenizer.Pop().Value)
		} else if current.Type == ImageLinkStart && tokenizer.Peek().Type == Text {
			file_path := assets_path + "/" + tokenizer.Pop().Value
			tokenizer.Pop()
			img, err := common.ImageFromFile(file_path)
			if err != nil {
				log.Fatal(err)
			}
			// Load image from file
			_, err = doc.AddImage(img)
			if err != nil {
				log.Fatal(err)
			}
			tokenizer.Pop()
		} else if current.Type == LinkNameStart && tokenizer.Peek().Type == Text {
			link := current_paragraph.AddHyperLink()
			current_hyperlink = &link
			link.AddRun().AddText(tokenizer.Pop().Value)
			tokenizer.Pop()
		} else if current.Type == LinkTargetStart && tokenizer.Peek().Type == Text {
			if current_hyperlink == nil {
				current_paragraph.AddRun().AddText(tokenizer.Pop().Value)
				continue;
			}
			current_hyperlink.SetTarget(tokenizer.Pop().Value)
			tokenizer.Pop()
		} else if current.Type == Code {
			run := current_paragraph.AddRun()
			run.AddText("`" + tokenizer.Pop().Value + "`")
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
func (t *Tokenizer) Peek() *Token {
	if t.HasNext() {
		return &t.tokens[t.index]
	}
	return nil
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
	Newline         = "newline"
	Bullet          = "bullet"
	ImageLinkStart  = "imagelinkstart"
	ImageLinkEnd    = "imagelinkend"
	LinkNameStart   = "linknamestart"
	LinkNameEnd     = "linknameend"
	LinkTargetStart = "linktargetstart"
	LinkTargetEnd   = "linktargetend"

	Indent          = "indent"
	Unindent        = "unindent"
	LinkMiddle      = "linkmiddle"

	Code            = "code"
	Text            = "text"
)

// Token structure
type Token struct {
	Type  string
	Value string
}

// Tokenize function
func Tokenize(input string) []Token {
	var tokens []Token
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		current := ""
		indent := ""
		for _, character := range line {
			current += string(character)

			if current == "[" {
				tokens = append(tokens, Token{Type: LinkNameStart, Value: current})
			}

			if current == "![[" {
				tokens = append(tokens, Token{Type: ImageLinkStart, Value: current})
			}

			if current == "]]" {
				tokens = append(tokens, Token{Type: ImageLinkEnd, Value: current})
			}
		}
		tokens = append(tokens, Token{Type: Newline, Value: current})
	}

	// Define regex patterns for different token types
	reBullet := regexp.MustCompile(`(?m)^- `)        // Match bullet points
	reCode := regexp.MustCompile("`[^`]+`")          // Match code blocks

	// Match link names [linkname](
	reLinkName := regexp.MustCompile(`\[[^\]]+\]\(`)
	// Match link targets ](linktarget)
	reLinkTarget := regexp.MustCompile(`\]\(.*\)`)
	// Match image links [[imagelink!]]
	reImageLink := regexp.MustCompile(`\[\[.*!\]\]`)

	// Split the input by newlines
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		// Check for bullet points
		if reBullet.MatchString(line) {
			tokens = append(tokens, Token{Type: Bullet, Value: "-"})
			line = reBullet.ReplaceAllString(line, "")
		}

		// Find code blocks
		codeMatches := reCode.FindAllString(line, -1)
		for _, code := range codeMatches {
			tokens = append(tokens, Token{Type: Code, Value: code})
			line = strings.Replace(line, code, "", 1) // Remove the code block from line
		}

		// Find image links
		imageLinkMatches := reImageLink.FindAllString(line, -1)
		for _, imageLink := range imageLinkMatches {
			tokens = append(tokens, Token{Type: ImageLinkStart, Value: "!"})
			tokens = append(tokens, Token{Type: Text, Value: imageLink[1:len(imageLink)-1]})
			tokens = append(tokens, Token{Type: ImageLinkEnd, Value: "!"})
			line = strings.Replace(line, imageLink, "", 1) // Remove the image link from line
		}

		// Find link names
		linkNameMatches := reLinkName.FindAllString(line, -1)
		for _, linkName := range linkNameMatches {
			tokens = append(tokens, Token{Type: LinkNameStart, Value: "["})
			tokens = append(tokens, Token{Type: Text, Value: linkName[1:len(linkName)-1]})
			tokens = append(tokens, Token{Type: LinkNameEnd, Value: "]"})
			line = strings.Replace(line, linkName, "", 1) // Remove the link name from line
		}


		// Find link targets
		linkTargetMatches := reLinkTarget.FindAllString(line, -1)
		for _, linkTarget := range linkTargetMatches {
			tokens = append(tokens, Token{Type: LinkTargetStart, Value: "("})
			tokens = append(tokens, Token{Type: Text, Value: linkTarget[1:len(linkTarget)-1]})

			tokens = append(tokens, Token{Type: LinkTargetEnd, Value: ")"})
			line = strings.Replace(line, linkTarget, "", 1) // Remove the link target from line
		}

		// Remaining text as a single token (if any)

		if strings.TrimSpace(line) != "" {

			tokens = append(tokens, Token{Type: Text, Value: line})

		}

		// Add newline token
		tokens = append(tokens, Token{Type: Newline, Value: "\n"})
	}

	return tokens
}
