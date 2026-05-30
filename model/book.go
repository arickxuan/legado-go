package model

// SearchBook represents a book found during search.
type SearchBook struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	CoverUrl    string `json:"coverUrl"`
	Intro       string `json:"intro"`
	Kind        string `json:"kind"`
	LastChapter string `json:"lastChapter"`
	BookUrl     string `json:"bookUrl"`
	WordCount   string `json:"wordCount"`
	Origin      string `json:"origin"`      // bookSourceUrl
	OriginName  string `json:"originName"`  // bookSourceName
}

// Book represents a book with full info.
type Book struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	CoverUrl    string `json:"coverUrl"`
	Intro       string `json:"intro"`
	Kind        string `json:"kind"`
	LastChapter string `json:"lastChapter"`
	WordCount   string `json:"wordCount"`
	TocUrl      string `json:"tocUrl"`
	BookUrl     string `json:"bookUrl"`
	Origin      string `json:"origin"`
	OriginName  string `json:"originName"`
}

// ToBook converts a SearchBook to a Book.
func (sb *SearchBook) ToBook() *Book {
	return &Book{
		Name:        sb.Name,
		Author:      sb.Author,
		CoverUrl:    sb.CoverUrl,
		Intro:       sb.Intro,
		Kind:        sb.Kind,
		LastChapter: sb.LastChapter,
		WordCount:   sb.WordCount,
		BookUrl:     sb.BookUrl,
		Origin:      sb.Origin,
		OriginName:  sb.OriginName,
	}
}
