package model

// BookChapter represents a chapter in the table of contents.
type BookChapter struct {
	Title   string `json:"title"`
	Url     string `json:"url"`
	BookUrl string `json:"bookUrl"`
	Index   int    `json:"index"`
}
