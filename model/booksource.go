package model

// BookSource represents a Legado book source JSON definition.
type BookSource struct {
	BookSourceName   string      `json:"bookSourceName"`
	BookSourceUrl    string      `json:"bookSourceUrl"`
	BookSourceGroup  string      `json:"bookSourceGroup"`
	BookSourceComment string     `json:"bookSourceComment"`
	BookSourceType   int         `json:"bookSourceType"`
	Enabled          bool        `json:"enabled"`
	EnabledExplore   bool        `json:"enabledExplore"`
	EnabledCookieJar bool        `json:"enabledCookieJar"`
	SearchUrl        string      `json:"searchUrl"`
	Header           string      `json:"header"`
	Weight           int         `json:"weight"`
	CustomOrder      int         `json:"customOrder"`
	LastUpdateTime   int64       `json:"lastUpdateTime"`
	RuleSearch       SearchRule  `json:"ruleSearch"`
	RuleBookInfo     BookInfoRule `json:"ruleBookInfo"`
	RuleToc          TocRule     `json:"ruleToc"`
	RuleContent      ContentRule `json:"ruleContent"`
	RuleExplore      ExploreRule `json:"ruleExplore"`
	ExploreUrl       interface{} `json:"exploreUrl"`
}

// SearchRule defines rules for parsing search results.
type SearchRule struct {
	BookList    string `json:"bookList"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	BookUrl     string `json:"bookUrl"`
	CoverUrl    string `json:"coverUrl"`
	Intro       string `json:"intro"`
	Kind        string `json:"kind"`
	LastChapter string `json:"lastChapter"`
	WordCount   string `json:"wordCount"`
}

// BookInfoRule defines rules for parsing book detail page.
type BookInfoRule struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	CoverUrl    string `json:"coverUrl"`
	Intro       string `json:"intro"`
	Kind        string `json:"kind"`
	LastChapter string `json:"lastChapter"`
	TocUrl      string `json:"tocUrl"`
	Init        string `json:"init"`
	WordCount   string `json:"wordCount"`
}

// TocRule defines rules for parsing table of contents.
type TocRule struct {
	ChapterList string `json:"chapterList"`
	ChapterName string `json:"chapterName"`
	ChapterUrl  string `json:"chapterUrl"`
	NextTocUrl  string `json:"nextTocUrl"`
}

// ContentRule defines rules for parsing chapter content.
type ContentRule struct {
	Content        string `json:"content"`
	NextContentUrl string `json:"nextContentUrl"`
	Title          string `json:"title"`
	ReplaceRegex   string `json:"replaceRegex"`
	ImageStyle     string `json:"imageStyle"`
}

// ExploreRule defines rules for explore/discover page.
type ExploreRule struct {
	BookList    string `json:"bookList"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	BookUrl     string `json:"bookUrl"`
	CoverUrl    string `json:"coverUrl"`
	Intro       string `json:"intro"`
	Kind        string `json:"kind"`
	LastChapter string `json:"lastChapter"`
	WordCount   string `json:"wordCount"`
}
