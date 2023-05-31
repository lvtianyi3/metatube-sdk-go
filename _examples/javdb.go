package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/proxy"
	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"log"
	"strings"
)

const (
	baseURL = "https://javdb.com/"
	//movieURL            = "https://www.javbus.com/ja/%s"
	searchURL = "https://javdb.com/search?q=%s&f=all"
	//searchUncensoredURL = "https://www.javbus.com/ja/uncensored/search/%s"
)

func main() {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
	)

	rp, err := proxy.RoundRobinProxySwitcher("http://127.0.0.1:7890", "https://127.0.0.1:7890")

	if err != nil {
		log.Fatal(err)
	}

	c.SetProxyFunc(rp)

	c.OnXML(`//div[@class="movie-list h cols-4 vcols-8"]/div[@class="item"][1]/a`, func(e *colly.XMLElement) {

		var cover string
		cover = e.ChildAttr(`.//div[@class="cover"]/img`, "src")
		fmt.Printf("cover: %s\n", cover)

		homepage := e.Request.AbsoluteURL(e.Attr("href"))

		fmt.Printf("homepage : %s\n", homepage)

		//var thumb, cover string
		//cover = e.Request.AbsoluteURL(e.ChildAttr(`.//a/div[@class="cover"]/img`, "src"))
		////c.Visit(e.Request.AbsoluteURL(link))
		//thumb = cover
		//fmt.Printf("Link found: %q -> %s\n", thumb, cover)
	})

	c.OnXML(`//a[@class="movie-listx"]`, func(e *colly.XMLElement) {

		//var thumb, cover string
		//thumb = e.Request.AbsoluteURL(e.ChildAttr(`.//div[1]/img`, "src"))
		//if re := regexp.MustCompile(`(?i)/thumbs?/([a-z\d]+)(?:_b)?\.(jpg|png)`); re.MatchString(thumb) {
		//	cover = re.ReplaceAllString(thumb, "/cover/${1}_b.${2}") // guess
		//}

		//homepage := e.Request.AbsoluteURL(e.Attr("href"))
		//id, _ := db.ParseIDFromURL(homepage)
		//results = append(results, &model.MovieSearchResult{
		//	ID:          id,
		//	Number:      e.ChildText(`.//div[2]/span/date[1]`),
		//	Title:       strings.SplitN(e.ChildText(`.//div[2]/span`), "\n", 2)[0],
		//	Provider:    db.Name(),
		//	Homepage:    homepage,
		//	ThumbURL:    thumb,
		//	CoverURL:    cover,
		//	ReleaseDate: parser.ParseDate(e.ChildText(`.//div[2]/span/date[2]`)),
		//})
	})

	c.OnXML(`//strong[@class="current-title"]`, func(e *colly.XMLElement) {
		fmt.Printf("标题: %s\n", e.Text)
	})

	c.OnXML(`//nav[@class="panel movie-panel-info"]/div`, func(e *colly.XMLElement) {
		switch e.ChildText(`.//strong`) {
		case "番號:":
			//info.Number = e.ChildText(`.//span[2]`)
			fmt.Printf("番號: %s\n", e.ChildText(`.//span[1]`))
		case "日期:":
			fields := strings.Fields(e.Text)
			fmt.Printf("日期: %s\n", fields[len(fields)-1])
			releaseDate := parser.ParseDate(fields[len(fields)-1])
			fmt.Printf("日期: %s\n", releaseDate)

		case "片商:":
			fmt.Printf("片商-Mark: %s\n", e.ChildText(`.//span[1]/a`))
		case "系列:":
			fmt.Printf("系列:Series: %s\n", e.ChildText(`.//span[1]/a`))
			//case "評分:":
			//	fmt.Printf("評分:Score: %s\n", e.ChildText(`.//span[1]/a`))

		case "類別:":
			var genres = e.ChildTexts(`.//span[@class="value"]/a`)
			fmt.Printf("Genres: %s\n", genres)
		case "演員:":
			var actors = e.ChildTexts(`.//span[@class="value"]/a`)
			fmt.Printf("演員: %s\n", actors)
		}

	})

	// Image
	c.OnXML(`//div[@class="column column-video-cover"]/a/img`, func(e *colly.XMLElement) {

		//info.CoverURL = e.Request.AbsoluteURL(e.Attr("src"))
		fmt.Printf("CoverURL: %s\n", e.Request.AbsoluteURL(e.Attr("src")))
	})

	// Previews
	c.OnXML(`//div[@class="tile-images preview-images"]/a`, func(e *colly.XMLElement) {
		//info.PreviewImages = append(info.PreviewImages, e.Request.AbsoluteURL(e.Attr("href")))
		fmt.Printf("PreviewImages: %s\n", e.Request.AbsoluteURL(e.Attr("href")))
	})

	//c.Visit(fmt.Sprintf(searchURL, "BabyGotBoobs.18.12.27"))

	c.Visit("https://javdb.com/v/zMd7z")
}
