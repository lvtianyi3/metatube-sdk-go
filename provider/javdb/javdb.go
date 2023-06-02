package javdb

import (
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/gocolly/colly/v2"
	"strings"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.MovieProvider = (*JavDB)(nil)
	_ provider.MovieSearcher = (*JavDB)(nil)
)

const (
	Name     = "JavDB"
	Priority = 1000 - 5
)

const (
	baseURL   = "https://javdb.com/"
	movieURL  = "https://www.javdb.com/ja/%s"
	searchURL = "https://javdb.com/search?q=%s&f=all"
)

type JavDB struct {
	*scraper.Scraper
}

func New() *JavDB {
	return &JavDB{scraper.NewDefaultScraper(Name, baseURL, Priority)}
}

func (db *JavDB) NormalizeID(id string) string {
	return id
}

func (db *JavDB) GetURLByID(id string) (string, error) {
	c := db.ClonedCollector()

	var homepage string
	var err error = nil

	c.OnXML(`//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"][1]/a`, func(e *colly.XMLElement) {
		homepage = e.Request.AbsoluteURL(e.Attr("href"))
	})

	keyword := regexp.MustCompile(`\.\d{2}\.\d{2}\.\d{2}`).ReplaceAllString(id, "")
	c.Visit(fmt.Sprintf(searchURL, keyword))
	c.Wait()

	if homepage == "" {
		err = errors.New("string is empty")
	}

	return homepage, err

}
func (db *JavDB) GetMovieInfoByID(id string) (info *model.MovieInfo, err error) {
	url, err := db.GetURLByID(id)
	if err != nil {
		return
	}
	return db.GetMovieInfoByURL(url)
}

func (db *JavDB) ParseIDFromURL(rawURL string) (string, error) {

	c := db.ClonedCollector()
	var number, title string
	var err error = nil

	c.OnXML(`//nav[@class="panel movie-panel-info"]/div[@class="panel-block first-block"]/span`, func(e *colly.XMLElement) {
		number = e.Text
	})
	c.OnXML(`//strong[@class="current-title"]`, func(e *colly.XMLElement) {
		title = e.Text
	})

	c.Visit(rawURL)
	c.Wait()

	if number == "" {
		err = errors.New("string is empty")
	}

	if title == "" {
		err = errors.New("string is empty")
	}

	if err != nil {
		return "", err
	}

	id := strings.TrimSpace(number + " " + title)
	return id, err
}

func (db *JavDB) GetMovieInfoByURL(rawURL string) (info *model.MovieInfo, err error) {
	id, err := db.ParseIDFromURL(rawURL)
	if err != nil {
		return
	}

	info = &model.MovieInfo{
		ID:            id,
		Provider:      db.Name(),
		Homepage:      rawURL,
		Actors:        []string{},
		PreviewImages: []string{},
		Genres:        []string{},
	}

	c := db.ClonedCollector()
	// Title
	c.OnXML(`//strong[@class="current-title"]`, func(e *colly.XMLElement) {
		info.Title = e.Text
	})

	// Image
	c.OnXML(`//div[@class="column column-video-cover"]/a/img`, func(e *colly.XMLElement) {
		info.CoverURL = e.Request.AbsoluteURL(e.Attr("src"))
		info.ThumbURL = info.CoverURL
	})

	// Fields
	c.OnXML(`//nav[@class="panel movie-panel-info"]/div`, func(e *colly.XMLElement) {
		switch e.ChildText(`.//strong`) {
		case "番號:":
			info.Number = e.ChildText(`.//span[1]`)
			//fmt.Printf("番號: %s\n", e.ChildText(`.//span[1]`))
		case "日期:":
			fields := strings.Fields(e.ChildText(`.//span[1]`))
			info.ReleaseDate = parser.ParseDate(fields[len(fields)-1])
			//fmt.Printf("日期: %s\n", e.ChildText(`.//span[1]`))
		case "片商:":
			info.Maker = e.ChildText(`.//span[1]/a`)
			//fmt.Printf("片商-Mark: %s\n", e.ChildText(`.//span[1]/a`))
		case "系列:":
			//fmt.Printf("系列:Series: %s\n", e.ChildText(`.//span[1]/a`))
			info.Series = e.ChildText(`.//span[1]/a`)
		// Genres
		case "類別:":
			var genres = e.ChildTexts(`.//span[@class="value"]/a`)
			info.Genres = append(info.Genres, genres...)
			//fmt.Printf("Genres: %s\n", genres)
		// Actors
		case "演員:":
			var actors = e.ChildTexts(`.//span[@class="value"]/a`)
			info.Actors = append(info.Actors, actors...)
			//fmt.Printf("演員: %s\n", actors)
		}
	})

	// Previews
	c.OnXML(`//div[@class="tile-images preview-images"]/a`, func(e *colly.XMLElement) {
		info.PreviewImages = append(info.PreviewImages, e.Request.AbsoluteURL(e.Attr("href")))
		//fmt.Printf("PreviewImages: %s\n", e.Request.AbsoluteURL(e.Attr("href")))
	})

	err = c.Visit(info.Homepage)
	return
}

func (db *JavDB) NormalizeKeyword(keyword string) string {
	return keyword
}

func (db *JavDB) SearchMovie(keyword string) (results []*model.MovieSearchResult, err error) {
	c := db.ClonedCollector()
	c.Async = true /* ASYNC */

	var mu sync.Mutex
	c.OnXML(`//div[@class="movie-list h cols-4 vcols-8"]/div[position() < 4][@class="item"]/a`, func(e *colly.XMLElement) {
		mu.Lock()
		defer mu.Unlock()
		homepage := e.Request.AbsoluteURL(e.Attr("href"))

		//var id string
		//if id, err = db.ParseIDFromURL(homepage); err != nil {
		//	return
		//}
		//fmt.Printf("homepage : %s\n", homepage)
		var info *model.MovieInfo
		if info, err = db.GetMovieInfoByURL(homepage); err != nil {
			return
		}
		results = append(results, info.ToSearchResult())
	})

	keyword = regexp.MustCompile(`\.\d{2}\.\d{2}\.\d{2}`).ReplaceAllString(keyword, "")

	for _, u := range []string{
		fmt.Sprintf(searchURL, keyword),
	} {
		if err = c.Visit(u); err != nil {
			return nil, err
		}
	}
	c.Wait()
	return
}

func init() {
	provider.RegisterMovieFactory(Name, New)
}
