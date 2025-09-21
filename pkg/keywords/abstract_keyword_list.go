package keywords

import (
	"strings"
)

// KeywordList - Abstract interface for a SQL reserved keyword dictionary.
type KeywordList struct {
	keywords map[string]int
	child    KeywordListInterface
}

func NewKeywordList(child KeywordListInterface) *KeywordList {
	return &KeywordList{child: child}
}

func (k *KeywordList) InitializeKeywords() {
	keywords := make(map[string]int)

	for index, keyword := range k.child.GetKeywords() {
		keywords[strings.ToUpper(keyword)] = index
	}

	k.keywords = keywords
}

// IsKeyword - Checks if the given word is a keyword of this dialect/vendor platform.
func (k *KeywordList) IsKeyword(word string) bool {
	if k.keywords == nil {
		k.InitializeKeywords()
	}

	_, found := k.keywords[strings.ToUpper(word)]

	return found
}
